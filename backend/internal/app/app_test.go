package app_test

import (
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// Kontrak mengikat frontend, jadi tiap operasi harus punya rute dengan nama parameter yang sama.
func TestEveryContractOperationHasARoute(t *testing.T) {
	a := testkit.NewApp(t)
	ops := a.Contract.Operations()
	if len(ops) == 0 {
		t.Fatal("kontrak tidak memuat operasi")
	}
	for _, op := range ops {
		path := pathParam.ReplaceAllString(op.Path, "019a0000-0000-7000-8000-000000000000")
		_, pattern := a.API.Mux.Handler(httptest.NewRequest(op.Method, path, nil))
		if want := op.Method + " " + op.Path; pattern != want {
			t.Errorf("%s: rute %q, seharusnya %q", op.ID, pattern, want)
		}
	}
}
