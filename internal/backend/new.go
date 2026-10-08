package backend

import "fmt"

// Spec describes which backend to build and how it should run runners.
// Both standalone runner sets and cloud start commands are turned into a Spec.
type Spec struct {
	// Backend is the backend name, docker or tart.
	Backend string
	// Image is the runner image. Empty means the backend default.
	Image string
	// Platform is the docker platform such as linux/arm64. Docker only.
	Platform string
	// Runtime is the container runtime passed to docker run, such as runsc. Docker only.
	Runtime string
}

// New builds the backend that runs a runner set from its spec.
func New(spec Spec) (Backend, error) {
	switch spec.Backend {
	case "tart":
		if spec.Runtime != "" {
			return nil, fmt.Errorf("runtime %q is not supported by the tart backend", spec.Runtime)
		}
		return NewTartBackend(spec.Image), nil
	case "docker":
		return NewDockerBackend(spec.Image, spec.Platform, spec.Runtime), nil
	default:
		return nil, fmt.Errorf("unknown backend %q, expected docker or tart", spec.Backend)
	}
}
