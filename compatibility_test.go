package executor

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestRepositoryCompatibilityPolicy(t *testing.T) {
	t.Run("module", func(t *testing.T) {
		module := readRepositoryFile(t, "go.mod")
		if !regexp.MustCompile(`(?m)^go 1\.22$`).MatchString(module) {
			t.Error("go.mod must declare Go 1.22")
		}

		privateDependency := regexp.MustCompile(`(?m)^\s*(?:require\s+)?(github\.com/justusschock/go-[^\s]+)\s+(v[^\s]+)\s*$`)
		matches := privateDependency.FindAllStringSubmatch(module, -1)
		if len(matches) == 0 {
			t.Fatal("go.mod must contain private github.com/justusschock/go-* dependencies")
		}
		for _, match := range matches {
			if match[2] != "v0.2.0" {
				t.Errorf("%s version = %s, want v0.2.0", match[1], match[2])
			}
		}
		if regexp.MustCompile(`(?m)^replace\b`).MatchString(module) {
			t.Error("go.mod must not contain replace directives")
		}
		if _, err := os.Stat("go.work"); err == nil {
			t.Error("repository must not contain go.work")
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat go.work: %v", err)
		}
	})

	t.Run("continuous integration", func(t *testing.T) {
		workflow := readRepositoryFile(t, ".github/workflows/ci.yml")
		for _, required := range []string{
			"'1.22.x'",
			"'1.23.x'",
			"'1.24.x'",
			"'1.25.x'",
			"'1.26.x'",
			"actions/checkout@v6",
			"actions/setup-go@v6",
			"secrets.GO_MODULES_TOKEN",
			"go test ./...",
			"go test -race ./...",
			"go vet ./...",
			"go mod verify",
			"go mod tidy -diff",
		} {
			if !strings.Contains(workflow, required) {
				t.Errorf("CI workflow must contain %q", required)
			}
		}
	})

	t.Run("readme", func(t *testing.T) {
		readme := readRepositoryFile(t, "README.md")
		if !strings.Contains(readme, "Go 1.22 or newer is required") {
			t.Error("README must document Go 1.22 or newer")
		}
	})
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
