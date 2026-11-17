package command

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// errReported is the error of a command that has already printed what went
// wrong, without the usage.
var errReported = errors.New("reported")

// Execute runs the root command. Cobra has printed the error, if any.
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

// NewRootCommand creates the root command and its subcommands. Given a file,
// the root command compiles it, as compile does, with the same flags.
func NewRootCommand() *cobra.Command {
	var opts compileOptions
	rootCmd := &cobra.Command{
		Use:   "safe-mjml [input]",
		Short: "MJML compiler written in Go - converts MJML to responsive HTML",
		Long: `safe-mjml is a native Go implementation of the MJML email framework.
It compiles MJML markup into responsive HTML suitable for email clients.

Available Commands:
  compile    Compile MJML to HTML (default: safe-mjml input.mjml)
  test       Run the integration tests against MJML 5.4.1
  version    Show version information`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return opts.compile(cmd, args[0])
		},
	}
	opts.addFlags(rootCmd)

	// Add subcommands
	rootCmd.AddCommand(NewCompileCommand())
	rootCmd.AddCommand(NewTestCommand())
	rootCmd.AddCommand(NewVersionCommand())

	return rootCmd
}

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "safe-mjml %s (MJML 5.4.1)\n", version())
		},
	}
}

// version returns the version of the module the binary was built from: its
// tag for go install, a pseudo-version for a build in a clone, (devel) when
// Go records none.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

// failed prints a message on the error output of cmd and returns
// errReported, so that the command exits with status 1.
func failed(cmd *cobra.Command, format string, a ...any) error {
	fmt.Fprintf(cmd.ErrOrStderr(), format, a...)
	return reported(cmd)
}

// reported returns errReported, for a command that has printed its error,
// and keeps cobra from printing it again with the usage.
func reported(cmd *cobra.Command) error {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return errReported
}
