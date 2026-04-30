package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/githubbroker"
	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

const (
	envBrokerFunction = "LABCTL_BROKER_FUNCTION"
	envAWSRegion      = "LABCTL_AWS_REGION"
)

type secretsGetFlags struct {
	field          string
	ref            string
	source         string
	repoDir        string
	brokerFunction string
	awsRegion      string
}

func newSecretsCommand(service appsecrets.Service, opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Fetch SOPS-encrypted lab secrets",
	}

	cmd.AddCommand(newSecretsGetCommand(service, opts))

	return cmd
}

func newSecretsGetCommand(service appsecrets.Service, opts Options) *cobra.Command {
	flags := secretsGetFlags{
		ref:    appsecrets.DefaultRef,
		source: string(appsecrets.SourceAuto),
	}

	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Decrypt a SOPS secret from the lab secrets repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := service.Get(cmd.Context(), appsecrets.Request{
				Path:         args[0],
				Ref:          flags.ref,
				Source:       appsecrets.SourceMode(flags.source),
				LocalRepoDir: flags.repoDir,
				Field:        flags.field,
				BrokerFunction: envOrDefault(
					opts,
					envBrokerFunction,
					flags.brokerFunction,
					githubbroker.DefaultFunctionName,
				),
				AWSRegion: envOrDefault(opts, envAWSRegion, flags.awsRegion, ""),
			})
			if err != nil {
				return err
			}

			if _, err := opts.Stdout.Write(result.Data); err != nil {
				return fmt.Errorf("write secret output: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&flags.field, "field", "", "RFC 6901 JSON Pointer to extract from the decrypted YAML")
	cmd.Flags().StringVar(&flags.ref, "ref", appsecrets.DefaultRef, "Git ref for GitHub source fetches")
	cmd.Flags().
		StringVar(&flags.source, "source", string(appsecrets.SourceAuto), "secret source: auto, local, or github")
	cmd.Flags().StringVar(&flags.repoDir, "repo-dir", "", "local checkout path for the secrets repository")
	cmd.Flags().StringVar(
		&flags.brokerFunction,
		"broker-function",
		"",
		"GitHub token broker Lambda function name",
	)
	cmd.Flags().StringVar(
		&flags.awsRegion,
		"aws-region",
		"",
		"AWS region override for broker invocation",
	)

	return cmd
}

func envOrDefault(opts Options, envName string, explicit string, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if value, ok := opts.LookupEnv(envName); ok && value != "" {
		return value
	}

	return fallback
}
