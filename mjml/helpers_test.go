package mjml

import (
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// withWarningHandler passes every validation warning to handle as it is
// delivered, as the CLI does through internal/cliopts.
func withWarningHandler(handle func(Warning)) Option {
	return func(c *config) {
		c.internal = append(c.internal, func(o *options.RenderOpts) {
			o.WarningHandler = func(w options.Warning) {
				handle(publicWarnings([]options.Warning{w})[0])
			}
		})
	}
}

// newComponentTree builds the component tree of ast as a compilation does,
// for the tests that look at the tree or render it more than once.
func newComponentTree(ast *parser.MJMLNode, opts ...Option) (components.Component, error) {
	renderOpts, warnings := newRenderOpts(newConfig(opts))
	component, err := buildComponentTree(ast, renderOpts)
	warnings.flush()
	return component, err
}

// renderComponentString renders a component tree as a compilation does.
func renderComponentString(c components.Component) (string, error) {
	var sb strings.Builder
	if err := renderMerged(c, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// mustParse parses an MJML template or fails the test.
func mustParse(t testing.TB, template string) *parser.MJMLNode {
	t.Helper()
	ast, err := parser.ParseMJML(template)
	if err != nil {
		t.Fatalf("ParseMJML: %v", err)
	}
	return ast
}
