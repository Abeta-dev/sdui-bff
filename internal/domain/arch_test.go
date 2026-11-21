package domain_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDomainCleanArchitectureIsolation verifies that internal/domain stays strictly free
// of HTTP, Gin, handler, usecase, vertical, and infrastructure dependencies.
func TestDomainCleanArchitectureIsolation(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"github.com/gin-gonic/gin",
		"net/http",
		"github.com/abeta-dev/sdui-bff/internal/handler",
		"github.com/abeta-dev/sdui-bff/internal/infra",
		"github.com/abeta-dev/sdui-bff/internal/usecase",
		"github.com/abeta-dev/sdui-bff/internal/verticals",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read domain dir: %v", err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		fileAST, parseErr := parser.ParseFile(fset, filepath.Join(".", entry.Name()), nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, imp := range fileAST.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") {
					t.Errorf("Clean Architecture violation: %s imports forbidden package %q", entry.Name(), path)
				}
			}
		}
	}
}
