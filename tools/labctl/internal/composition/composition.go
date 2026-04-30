package composition

import (
	"net/http"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubauth"
	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubbroker"
	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubcontents"
	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/secretslocal"
	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/sopsdecrypt"
	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/yamldoc"
	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
	appversion "github.com/gilmanlab/platform/tools/labctl/internal/app/version"
)

// Input describes process-level values used to build the dependency graph.
type Input struct {
	// Version is the release version embedded into the labctl binary.
	Version string
	// LookupEnv reads process environment variables.
	LookupEnv func(string) (string, bool)
	// HTTPClient is the HTTP client used by network adapters.
	HTTPClient *http.Client
}

// Dependencies groups app services for CLI command construction.
type Dependencies struct {
	// Version provides labctl build-version information.
	Version appversion.Service
	// Secrets provides reusable encrypted secret fetching.
	Secrets appsecrets.Service
}

// New wires app services to their concrete adapters.
func New(input Input) Dependencies {
	lookupEnv := input.LookupEnv
	if lookupEnv == nil {
		lookupEnv = func(string) (string, bool) {
			return "", false
		}
	}

	broker := githubbroker.NewProvider(nil)
	authProvider := githubauth.NewProvider(lookupEnv, broker)
	githubAPIBaseURL, _ := lookupEnv(githubcontents.EnvAPIBaseURL)

	return Dependencies{
		Version: appversion.NewService(input.Version),
		Secrets: appsecrets.NewService(appsecrets.Dependencies{
			Local:          secretslocal.NewSource(lookupEnv),
			Remote:         githubcontents.NewSource(input.HTTPClient, authProvider, githubAPIBaseURL),
			Decrypter:      sopsdecrypt.Decrypter{},
			FieldExtractor: yamldoc.Extractor{},
		}),
	}
}
