// Package options contains render options for MJML components
package options

import (
	"sort"

	"github.com/Notifuse/safe-mjml/internal/globals"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// RenderOpts holds the options of one render, from the mjml package's
// options (WithKeepComments, WithIDGenerator), and the state the render
// shares between components. The commands of this module also set DebugTags
// and WarningHandler, which have no public option (internal/cliopts).
type RenderOpts struct {
	DebugTags        bool                      // Whether to include debug attributes in output
	Lang             string                    // Language attribute from root MJML element
	Dir              string                    // Direction attribute from root MJML element, auto by default
	Title            string                    // Document title extracted from <mj-title>
	WarningHandler   func(Warning)             // Receives validation warnings (unknown or misplaced tags, invalid attributes or values)
	GlobalAttributes *globals.GlobalAttributes // Per-render global attributes (mj-all, mj-class, component defaults)

	// SkipComments drops the HTML comments of the template, as MJML's
	// keepComments: false; set it with mjml.WithKeepComments. Comments inside
	// ending tags (mj-text, mj-button, mj-raw…) are content and stay.
	SkipComments bool

	// IDGenerator, when set, provides the 16-hex-digit IDs that mj-carousel and
	// the checkbox of an mj-navbar with hamburger need, instead of random ones.
	// It is called once per such element, from the rendering goroutine, in the
	// order the IDs first appear in the output. A value that is not 16
	// lowercase hex digits is replaced by a random ID. A generator shared by
	// concurrent renders must be safe for concurrent use.
	IDGenerator func() string

	// AttributePassMarker, set while the body of a template with
	// mj-html-attributes is written, makes the components write it as the
	// XML that cheerio reads in MJML's unminified body: void elements and
	// the fluid hero's spacer cells closed with "/>", and the marker inside
	// each element MJML writes with nothing but whitespace in it, so that
	// it is not empty. The marker, a character no template can hold, is
	// removed after the pass.
	AttributePassMarker string

	// Limits bounds the render; a zero field is safe-mjml's default (see
	// mjml.WithLimits).
	Limits Limits
}

// Limits are the limits of a render, as mjml.WithLimits sets them: zero
// for the default, negative for none.
type Limits struct {
	SelectorBytes int
	WorkSteps     int64
	OutputSize    int
}

// WarningKind classifies a validation warning.
type WarningKind string

const (
	// WarningUnknownTag: a tag MJML does not know; it is left out.
	WarningUnknownTag WarningKind = "unknown-tag"
	// WarningUnsupportedTag: a tag MJML knows but safe-mjml does not implement
	// (mj-include); it is left out, and
	// for mj-include, what it holds is rendered in its place, as in MJML 5.
	WarningUnsupportedTag WarningKind = "unsupported-tag"
	// WarningMisplacedTag: a known component where MJML does not allow it. As
	// with MJML's soft validation it is rendered all the same by its parent,
	// except directly under the mjml root, which only uses mj-head and
	// mj-body.
	WarningMisplacedTag WarningKind = "misplaced-tag"
	// WarningInvalidAttribute: an attribute the tag does not accept; it is
	// ignored.
	WarningInvalidAttribute WarningKind = "invalid-attribute"
	// WarningInvalidAttributeValue: a value that does not have the type of
	// its attribute (an enum value it does not list, a length without an
	// accepted unit, a color mjml-core does not know); it is used all the
	// same.
	WarningInvalidAttributeValue WarningKind = "invalid-attribute-value"
)

// Warning is a validation problem that does not stop rendering, like the
// errors MJML reports with its default soft validation. Messages follow
// mjml-validator's wording.
type Warning struct {
	Kind    WarningKind `json:"kind"`
	TagName string      `json:"tagName"`
	Line    int         `json:"line"`
	Message string      `json:"message"`

	position int64 // StartOffset of the element, to sort warnings in document order
}

// NewWarning returns a warning about the element node.
func NewWarning(kind WarningKind, node *parser.MJMLNode, message string) Warning {
	return Warning{
		Kind:     kind,
		TagName:  node.GetTagName(),
		Line:     node.GetLineNumber(),
		Message:  message,
		position: node.StartOffset,
	}
}

// SortWarnings orders warnings as their elements appear in the document,
// keeping the order of several warnings about one element.
func SortWarnings(warnings []Warning) {
	sort.SliceStable(warnings, func(i, j int) bool { return warnings[i].position < warnings[j].position })
}

// Warn passes a warning to WarningHandler, if any.
func (o *RenderOpts) Warn(w Warning) {
	if o != nil && o.WarningHandler != nil {
		o.WarningHandler(w)
	}
}
