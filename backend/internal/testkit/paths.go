package testkit

import (
	"path/filepath"
	"runtime"
)

// ContractPath mengembalikan lokasi api/openapi.yaml.
func ContractPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "api", "openapi.yaml")
}
