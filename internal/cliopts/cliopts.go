// Package cliopts lets the commands of this module set the render options
// that have no public option, such as the debug attributes, without exposing
// them to other modules.
package cliopts

import "github.com/Notifuse/safe-mjml/internal/options"

// New returns an mjml.Option, as an any, that applies set to the render
// options of a compilation. The mjml package sets it when it is loaded; it is
// nil in a program that does not import mjml.
var New func(set func(*options.RenderOpts)) any
