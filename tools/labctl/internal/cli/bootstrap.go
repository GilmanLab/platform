package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubbroker"
	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/secretrefs"
	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
	"github.com/gilmanlab/platform/tools/labctl/internal/composition"
)

const stdinName = "-"

type imageBuildOutput struct {
	Name          string `json:"name"`
	ArtifactPath  string `json:"artifactPath"`
	SourceVersion string `json:"sourceVersion"`
	SourceURL     string `json:"sourceURL"`
	SourceSHA256  string `json:"sourceSHA256"`
}

type imageBuildSecretsFlags struct {
	ref            string
	source         string
	repoDir        string
	brokerFunction string
	awsRegion      string
}

func newBootstrapCommand(deps composition.Dependencies, opts Options, flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Operate bootstrap workflows",
	}

	cmd.AddCommand(newBootstrapIncusOSCommand(deps, opts, flags))

	return cmd
}

func newBootstrapIncusOSCommand(deps composition.Dependencies, opts Options, flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "incusos",
		Short: "Build IncusOS bootstrap artifacts",
	}

	cmd.AddCommand(newBootstrapIncusOSImageCommand(deps, opts, flags))

	return cmd
}

func newBootstrapIncusOSImageCommand(deps composition.Dependencies, opts Options, flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Build IncusOS images",
	}

	cmd.AddCommand(newBootstrapIncusOSImageBuildCommand(deps, opts, flags))

	return cmd
}

func newBootstrapIncusOSImageBuildCommand(
	deps composition.Dependencies,
	opts Options,
	flags *rootFlags,
) *cobra.Command {
	var jsonOutput bool
	secretsFlags := imageBuildSecretsFlags{
		ref:    appsecrets.DefaultRef,
		source: string(appsecrets.SourceAuto),
	}

	cmd := &cobra.Command{
		Use:   "build <config.yaml>",
		Short: "Build a seeded IncusOS image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input, err := readImageBuildInput(args[0], opts)
			if err != nil {
				return err
			}

			config, err := deps.IncusOSConfig.ValidateYAML(input.name, input.data)
			if err != nil {
				return err
			}

			result, err := deps.IncusOSImage.Build(cmd.Context(), incusosimage.Request{
				Config:  config,
				BaseDir: input.baseDir,
				Secrets: secretrefs.NewResolver(deps.Secrets, secretrefs.Options{
					Ref:          secretsFlags.ref,
					Source:       appsecrets.SourceMode(secretsFlags.source),
					LocalRepoDir: secretsFlags.repoDir,
					BrokerFunction: envOrDefault(
						opts,
						envBrokerFunction,
						secretsFlags.brokerFunction,
						githubbroker.DefaultFunctionName,
					),
					AWSRegion: envOrDefault(opts, envAWSRegion, secretsFlags.awsRegion, ""),
				}),
			})
			if err != nil {
				return err
			}

			return renderImageBuildResult(result, opts, flags, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print build result as JSON")
	cmd.Flags().
		StringVar(&secretsFlags.source, "secrets-source", string(appsecrets.SourceAuto), "secret source: auto, local, or github")
	cmd.Flags().StringVar(&secretsFlags.ref, "secrets-ref", appsecrets.DefaultRef, "Git ref for GitHub secret fetches")
	cmd.Flags().
		StringVar(&secretsFlags.repoDir, "secrets-repo-dir", "", "local checkout path for the secrets repository")
	cmd.Flags().StringVar(
		&secretsFlags.brokerFunction,
		"broker-function",
		"",
		"GitHub token broker Lambda function name",
	)
	cmd.Flags().StringVar(
		&secretsFlags.awsRegion,
		"aws-region",
		"",
		"AWS region override for broker invocation",
	)

	return cmd
}

type imageBuildInput struct {
	name    string
	data    []byte
	baseDir string
}

func readImageBuildInput(path string, opts Options) (imageBuildInput, error) {
	if path == stdinName {
		baseDir, err := os.Getwd()
		if err != nil {
			return imageBuildInput{}, fmt.Errorf("resolve current directory: %w", err)
		}

		data, err := io.ReadAll(opts.Stdin)
		if err != nil {
			return imageBuildInput{}, fmt.Errorf("read image build config from stdin: %w", err)
		}

		return imageBuildInput{
			name:    "stdin.yaml",
			data:    data,
			baseDir: baseDir,
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return imageBuildInput{}, fmt.Errorf("read image build config %q: %w", path, err)
	}

	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return imageBuildInput{}, fmt.Errorf("resolve config directory for %q: %w", path, err)
	}

	return imageBuildInput{
		name:    path,
		data:    data,
		baseDir: baseDir,
	}, nil
}

func renderImageBuildResult(result incusosimage.Result, opts Options, flags *rootFlags, jsonOutput bool) error {
	output := imageBuildOutput{
		Name:          result.Name,
		ArtifactPath:  result.ArtifactPath,
		SourceVersion: result.SourceVersion,
		SourceURL:     result.SourceURL,
		SourceSHA256:  result.SourceSHA256,
	}

	if jsonOutput {
		return writeJSON(opts.Stdout, output)
	}
	if flags.quiet {
		return nil
	}

	return writeLine(opts.Stdout, "%s", result.ArtifactPath)
}
