package incusosimage

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	schemaincusos "github.com/gilmanlab/platform/schemas/lab/incusos"
	"go.yaml.in/yaml/v4"
)

const tarFileMode = 0o644

type applicationsSeed struct {
	Version      string            `yaml:"version"`
	Applications []applicationSeed `yaml:"applications"`
}

type applicationSeed struct {
	Name string `yaml:"name"`
}

type incusSeed struct {
	Version       string       `yaml:"version"`
	ApplyDefaults bool         `yaml:"apply_defaults"`
	Preseed       incusPreseed `yaml:"preseed"`
}

type incusPreseed struct {
	Config       map[string]string `yaml:"config,omitempty"`
	Certificates []incusCert       `yaml:"certificates"`
}

type incusCert struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Certificate string `yaml:"certificate"`
}

func (s Service) buildSeed(ctx context.Context, seed schemaincusos.Seed, secrets SecretResolver) ([]byte, error) {
	if secrets == nil {
		return nil, errors.New("secret resolver is required")
	}

	applications, err := encodeApplications(seed.Applications)
	if err != nil {
		return nil, err
	}

	incus, err := encodeIncus(ctx, seed.Incus, secrets)
	if err != nil {
		return nil, err
	}

	return buildSeedTar([]seedFile{
		{name: "applications.yaml", payload: applications},
		{name: "incus.yaml", payload: incus},
	})
}

func encodeApplications(seed schemaincusos.ApplicationsSeed) ([]byte, error) {
	applications := make([]applicationSeed, 0, len(seed.Applications))
	for _, app := range seed.Applications {
		applications = append(applications, applicationSeed{
			Name: string(app.Name),
		})
	}

	data, err := yaml.Marshal(applicationsSeed{
		Version:      seed.Version,
		Applications: applications,
	})
	if err != nil {
		return nil, fmt.Errorf("encode applications seed: %w", err)
	}

	return data, nil
}

func encodeIncus(ctx context.Context, seed schemaincusos.IncusSeed, secrets SecretResolver) ([]byte, error) {
	certs := make([]incusCert, 0, len(seed.Preseed.Certificates))
	for _, cert := range seed.Preseed.Certificates {
		certificate, err := secrets.Resolve(ctx, SecretRef{
			Path:    string(cert.Certificate.SecretRef.Path),
			Pointer: string(cert.Certificate.SecretRef.Pointer),
		})
		if err != nil {
			return nil, fmt.Errorf("resolve trusted client certificate %q: %w", cert.Name, err)
		}

		certs = append(certs, incusCert{
			Name:        string(cert.Name),
			Type:        cert.Type,
			Certificate: certificate,
		})
	}

	data, err := yaml.Marshal(incusSeed{
		Version:       seed.Version,
		ApplyDefaults: seed.ApplyDefaults,
		Preseed: incusPreseed{
			Config:       seed.Preseed.Config,
			Certificates: certs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Incus seed: %w", err)
	}

	return data, nil
}

type seedFile struct {
	name    string
	payload []byte
}

func buildSeedTar(files []seedFile) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	archive := tar.NewWriter(buffer)

	for _, file := range files {
		header := &tar.Header{
			Name:    file.name,
			Mode:    tarFileMode,
			Size:    int64(len(file.payload)),
			ModTime: time.Unix(0, 0),
		}
		if err := archive.WriteHeader(header); err != nil {
			_ = archive.Close()
			return nil, fmt.Errorf("write seed tar header %q: %w", file.name, err)
		}
		if _, err := archive.Write(file.payload); err != nil {
			_ = archive.Close()
			return nil, fmt.Errorf("write seed tar payload %q: %w", file.name, err)
		}
	}

	if err := archive.Close(); err != nil {
		return nil, fmt.Errorf("close seed tar: %w", err)
	}

	return buffer.Bytes(), nil
}
