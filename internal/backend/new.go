package backend

import "fmt"

// New builds the backend that runs a runner set from its name, image, and platform.
// Both standalone runner sets and cloud start commands go through this one constructor.
func New(name, image, platform string) (Backend, error) {
	switch name {
	case "tart":
		return NewTartBackend(image), nil
	case "docker":
		return NewDockerBackend(image, platform), nil
	default:
		return nil, fmt.Errorf("unknown backend %q, expected docker or tart", name)
	}
}
