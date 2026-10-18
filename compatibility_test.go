package executor

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

// NFR-EXEC-04: the module declares the Go 1.25 floor (testing/synctest),
// builds without replace directives or a committed workspace, and its README
// states the requirement.
func TestRepositoryCompatibilityPolicy(t *testing.T) {
	t.Run("module", func(t *testing.T) {
		module := readRepositoryFile(t, "go.mod")
		if !regexp.MustCompile(`(?m)^go 1\.25$`).MatchString(module) {
			t.Error("go.mod must declare Go 1.25")
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
			"'1.25.x'",
			"go mod verify",
			"go vet ./...",
			"go test -race -count=1 ./...",
		} {
			if !strings.Contains(workflow, required) {
				t.Errorf("CI workflow must contain %q", required)
			}
		}
		for _, forbidden := range []string{"pull_request_target", "secrets.", "GOPRIVATE", "GONOSUMDB"} {
			if strings.Contains(workflow, forbidden) {
				t.Errorf("CI workflow must not contain %q", forbidden)
			}
		}
	})

	t.Run("readme", func(t *testing.T) {
		readme := readRepositoryFile(t, "README.md")
		if !strings.Contains(readme, "Go 1.25 or newer") {
			t.Error("README must document Go 1.25 or newer")
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
