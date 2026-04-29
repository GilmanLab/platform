package composition

import appversion "github.com/gilmanlab/platform/tools/labctl/internal/app/version"

// Input describes process-level values used to build the dependency graph.
type Input struct {
	// Version is the release version embedded into the labctl binary.
	Version string
}

// Dependencies groups app services for CLI command construction.
type Dependencies struct {
	// Version provides labctl build-version information.
	Version appversion.Service
}

// New wires app services to their concrete adapters.
func New(input Input) Dependencies {
	return Dependencies{
		Version: appversion.NewService(input.Version),
	}
}
