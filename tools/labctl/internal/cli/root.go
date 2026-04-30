package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/gilmanlab/platform/tools/labctl/internal/composition"
)

const (
	exitSuccess = 0
	exitFailure = 1
)

// Options describes process-level inputs for running labctl.
type Options struct {
	// Version is the release version embedded into the labctl binary.
	Version string
	// LookupEnv reads process environment variables.
	LookupEnv func(string) (string, bool)
	// Stdin provides command input.
	Stdin io.Reader
	// Stdout receives command results.
	Stdout io.Writer
	// Stderr receives diagnostics, prompts, progress, and errors.
	Stderr io.Writer
}

type rootFlags struct {
	configFile     string
	nonInteractive bool
	quiet          bool
	verbosity      int
}

// Run executes labctl with the supplied arguments and process-level options.
func Run(ctx context.Context, args []string, opts Options) int {
	if opts.LookupEnv == nil {
		opts.LookupEnv = os.LookupEnv
	}

	deps := composition.New(composition.Input{
		Version:   opts.Version,
		LookupEnv: opts.LookupEnv,
	})

	root := newRootCommand(deps, opts)
	root.SetArgs(args)
	root.SetIn(opts.Stdin)
	root.SetOut(opts.Stdout)
	root.SetErr(opts.Stderr)

	if err := root.ExecuteContext(ctx); err != nil {
		renderError(opts.Stderr, err)
		return exitFailure
	}

	return exitSuccess
}

func newRootCommand(deps composition.Dependencies, opts Options) *cobra.Command {
	vp := viper.New()
	flags := rootFlags{}

	cmd := &cobra.Command{
		Use:           "labctl",
		Short:         "Operate the GilmanLab homelab",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return initializeConfig(cmd, vp, flags.configFile)
		},
	}

	bindGlobalFlags(cmd, &flags)
	cmd.AddCommand(newBootstrapCommand(deps, opts, &flags))
	cmd.AddCommand(newVersionCommand(deps.Version, opts, &flags))
	cmd.AddCommand(newSecretsCommand(deps.Secrets, opts))

	return cmd
}

func bindGlobalFlags(cmd *cobra.Command, flags *rootFlags) {
	cmd.PersistentFlags().CountVarP(&flags.verbosity, "verbose", "v", "increase log verbosity")
	cmd.PersistentFlags().BoolVarP(&flags.quiet, "quiet", "q", false, "suppress human-facing output")
	cmd.PersistentFlags().BoolVarP(
		&flags.nonInteractive,
		"non-interactive",
		"i",
		false,
		"disable prompts and terminal styling for automation",
	)
	cmd.PersistentFlags().StringVar(&flags.configFile, "config", "", "config file path")
}

func initializeConfig(cmd *cobra.Command, vp *viper.Viper, configFile string) error {
	vp.SetEnvPrefix("LABCTL")
	vp.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	vp.AutomaticEnv()

	if configFile != "" {
		vp.SetConfigFile(configFile)
	}

	if err := vp.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	if err := vp.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}

	return nil
}
