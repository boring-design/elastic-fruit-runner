package probe

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/backend"
	"github.com/boring-design/elastic-fruit-runner/internal/binpath"
)

// BackendResult is the outcome of checking one runner backend CLI.
type BackendResult struct {
	Backend   string
	Available bool
	Version   string
	HostOS    string
	HostArch  string
	Error     string
	// Runtimes lists the container runtimes Docker knows, such as runc and runsc. Docker only.
	Runtimes []string
}

const backendTimeout = 10 * time.Second

// CheckBackend runs the backend CLI once to prove it is installed and working.
func CheckBackend(ctx context.Context, name string) BackendResult {
	result := BackendResult{
		Backend:  name,
		HostOS:   runtime.GOOS,
		HostArch: runtime.GOARCH,
	}
	switch name {
	case "docker":
		result.Version, result.Error = runVersionCommand(ctx, "docker", "version", "--format", "{{.Server.Version}}")
		if result.Error == "" {
			result.Runtimes, result.Error = dockerRuntimes(ctx)
		}
	case "tart":
		if runtime.GOOS != "darwin" {
			result.Error = "tart runs only on macOS"
			return result
		}
		result.Version, result.Error = runVersionCommand(ctx, "tart", "--version")
	default:
		result.Error = "unknown backend " + name + ", expected docker or tart"
		return result
	}
	result.Available = result.Error == ""
	return result
}

func dockerRuntimes(ctx context.Context) (runtimes []string, errorMessage string) {
	ctx, cancel := context.WithTimeout(ctx, backendTimeout)
	defer cancel()
	runtimes, err := backend.DockerRuntimes(ctx)
	if err != nil {
		return nil, err.Error()
	}
	return runtimes, ""
}

func runVersionCommand(ctx context.Context, name string, args ...string) (version, errorMessage string) {
	path := binpath.Lookup(name)
	if !filepath.IsAbs(path) {
		return "", name + " CLI not found in PATH or /opt/homebrew/bin, /usr/local/bin"
	}
	ctx, cancel := context.WithTimeout(ctx, backendTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			text = name + " failed: " + err.Error()
		}
		return "", text
	}
	return text, ""
}
