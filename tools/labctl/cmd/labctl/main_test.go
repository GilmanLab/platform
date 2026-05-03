package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/ulikunitz/xz"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubcontents"
)

const defaultTalosSchematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

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
	talosArchive, err := xzBytes([]byte("talos-raw-image"))
	if err != nil {
		return err
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/GilmanLab/secrets/contents/network/keycloak.sops.yaml":
			handleSecretFixture(env, w, r)
		case "/image/" + defaultTalosSchematicID + "/v1.13.0/nocloud-amd64.raw.xz":
			_, _ = w.Write(talosArchive)
		default:
			http.Error(w, fmt.Sprintf("unexpected path %s", r.URL.Path), http.StatusNotFound)
		}
	}))
	env.Defer(server.Close)
	env.Setenv(githubcontents.EnvAPIBaseURL, server.URL)

	if err := os.WriteFile(
		filepath.Join(env.WorkDir, "controlplane.yaml"),
		[]byte("machine:\n  type: controlplane\n"),
		0o600,
	); err != nil {
		return err
	}
	if err := os.WriteFile(
		filepath.Join(env.WorkDir, "network-config.yaml"),
		[]byte("version: 1\n"),
		0o600,
	); err != nil {
		return err
	}
	validConfig := fmt.Appendf(nil, `name: talos-test
source:
  factoryURL: %s
  version: v1.13.0
config:
  userData:
    path: controlplane.yaml
  metaData:
    localHostname: bootstrap-controlplane-1
  networkConfig:
    path: network-config.yaml
output:
  dir: .state/images
  format: img
  bootArtifactName: talos-boot.img
  configArtifactName: talos-cidata.img
`, server.URL)
	if err := os.WriteFile(filepath.Join(env.WorkDir, "talos-valid.yaml"), validConfig, 0o600); err != nil {
		return err
	}

	return nil
}

func handleSecretFixture(env *testscript.Env, w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("ref") != "feature" {
		http.Error(w, fmt.Sprintf("unexpected ref %s", r.URL.Query().Get("ref")), http.StatusBadRequest)

		return
	}
	if r.Header.Get("Authorization") != "Bearer ghs_test" {
		http.Error(w, "unexpected authorization header", http.StatusUnauthorized)

		return
	}

	http.ServeFile(w, r, filepath.Join(env.WorkDir, "secrets/network/keycloak.sops.yaml"))
}

func xzBytes(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := xz.NewWriter(&buffer)
	if err != nil {
		return nil, err
	}
	_, err = writer.Write(data)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
