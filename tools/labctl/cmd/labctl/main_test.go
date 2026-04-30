package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubcontents"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"labctl": func() {
			os.Exit(run())
		},
	})
}

func TestCLI(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:   "testdata/script",
		Setup: setupTestScript,
	})
}

func setupTestScript(env *testscript.Env) error {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/GilmanLab/secrets/contents/network/keycloak.sops.yaml" {
			http.Error(w, fmt.Sprintf("unexpected path %s", r.URL.Path), http.StatusNotFound)

			return
		}
		if r.URL.Query().Get("ref") != "feature" {
			http.Error(w, fmt.Sprintf("unexpected ref %s", r.URL.Query().Get("ref")), http.StatusBadRequest)

			return
		}
		if r.Header.Get("Authorization") != "Bearer ghs_test" {
			http.Error(w, "unexpected authorization header", http.StatusUnauthorized)

			return
		}

		http.ServeFile(w, r, filepath.Join(env.WorkDir, "secrets/network/keycloak.sops.yaml"))
	}))
	env.Defer(server.Close)
	env.Setenv(githubcontents.EnvAPIBaseURL, server.URL)

	return nil
}
