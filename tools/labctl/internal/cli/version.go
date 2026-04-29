package cli

import (
	"github.com/spf13/cobra"

	appversion "github.com/gilmanlab/platform/tools/labctl/internal/app/version"
)

func newVersionCommand(service appversion.Service, opts Options, flags *rootFlags) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print labctl version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			info := service.Info()
			if jsonOutput {
				return writeJSON(opts.Stdout, info)
			}
			if flags.quiet {
				return nil
			}

			return writeLine(opts.Stdout, "labctl %s", info.Version)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print version as JSON")

	return cmd
}
