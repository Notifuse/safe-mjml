package command

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// errReported is the error of a command that has already printed what went
// wrong: Execute exits with status 1 without printing it again.
var errReported = errors.New("reported")

// Execute runs the root command
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		if !errors.Is(err, errReported) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

// NewRootCommand creates the root command and its subcommands
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "safe-mjml",
		Short: "MJML compiler written in Go - converts MJML to responsive HTML",
		Long: `safe-mjml is a native Go implementation of the MJML email framework.
It compiles MJML markup into responsive HTML suitable for email clients.

Available Commands:
  compile    Compile MJML to HTML (default)
  test       Run the integration tests against MJML 4.18
  version    Show version information`,
	}

	// Add subcommands
	rootCmd.AddCommand(NewCompileCommand())
	rootCmd.AddCommand(NewTestCommand())

	// If no command is specified, default to compile
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		// Default to compile command behavior
		compileCmd := NewCompileCommand()
		return compileCmd.RunE(cmd, args)
	}

	return rootCmd
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
