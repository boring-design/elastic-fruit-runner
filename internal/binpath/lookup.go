package binpath

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Well-known directories where tart, docker, and other tools may be installed.
var wellKnownDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
	"/bin",
	"/usr/sbin",
	"/sbin",
}

var (
	cache   = make(map[string]string)
	cacheMu sync.RWMutex
)

// Lookup resolves a binary name to its absolute path.
// It first tries exec.LookPath (respects $PATH), then falls back to
// probing well-known directories. The result is cached for the process lifetime.
func Lookup(name string) string {
	cacheMu.RLock()
	if p, ok := cache[name]; ok {
		cacheMu.RUnlock()
		return p
	}
	cacheMu.RUnlock()

	resolved := resolve(name)

	cacheMu.Lock()
	cache[name] = resolved
	cacheMu.Unlock()

	return resolved
}

func resolve(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	for _, dir := range wellKnownDirs {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}

	// Fallback: return the bare name and let exec fail with a clear error.
	return name
}
