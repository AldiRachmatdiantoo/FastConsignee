package testkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const specURL = "mem://openapi.json"

// Contract memvalidasi jawaban API terhadap api/openapi.yaml. Skema OpenAPI 3.1 sama dengan
// JSON Schema 2020-12, jadi validasinya memakai validator JSON Schema penuh.
type Contract struct {
	doc      map[string]any
	compiler *jsonschema.Compiler
	routes   []route

	mu      sync.Mutex
	schemas map[string]*jsonschema.Schema
}

type route struct {
	template string
	pattern  *regexp.Regexp
	item     map[string]any
}

// Operation adalah satu operasi di kontrak.
type Operation struct {
	ID     string
	Method string
	Path   string
}

// LoadContract membaca kontrak dari file YAML.
func LoadContract(path string) (*Contract, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	asJSON, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(asJSON))
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(asJSON, &doc); err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	if err := compiler.AddResource(specURL, resource); err != nil {
		return nil, err
	}

	c := &Contract{doc: doc, compiler: compiler, schemas: map[string]*jsonschema.Schema{}}
	paths, _ := doc["paths"].(map[string]any)
	param := regexp.MustCompile(`\\\{[^/]+\\\}`)
	for template, item := range paths {
		quoted := regexp.QuoteMeta(template)
		c.routes = append(c.routes, route{
			template: template,
			pattern:  regexp.MustCompile("^" + param.ReplaceAllString(quoted, "[^/]+") + "$"),
			item:     item.(map[string]any),
		})
	}
	return c, nil
}

// Operations mengembalikan semua operasi.
func (c *Contract) Operations() []Operation {
	var out []Operation
	for _, r := range c.routes {
		for method, v := range r.item {
			op, ok := v.(map[string]any)
			if !ok || method == "parameters" {
				continue
			}
			id, _ := op["operationId"].(string)
			out = append(out, Operation{ID: id, Method: strings.ToUpper(method), Path: r.template})
		}
	}
	return out
}

// ErrorCodes mengembalikan isi enum ErrorCode.
func (c *Contract) ErrorCodes() []string {
	schema := c.resolve("#/components/schemas/ErrorCode")
	var out []string
	for _, v := range schema["enum"].([]any) {
		out = append(out, v.(string))
	}
	return out
}

func (c *Contract) find(path string) (route, bool) {
	for _, r := range c.routes {
		if r.template == path {
			return r, true
		}
	}
	for _, r := range c.routes {
		if r.pattern.MatchString(path) && !c.hasExact(path) {
			return r, true
		}
	}
	return route{}, false
}

func (c *Contract) hasExact(path string) bool {
	for _, r := range c.routes {
		if r.template == path {
			return true
		}
	}
	return false
}

func (c *Contract) resolve(ref string) map[string]any {
	var node any = c.doc
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		part = strings.NewReplacer("~1", "/", "~0", "~").Replace(part)
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[part]
	}
	out, _ := node.(map[string]any)
	return out
}

func (c *Contract) schema(ref string) (*jsonschema.Schema, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.schemas[ref]; ok {
		return s, nil
	}
	s, err := c.compiler.Compile(specURL + ref)
	if err != nil {
		return nil, err
	}
	c.schemas[ref] = s
	return s, nil
}

// Validate memeriksa status, header, media type, dan isi jawaban.
func (c *Contract) Validate(method, path string, status int, header http.Header, body []byte) error {
	r, ok := c.find(path)
	if !ok {
		return fmt.Errorf("path %s tidak ada di kontrak", path)
	}
	op, ok := r.item[strings.ToLower(method)].(map[string]any)
	if !ok {
		return fmt.Errorf("%s %s tidak ada di kontrak", method, r.template)
	}
	responses, _ := op["responses"].(map[string]any)
	resp, ok := responses[strconv.Itoa(status)].(map[string]any)
	if !ok {
		return fmt.Errorf("%s %s: status %d tidak ada di kontrak. Isi: %s", method, r.template, status, body)
	}
	if ref, ok := resp["$ref"].(string); ok {
		resp = c.resolve(ref)
	}
	if header.Get("X-Request-Id") == "" {
		return errors.New("header X-Request-Id tidak ada")
	}
	if err := c.checkHeaders(r.template, status, resp, header); err != nil {
		return fmt.Errorf("%s %s %d: %w", method, r.template, status, err)
	}

	content, _ := resp["content"].(map[string]any)
	media := strings.SplitN(header.Get("Content-Type"), ";", 2)[0]
	spec, ok := content[media].(map[string]any)
	if !ok {
		return fmt.Errorf("%s %s %d: media type %q tidak ada di kontrak", method, r.template, status, media)
	}
	schemaNode, _ := spec["schema"].(map[string]any)
	ref, _ := schemaNode["$ref"].(string)
	if ref == "" {
		return fmt.Errorf("%s %s %d: skema harus memakai $ref", method, r.template, status)
	}
	schema, err := c.schema(ref)
	if err != nil {
		return fmt.Errorf("compile %s: %w", ref, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s %s %d: isi bukan JSON: %w", method, r.template, status, err)
	}
	if err := schema.Validate(inst); err != nil {
		return fmt.Errorf("%s %s %d melanggar kontrak: %w\nIsi: %s", method, r.template, status, err, body)
	}
	if media == "application/problem+json" {
		return checkProblem(status, header, body)
	}
	return nil
}

func (c *Contract) checkHeaders(template string, status int, resp map[string]any, header http.Header) error {
	declared, _ := resp["headers"].(map[string]any)
	for name := range declared {
		if name == "Idempotency-Replayed" {
			continue
		}
		if header.Get(name) == "" {
			return fmt.Errorf("header %s tidak ada", name)
		}
	}
	if template != "/healthz" && status < 500 && header.Get("RateLimit-Limit") == "" {
		return errors.New("header RateLimit-Limit tidak ada")
	}
	return nil
}

func checkProblem(status int, header http.Header, body []byte) error {
	var p struct {
		Status    int    `json:"status"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return err
	}
	if p.Status != status {
		return fmt.Errorf("status problem %d berbeda dengan HTTP %d", p.Status, status)
	}
	if p.RequestID != header.Get("X-Request-Id") {
		return errors.New("request_id berbeda dengan header X-Request-Id")
	}
	return nil
}
