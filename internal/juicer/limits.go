package juicer

import (
	"fmt"
	"math"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/juicer/htmlparser2"
	"github.com/Notifuse/safe-mjml/internal/juicer/mensch"
	"golang.org/x/net/html"
)

// Limits on the input, so a hostile or broken document costs bounded time
// and memory. Unclosed tags nest without end, and matching a descendant
// selector walks every ancestor of every element, so depth multiplies the
// work.

const (
	defaultMaxInputSize = 16 << 20
	defaultMaxDepth     = 1024
)

// MaxInputSize limits the document Inline accepts, in bytes. Larger input
// fails with an error wrapping ErrLimitExceeded. The default is 16 MiB; 0
// removes the limit.
func MaxInputSize(n int) Option { return func(o *options) { o.maxInputSize = n } }

// MaxDepth limits how deeply elements, and the blocks and parentheses of a
// stylesheet, may nest. A deeper document fails with an error wrapping
// ErrLimitExceeded. The default is 1024; 0 removes the limit.
func MaxDepth(n int) Option { return func(o *options) { o.maxDepth = n } }

// MaxStylesheetDepth limits how deeply the blocks and parentheses of a
// stylesheet or a style attribute may nest, in place of MaxDepth: parsing
// nested at-rules and selectors costs more than linear time in their depth,
// and a real stylesheet nests a few levels. A deeper one fails with an error
// wrapping ErrLimitExceeded. The default, 0, leaves it to MaxDepth.
func MaxStylesheetDepth(n int) Option { return func(o *options) { o.maxStylesheetDepth = n } }

// MaxMatchSteps limits the work of matching the selectors to the document,
// in steps: each rule tested on an element, and each element, attribute list
// or node a selector looks at (see cascadia.Budget), and declarationSteps for
// each declaration an element gets. Matching costs rules times elements
// times the walk of each selector, so a stylesheet and a document chosen for
// it can make it cubic; past the limit, Inline fails with an error wrapping
// ErrLimitExceeded. The default, 0, has no limit.
func MaxMatchSteps(n int64) Option { return func(o *options) { o.maxMatchSteps = n } }

// declarationSteps is what a declaration applied to an element costs in
// MaxMatchSteps, with valueSteps for its value: it is held, with its value,
// until the document is written, and it is written into the output (100 KB
// of value on 500 elements wrote 50 MB).
const declarationSteps = 250

// stepsPerValueByte is what a byte of value costs, as a declaration applies
// it, as a CSS variable or a "$" pattern copies it, or as a template block
// is written again: 16, so that what a document gets is bounded by a
// sixteenth of MaxMatchSteps in bytes (15 MB under safe-mjml's limit), and
// held with a few times that in memory.
const stepsPerValueByte = 16

// valueSteps is what n bytes of value cost, saturated so that the product
// cannot overflow.
func valueSteps(n int) int {
	const most = math.MaxInt / stepsPerValueByte
	return min(n, most) * stepsPerValueByte
}

// bytesPerStep is how many bytes read cost a step where a name or a text is
// compared, as in cascadia.
const bytesPerStep = 16

func checkSize(doc []byte, o *options) error {
	if o.maxInputSize > 0 && len(doc) > o.maxInputSize {
		return fmt.Errorf("%w: the document is %d bytes, over MaxInputSize %d", ErrLimitExceeded, len(doc), o.maxInputSize)
	}
	return nil
}

// parseDocument parses src, refusing elements nested deeper than MaxDepth as
// the parse reaches them: measured after the parse, a deep document with many
// close tags cost seconds before its refusal.
func parseDocument(src []byte, o *options) (*html.Node, error) {
	root, err := htmlparser2.Parse(src, o.xmlMode, o.maxDepth)
	if err != nil {
		return nil, fmt.Errorf("%w: the document nests more than %d deep", ErrLimitExceeded, o.maxDepth)
	}
	return root, nil
}

// checkDepth measures element nesting, and the nesting of every stylesheet
// and style attribute the parsers would read. Stylesheets are measured
// joined, as compatibility modes read them.
func checkDepth(root *html.Node, o *options) error {
	cssLimit := o.maxStylesheetDepth
	if cssLimit <= 0 {
		cssLimit = o.maxDepth
	}
	if o.maxDepth <= 0 && cssLimit <= 0 {
		return nil
	}
	depth, deepest, deepestCSS := 0, 0, 0
	var css strings.Builder
	css.WriteString(o.extraCSS)
	htmlparser2.Traverse(root, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		depth++
		deepest = max(deepest, depth)
		if n.Data == "style" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
			css.WriteString(n.FirstChild.Data)
		}
		if v, ok := getAttr(n, o.styleAttributeName); ok {
			deepestCSS = max(deepestCSS, blockDepth(v))
		}
		return true
	}, func(n *html.Node) {
		if n.Type == html.ElementNode {
			depth--
		}
	})
	deepestCSS = max(deepestCSS, blockDepth(css.String()))
	if o.mensch && cssLimit > 0 && deepestCSS <= cssLimit {
		// mensch nests at-rules deeper than their braces do, and its parse
		// and stringifier recurse once per level: MJML's stack overflows
		// on a long enough run of nested at-rules.
		deepestCSS = max(deepestCSS, mensch.Depth(mensch.Lex(css.String())))
	}
	if cssLimit > 0 && deepestCSS > cssLimit {
		return fmt.Errorf("%w: a stylesheet nests %d deep, over %d", ErrLimitExceeded, deepestCSS, cssLimit)
	}
	if o.maxDepth > 0 && deepest > o.maxDepth {
		return fmt.Errorf("%w: the document nests %d deep, over %d", ErrLimitExceeded, deepest, o.maxDepth)
	}
	return nil
}

// blockDepth is how deeply braces and parentheses nest in css, together.
// Selectors nest in parentheses, as in :is(:not(a)). Those in strings and
// comments count too, which can only overstate it.
func blockDepth(css string) int {
	depth, deepest := 0, 0
	for i := 0; i < len(css); i++ {
		switch css[i] {
		case '{', '(':
			depth++
			deepest = max(deepest, depth)
		case '}', ')':
			depth = max(depth-1, 0)
		}
	}
	return deepest
}
