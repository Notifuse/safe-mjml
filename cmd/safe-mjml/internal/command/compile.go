package command

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Notifuse/safe-mjml/internal/cliopts"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/mjml"
	"github.com/spf13/cobra"
)

// NewCompileCommand creates the compile command
func NewCompileCommand() *cobra.Command {
	var (
		outputFile string
		stdout     bool
		debug      bool
		strict     bool
	)

	cmd := &cobra.Command{
		Use:   "compile [input]",
		Short: "Compile MJML to HTML",
		Long: `Compile MJML markup to responsive HTML.

Examples:
  safe-mjml compile input.mjml -o output.html
  safe-mjml compile input.mjml -s
  safe-mjml compile input.mjml --debug
  safe-mjml compile input.mjml -o output.html --strict

Validation problems (unknown or misplaced tags, invalid attributes or values) are
printed to stderr as warnings, as MJML's soft validation does; the HTML is
still written. With --strict, the command exits with status 1 when there is
at least one warning.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]

			// Read MJML file
			mjmlContent, err := os.ReadFile(inputFile)
			if err != nil {
				return failed(cmd, "Error reading file: %v\n", err)
			}

			// Render MJML to HTML using library
			var opts []mjml.Option
			if debug {
				// The debug attributes have no public option.
				opts = append(opts, internalOption(func(o *options.RenderOpts) { o.DebugTags = true }))
			}
			html, warnings, err := renderWithWarnings(string(mjmlContent), inputFile, cmd.ErrOrStderr(), opts...)
			if err != nil {
				printRenderError(cmd.ErrOrStderr(), err)
				return reported(cmd)
			}

			// Output HTML
			if outputFile != "" {
				err := os.WriteFile(outputFile, []byte(html), 0o644)
				if err != nil {
					return failed(cmd, "Error writing output file: %v\n", err)
				}
			} else {
				fmt.Fprint(cmd.OutOrStdout(), html)
			}

			if strict && warnings > 0 {
				return failed(cmd, "%d validation warning(s), failing because of --strict\n", warnings)
			}
			return nil
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file path")
	cmd.Flags().BoolVarP(&stdout, "stdout", "s", false, "output to stdout")
	cmd.Flags().BoolVar(&debug, "debug", false, "include debug attributes in output")
	cmd.Flags().BoolVar(&strict, "strict", false, "exit with status 1 when there are validation warnings")

	return cmd
}

// renderWithWarnings renders an MJML document and prints each validation
// warning to out as "file:line: warning: (tag) message", returning how many
// there were. The warnings found before a failure come with its error, and
// are printed too.
func renderWithWarnings(content, inputFile string, out io.Writer, opts ...mjml.Option) (string, int, error) {
	res, err := mjml.Compile(content, opts...)
	var warnings []mjml.Warning
	var mjmlErr *mjml.Error
	switch {
	case err == nil:
		warnings = res.Warnings
	case errors.As(err, &mjmlErr):
		warnings = mjmlErr.Warnings
	}
	for _, w := range warnings {
		fmt.Fprintf(out, "%s:%d: warning: (%s) %s\n", inputFile, w.Line, w.TagName, w.Message)
	}
	if err != nil {
		return "", len(warnings), err
	}
	return res.HTML, len(warnings), nil
}

// internalOption returns an mjml option that sets render options the public
// API does not offer.
func internalOption(set func(*options.RenderOpts)) mjml.Option {
	return cliopts.New(set).(mjml.Option)
}

// printRenderError writes a render error, and the stack of a panic inside
// safe-mjml (an mjml.ErrInternal), which a bug report needs.
func printRenderError(w io.Writer, err error) {
	fmt.Fprintf(w, "Error rendering MJML: %v\n", err)
	var mjmlErr *mjml.Error
	if errors.Is(err, mjml.ErrInternal) && errors.As(err, &mjmlErr) && mjmlErr.Stack != "" {
		fmt.Fprintf(w, "\nThis is a bug of safe-mjml: please report it with the template and this stack.\n%s", mjmlErr.Stack)
	}
}
