package talosimage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	schematalos "github.com/gilmanlab/platform/schemas/lab/talos"
	"github.com/ulikunitz/xz"
	"go.yaml.in/yaml/v4"
)

const (
	fileModeDir = 0o755

	// MaxBootImageBytes bounds the decompressed boot image to defend against
	// upstream artifacts whose decompressed size is unexpectedly large.
	MaxBootImageBytes int64 = 4 * 1024 * 1024 * 1024

	sha256Suffix = ".sha256"
)

// Service builds Talos image artifacts.
type Service struct {
	upstream   Upstream
	files      FileSystem
	configDisk ConfigDiskBuilder
}

// NewService constructs a Service from external adapters.
func NewService(deps Dependencies) Service {
	return Service{
		upstream:   deps.Upstream,
		files:      deps.Files,
		configDisk: deps.ConfigDisk,
	}
}

// Build builds Talos boot and NoCloud cidata image artifacts.
func (s Service) Build(ctx context.Context, request Request) (Result, error) {
	if err := s.validate(); err != nil {
		return Result{}, err
	}

	paths, err := resolvePaths(request.BaseDir, request.Config.Output)
	if err != nil {
		return Result{}, err
	}

	err = s.prepareDirectories(paths)
	if err != nil {
		return Result{}, err
	}

	image := buildSourceImage(request.Config.Source)
	archivePath := filepath.Join(paths.downloadsDir, image.schematicID, image.version, image.filename)
	err = s.prepareDownloadDirectory(archivePath)
	if err != nil {
		return Result{}, err
	}

	expectedSHA256, err := s.upstream.FetchSHA256(ctx, image.url+sha256Suffix)
	if err != nil {
		return Result{}, fmt.Errorf("fetch Talos image checksum: %w", err)
	}

	err = s.ensureCachedArchive(ctx, image.url, archivePath, expectedSHA256)
	if err != nil {
		return Result{}, err
	}

	bootSHA256, err := s.writeBootImage(archivePath, paths.bootArtifactPath)
	if err != nil {
		return Result{}, err
	}

	payload, err := s.buildConfigDiskPayload(request.BaseDir, request.Config.Config)
	if err != nil {
		return Result{}, err
	}
	err = s.configDisk.Build(paths.configArtifactPath, payload)
	if err != nil {
		return Result{}, fmt.Errorf("build NoCloud cidata image %q: %w", paths.configArtifactPath, err)
	}

	configSHA256, err := s.hashFile(paths.configArtifactPath)
	if err != nil {
		return Result{}, fmt.Errorf("hash NoCloud cidata image %q: %w", paths.configArtifactPath, err)
	}

	return Result{
		Name:                 string(request.Config.Name),
		BootArtifactPath:     paths.bootArtifactPath,
		BootArtifactSHA256:   bootSHA256,
		ConfigArtifactPath:   paths.configArtifactPath,
		ConfigArtifactSHA256: configSHA256,
		SourceVersion:        image.version,
		SourceURL:            image.url,
		SourceSchematicID:    image.schematicID,
		Platform:             image.platform,
		Arch:                 image.arch,
		Format:               string(request.Config.Output.Format),
	}, nil
}

func (s Service) validate() error {
	if s.upstream == nil {
		return errors.New("talos image builder missing upstream adapter")
	}
	if s.files == nil {
		return errors.New("talos image builder missing filesystem adapter")
	}
	if s.configDisk == nil {
		return errors.New("talos image builder missing config disk adapter")
	}

	return nil
}

func (s Service) prepareDirectories(paths buildPaths) error {
	if err := s.files.MkdirAll(paths.downloadsDir, fileModeDir); err != nil {
		return fmt.Errorf("create downloads directory %q: %w", paths.downloadsDir, err)
	}
	if err := s.files.MkdirAll(paths.outputDir, fileModeDir); err != nil {
		return fmt.Errorf("create output directory %q: %w", paths.outputDir, err)
	}

	return nil
}

func (s Service) prepareDownloadDirectory(archivePath string) error {
	dir := filepath.Dir(archivePath)
	if err := s.files.MkdirAll(dir, fileModeDir); err != nil {
		return fmt.Errorf("create download cache directory %q: %w", dir, err)
	}

	return nil
}

// ensureCachedArchive guarantees that destination contains the archive whose
// SHA256 matches expected. A cached file with a matching digest is reused; a
// cached file with a divergent digest is removed and redownloaded; a missing
// destination triggers a fresh download.
func (s Service) ensureCachedArchive(ctx context.Context, url, destination, expected string) error {
	exists, err := s.files.IsFile(destination)
	if err != nil {
		return fmt.Errorf("check cached archive %q: %w", destination, err)
	}
	if exists {
		actual, err := s.hashFile(destination)
		if err != nil {
			return fmt.Errorf("hash cached archive %q: %w", destination, err)
		}
		if actual == expected {
			return nil
		}
		if err := s.files.Remove(destination); err != nil {
			return fmt.Errorf("remove stale cached archive %q: %w", destination, err)
		}
	}

	return s.downloadArchive(ctx, url, destination, expected)
}

// downloadArchive streams the archive into a sibling temp file, hashing as
// it writes, and atomically renames into place only when the streamed digest
// matches expected. A leftover temp file from a failed run is removed on
// best effort; a stranded `.tmp-*` is harmless because subsequent runs
// create new temp files with unique suffixes.
func (s Service) downloadArchive(ctx context.Context, url, destination, expected string) error {
	parent := filepath.Dir(destination)
	tempName := "." + filepath.Base(destination) + ".tmp-*"

	temp, err := s.files.CreateTemp(parent, tempName)
	if err != nil {
		return fmt.Errorf("create temporary archive in %q: %w", parent, err)
	}
	tempPath := temp.Name()

	closed := false
	committed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		if !committed {
			_ = s.files.Remove(tempPath)
		}
	}()

	source, err := s.upstream.Download(ctx, url)
	if err != nil {
		return fmt.Errorf("download Talos image %q: %w", url, err)
	}
	defer source.Close()

	hasher := sha256.New()
	_, err = io.Copy(io.MultiWriter(temp, hasher), source)
	if err != nil {
		return fmt.Errorf("write temporary archive %q: %w", tempPath, err)
	}
	err = temp.Close()
	if err != nil {
		return fmt.Errorf("close temporary archive %q: %w", tempPath, err)
	}
	closed = true

	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch for %q: expected %s, got %s", url, expected, actual)
	}

	err = s.files.Rename(tempPath, destination)
	if err != nil {
		return fmt.Errorf("install archive %q: %w", destination, err)
	}
	committed = true

	return nil
}

// writeBootImage decompresses the cached xz archive into a temp file beside
// bootArtifactPath, hashes the decompressed bytes, bounds the output to
// MaxBootImageBytes, and atomically renames into place. The returned digest
// is the lowercase hex SHA256 of the decompressed boot image.
func (s Service) writeBootImage(archivePath, bootArtifactPath string) (string, error) {
	archive, err := s.files.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open compressed archive %q: %w", archivePath, err)
	}
	defer archive.Close()

	raw, err := xz.NewReader(archive)
	if err != nil {
		return "", fmt.Errorf("read xz archive %q: %w", archivePath, err)
	}

	parent := filepath.Dir(bootArtifactPath)
	tempName := "." + filepath.Base(bootArtifactPath) + ".tmp-*"

	temp, err := s.files.CreateTemp(parent, tempName)
	if err != nil {
		return "", fmt.Errorf("create temporary boot image in %q: %w", parent, err)
	}
	tempPath := temp.Name()

	closed := false
	committed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		if !committed {
			_ = s.files.Remove(tempPath)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hasher), io.LimitReader(raw, MaxBootImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("decompress boot image %q: %w", bootArtifactPath, err)
	}
	if written > MaxBootImageBytes {
		return "", fmt.Errorf(
			"decompressed boot image %q exceeds maximum size %d bytes",
			bootArtifactPath,
			MaxBootImageBytes,
		)
	}
	err = temp.Close()
	if err != nil {
		return "", fmt.Errorf("close temporary boot image %q: %w", tempPath, err)
	}
	closed = true

	err = s.files.Rename(tempPath, bootArtifactPath)
	if err != nil {
		return "", fmt.Errorf("install boot image %q: %w", bootArtifactPath, err)
	}
	committed = true

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s Service) hashFile(path string) (string, error) {
	source, err := s.files.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %q: %w", path, err)
	}
	defer source.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, source); err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s Service) buildConfigDiskPayload(baseDir string, config schematalos.MachineConfig) (ConfigDiskPayload, error) {
	userData, err := s.readConfigInput(baseDir, config.UserData, "user-data")
	if err != nil {
		return ConfigDiskPayload{}, err
	}

	metaData, err := buildMetaData(config.MetaData)
	if err != nil {
		return ConfigDiskPayload{}, err
	}

	var networkConfig []byte
	if config.NetworkConfig.Path != "" {
		networkConfig, err = s.readConfigInput(baseDir, config.NetworkConfig, "network-config")
		if err != nil {
			return ConfigDiskPayload{}, err
		}
	}

	return ConfigDiskPayload{
		UserData:      userData,
		MetaData:      metaData,
		NetworkConfig: networkConfig,
	}, nil
}

func (s Service) readConfigInput(baseDir string, input schematalos.FileInput, name string) ([]byte, error) {
	path, err := resolvePath(baseDir, string(input.Path))
	if err != nil {
		return nil, err
	}

	source, err := s.files.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open NoCloud %s input %q: %w", name, path, err)
	}
	defer source.Close()

	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf("read NoCloud %s input %q: %w", name, path, err)
	}

	return data, nil
}

func buildSourceImage(source schematalos.ImageSource) sourceImage {
	filename := fmt.Sprintf("%s-%s.%s", source.Platform, source.Arch, source.Artifact)
	url := strings.TrimRight(source.FactoryURL, "/") +
		"/image/" + source.SchematicID +
		"/" + string(source.Version) +
		"/" + filename

	return sourceImage{
		version:     string(source.Version),
		url:         url,
		filename:    filename,
		schematicID: source.SchematicID,
		platform:    string(source.Platform),
		arch:        string(source.Arch),
	}
}

type noCloudMetaData struct {
	InstanceID    string `yaml:"instance-id"`
	LocalHostname string `yaml:"local-hostname"`
}

func buildMetaData(metaData schematalos.NoCloudMetaData) ([]byte, error) {
	data, err := yaml.Marshal(noCloudMetaData{
		InstanceID:    metaData.InstanceID,
		LocalHostname: string(metaData.LocalHostname),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal NoCloud meta-data: %w", err)
	}

	return data, nil
}

type buildPaths struct {
	downloadsDir       string
	outputDir          string
	bootArtifactPath   string
	configArtifactPath string
}

func resolvePaths(baseDir string, output schematalos.ImageOutput) (buildPaths, error) {
	outputDir, err := resolvePath(baseDir, output.Dir)
	if err != nil {
		return buildPaths{}, err
	}

	return buildPaths{
		downloadsDir:       filepath.Join(filepath.Dir(outputDir), "downloads", "talos"),
		outputDir:          outputDir,
		bootArtifactPath:   filepath.Join(outputDir, output.BootArtifactName),
		configArtifactPath: filepath.Join(outputDir, output.ConfigArtifactName),
	}, nil
}

func resolvePath(baseDir, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if baseDir == "" {
		return "", errors.New("base directory is required for relative paths")
	}

	return filepath.Abs(filepath.Join(baseDir, path))
}
