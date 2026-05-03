package talosimage

import (
	"context"
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
	err = s.downloadArchive(ctx, image.url, archivePath)
	if err != nil {
		return Result{}, err
	}
	err = s.writeBootImage(archivePath, paths.bootArtifactPath)
	if err != nil {
		return Result{}, err
	}

	payload, err := s.buildConfigDiskPayload(request.BaseDir, request.Config.Config)
	if err != nil {
		return Result{}, err
	}
	if err := s.configDisk.Build(paths.configArtifactPath, payload); err != nil {
		return Result{}, fmt.Errorf("build NoCloud cidata image %q: %w", paths.configArtifactPath, err)
	}

	return Result{
		Name:               string(request.Config.Name),
		BootArtifactPath:   paths.bootArtifactPath,
		ConfigArtifactPath: paths.configArtifactPath,
		SourceVersion:      image.version,
		SourceURL:          image.url,
		SourceSchematicID:  image.schematicID,
		Platform:           image.platform,
		Arch:               image.arch,
		Format:             string(request.Config.Output.Format),
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

func (s Service) downloadArchive(ctx context.Context, url string, destination string) error {
	exists, err := s.files.IsFile(destination)
	if err != nil {
		return fmt.Errorf("check cached archive %q: %w", destination, err)
	}
	if exists {
		return nil
	}

	source, err := s.upstream.Download(ctx, url)
	if err != nil {
		return fmt.Errorf("download Talos image %q: %w", url, err)
	}
	defer source.Close()

	target, err := s.files.Create(destination)
	if err != nil {
		return fmt.Errorf("create archive %q: %w", destination, err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("write archive %q: %w", destination, err)
	}

	return nil
}

func (s Service) writeBootImage(archivePath string, bootArtifactPath string) error {
	archive, err := s.files.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open compressed archive %q: %w", archivePath, err)
	}
	defer archive.Close()

	raw, err := xz.NewReader(archive)
	if err != nil {
		return fmt.Errorf("read xz archive %q: %w", archivePath, err)
	}

	target, err := s.files.Create(bootArtifactPath)
	if err != nil {
		return fmt.Errorf("create boot image %q: %w", bootArtifactPath, err)
	}
	defer target.Close()

	if _, err := io.Copy(target, raw); err != nil {
		return fmt.Errorf("decompress boot image %q: %w", bootArtifactPath, err)
	}

	return nil
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
	outputDir, err := resolvePath(baseDir, string(output.Dir))
	if err != nil {
		return buildPaths{}, err
	}

	return buildPaths{
		downloadsDir:       filepath.Join(filepath.Dir(outputDir), "downloads", "talos"),
		outputDir:          outputDir,
		bootArtifactPath:   filepath.Join(outputDir, string(output.BootArtifactName)),
		configArtifactPath: filepath.Join(outputDir, string(output.ConfigArtifactName)),
	}, nil
}

func resolvePath(baseDir string, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if baseDir == "" {
		return "", errors.New("base directory is required for relative paths")
	}

	return filepath.Abs(filepath.Join(baseDir, path))
}
