package incusosimage

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	schemaincusos "github.com/gilmanlab/platform/schemas/lab/incusos"
)

const (
	componentOS      = "os"
	compressedSuffix = ".gz"
	fileModeDir      = 0o755
	imageTypeRaw     = "image-raw"
	versionLatest    = "latest"
)

// Service builds seeded IncusOS image artifacts.
type Service struct {
	upstream Upstream
	files    FileSystem
}

// NewService constructs a Service from external adapters.
func NewService(deps Dependencies) Service {
	return Service{
		upstream: deps.Upstream,
		files:    deps.Files,
	}
}

// Build builds a seeded IncusOS image artifact.
func (s Service) Build(ctx context.Context, request Request) (Result, error) {
	if err := s.validate(); err != nil {
		return Result{}, err
	}

	paths, err := resolvePaths(request.BaseDir, request.Config.Output)
	if err != nil {
		return Result{}, err
	}

	if prepareErr := s.prepareDirectories(paths); prepareErr != nil {
		return Result{}, prepareErr
	}

	image, err := s.selectImage(ctx, request.Config.Source)
	if err != nil {
		return Result{}, err
	}

	archivePath := filepath.Join(paths.downloadsDir, image.filename)
	if downloadErr := s.downloadArchive(ctx, image.url, archivePath); downloadErr != nil {
		return Result{}, downloadErr
	}

	if verifyErr := s.verifySHA256(archivePath, image.sha256); verifyErr != nil {
		return Result{}, verifyErr
	}

	seed, err := s.buildSeed(ctx, request.Config.Seed, request.Secrets)
	if err != nil {
		return Result{}, err
	}

	if err := s.writeRawImage(
		archivePath,
		paths.rawPath,
		request.Config.Output,
		request.Config.Seed,
		seed,
	); err != nil {
		return Result{}, err
	}

	if err := s.compressRawImage(paths.rawPath, paths.artifactPath); err != nil {
		return Result{}, err
	}

	return Result{
		Name:          string(request.Config.Name),
		ArtifactPath:  paths.artifactPath,
		SourceVersion: image.version,
		SourceURL:     image.url,
		SourceSHA256:  image.sha256,
	}, nil
}

func (s Service) validate() error {
	if s.upstream == nil {
		return errors.New("incusos image builder missing upstream adapter")
	}
	if s.files == nil {
		return errors.New("incusos image builder missing filesystem adapter")
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

func (s Service) selectImage(ctx context.Context, source schemaincusos.ImageSource) (sourceImage, error) {
	index, err := s.upstream.FetchIndex(ctx, string(source.IndexURL))
	if err != nil {
		return sourceImage{}, fmt.Errorf("fetch IncusOS image index %q: %w", source.IndexURL, err)
	}

	image, err := selectImage(index, source)
	if err != nil {
		return sourceImage{}, err
	}

	return image, nil
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
		return fmt.Errorf("download IncusOS image %q: %w", url, err)
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

func (s Service) verifySHA256(path string, expected string) error {
	source, err := s.files.Open(path)
	if err != nil {
		return fmt.Errorf("open archive %q: %w", path, err)
	}
	defer source.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, source); err != nil {
		return fmt.Errorf("hash archive %q: %w", path, err)
	}

	actual := hex.EncodeToString(digest.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch for %q: expected %s, got %s", path, expected, actual)
	}

	return nil
}

func (s Service) writeRawImage(
	archivePath string,
	rawPath string,
	output schemaincusos.ImageOutput,
	seedConfig schemaincusos.Seed,
	seed []byte,
) error {
	size, err := parseSize(string(output.Size))
	if err != nil {
		return fmt.Errorf("parse image size %q: %w", output.Size, err)
	}

	archive, err := s.files.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open compressed archive %q: %w", archivePath, err)
	}
	defer archive.Close()

	compressed, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("read gzip archive %q: %w", archivePath, err)
	}
	defer compressed.Close()

	raw, err := s.files.Create(rawPath)
	if err != nil {
		return fmt.Errorf("create raw image %q: %w", rawPath, err)
	}
	written, err := io.Copy(raw, io.LimitReader(compressed, size+1))
	if err != nil {
		_ = raw.Close()
		return fmt.Errorf("decompress raw image %q: %w", rawPath, err)
	}
	if written > size {
		_ = raw.Close()
		return fmt.Errorf("decompressed image %q is larger than configured size %q", rawPath, output.Size)
	}
	if err := raw.Close(); err != nil {
		return fmt.Errorf("close raw image %q: %w", rawPath, err)
	}

	return s.mutateRawImage(rawPath, size, seedConfig, seed)
}

func (s Service) mutateRawImage(
	rawPath string,
	size int64,
	seedConfig schemaincusos.Seed,
	seed []byte,
) error {
	raw, err := s.files.OpenReadWrite(rawPath)
	if err != nil {
		return fmt.Errorf("open raw image %q: %w", rawPath, err)
	}
	defer raw.Close()

	if err := raw.Truncate(size); err != nil {
		return fmt.Errorf("resize raw image %q: %w", rawPath, err)
	}
	if int64(seedConfig.Offset)+int64(len(seed)) > size {
		return fmt.Errorf("seed data at offset %d exceeds configured image size", seedConfig.Offset)
	}
	if _, err := raw.Seek(int64(seedConfig.Offset), io.SeekStart); err != nil {
		return fmt.Errorf("seek seed offset %d in %q: %w", seedConfig.Offset, rawPath, err)
	}
	if _, err := raw.Write(seed); err != nil {
		return fmt.Errorf("write seed data into %q: %w", rawPath, err)
	}

	return nil
}

func (s Service) compressRawImage(rawPath string, artifactPath string) error {
	source, err := s.files.Open(rawPath)
	if err != nil {
		return fmt.Errorf("open raw image %q: %w", rawPath, err)
	}
	defer source.Close()

	target, err := s.files.Create(artifactPath)
	if err != nil {
		return fmt.Errorf("create compressed artifact %q: %w", artifactPath, err)
	}
	defer target.Close()

	compressed, err := gzip.NewWriterLevel(target, gzip.BestSpeed)
	if err != nil {
		return fmt.Errorf("create gzip writer for %q: %w", artifactPath, err)
	}
	compressed.Name = ""
	compressed.ModTime = time.Unix(0, 0)

	if _, err := io.Copy(compressed, source); err != nil {
		_ = compressed.Close()
		return fmt.Errorf("compress raw image %q: %w", rawPath, err)
	}
	if err := compressed.Close(); err != nil {
		return fmt.Errorf("close gzip artifact %q: %w", artifactPath, err)
	}

	return nil
}

func selectImage(index Index, source schemaincusos.ImageSource) (sourceImage, error) {
	for _, update := range index.Updates {
		if !matchesUpdate(update, source) {
			continue
		}
		for _, file := range update.Files {
			if matchesFile(file, source) {
				return buildSourceImage(source, update, file), nil
			}
		}
	}

	return sourceImage{}, fmt.Errorf(
		"could not find %s IncusOS raw image for architecture %s and version %s",
		source.Channel,
		source.Arch,
		source.Version,
	)
}

func matchesUpdate(update Update, source schemaincusos.ImageSource) bool {
	if string(source.Version) != versionLatest && update.Version != string(source.Version) {
		return false
	}

	return slices.Contains(update.Channels, source.Channel)
}

func matchesFile(file File, source schemaincusos.ImageSource) bool {
	return file.Architecture == source.Arch &&
		file.Component == componentOS &&
		file.Type == imageTypeRaw &&
		file.Filename != "" &&
		file.SHA256 != ""
}

func buildSourceImage(source schemaincusos.ImageSource, update Update, file File) sourceImage {
	return sourceImage{
		version:  update.Version,
		url:      strings.TrimRight(string(source.BaseURL), "/") + update.URL + "/" + file.Filename,
		filename: file.Filename,
		sha256:   file.SHA256,
	}
}

type buildPaths struct {
	downloadsDir string
	outputDir    string
	rawPath      string
	artifactPath string
}

func resolvePaths(baseDir string, output schemaincusos.ImageOutput) (buildPaths, error) {
	outputDir, err := resolvePath(baseDir, string(output.Dir))
	if err != nil {
		return buildPaths{}, err
	}

	artifactPath := filepath.Join(outputDir, string(output.ArtifactName))
	rawPath := strings.TrimSuffix(artifactPath, compressedSuffix)
	if rawPath == artifactPath {
		return buildPaths{}, fmt.Errorf("artifact name %q must end with %q", output.ArtifactName, compressedSuffix)
	}

	return buildPaths{
		downloadsDir: filepath.Join(filepath.Dir(outputDir), "downloads"),
		outputDir:    outputDir,
		rawPath:      rawPath,
		artifactPath: artifactPath,
	}, nil
}

func resolvePath(baseDir string, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if baseDir == "" {
		return "", errors.New("base directory is required for relative output paths")
	}

	return filepath.Abs(filepath.Join(baseDir, path))
}
