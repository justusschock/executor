package executor

import (
	"errors"
	"os"
	"os/exec"
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

		assertPrivateDependencyVersions(t)
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
			"go mod tidy",
			"git diff --exit-code -- go.mod go.sum",
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

func assertPrivateDependencyVersions(t *testing.T) {
	t.Helper()
	command := exec.Command("go", "list", "-m", "all")
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list module graph: %v", err)
	}

	privateDependencies := 0
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "github.com/justusschock/go-") {
			continue
		}
		privateDependencies++
		if fields[1] != "v0.2.0" {
			t.Errorf("%s resolved version = %s, want v0.2.0", fields[0], fields[1])
		}
	}
	if privateDependencies == 0 {
		t.Fatal("module graph must contain private github.com/justusschock/go-* dependencies")
	}
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
