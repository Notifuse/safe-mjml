package mjml

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/juicer"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// isInlineStyle reports whether an mj-style is inlined: its inline attribute
// is exactly "inline", as mjml-head-style compares it (inline="INLINE" or
// " inline" is a plain <style>). As written: neither mj-attributes nor
// mj-all reach mj-style.
func isInlineStyle(style *components.MJStyleComponent) bool {
	return style.Node.GetAttribute("inline") == "inline"
}

// juiceDocument inlines the inline mj-style rules into the document as
// mjml-core does, after the skeleton:
//
//	juice(content, {applyStyleTags: false, extraCss: inlineStyle.join(''),
//	  insertPreservedExtraCss: false, removeStyleTags: false})
//
// MJML 4.18 runs juice 10.0.1 on cheerio 1.0.0-rc.12, which juicer's
// Juice10 profile follows: htmlparser2's tree, css-select's matching,
// juice's specificity, CSS variables resolved, and cheerio's serialization,
// over the whole document and for every selector. The <style> elements are
// neither read nor changed, and what cannot be inlined (media queries,
// :hover) is dropped. Where juice throws, the render fails, as MJML's does.
func juiceDocument(document string, styles []string) (string, error) {
	out, err := juicer.Inline(document,
		juicer.CompatibilityMode(juicer.Juice10),
		juicer.ExtraCSS(strings.Join(styles, "")),
		juicer.ApplyStyleTags(false),
		juicer.RemoveStyleTags(false),
		juicer.InsertPreservedExtraCSS(false),
		// juicer's code blocks, {{ }}, <% %> and Liquid's {% %}: juice
		// protects the first two only, and a Liquid tag inside an HTML tag
		// (<p {% if x %}class="a"{% endif %}>) loses its end there. safe-mjml
		// keeps it, on purpose (Pierre's decision).
		juicer.CodeBlocks(juicer.DefaultCodeBlocks()),
		// safe-mjml wrote the document from a template it bounded: no limit on
		// its size (a carousel of many images is large), and room for its
		// deepest nesting, the template's plus the skeleton's.
		juicer.MaxInputSize(0),
		juicer.MaxDepth(juiceMaxDepth),
		juicer.MaxStylesheetDepth(juiceMaxStylesheetDepth),
		juicer.MaxMatchSteps(juiceMaxMatchSteps),
	)
	switch {
	case errors.Is(err, juicer.ErrJuiceFails) && !holdsTemplateTags(document):
		return "", fmt.Errorf("inline mj-style: MJML fails on this template too: %w", err)
	case err != nil:
		return "", fmt.Errorf("inline mj-style: %w", err)
	}
	return out, nil
}

// juiceMaxDepth bounds the nesting of the document juicer accepts: twice
// the template's, which leaves the skeleton room around the deepest
// template.
// holdsTemplateTags reports whether the document holds a template tag
// juicer protects. safe-mjml keeps them where juice does not (a Liquid tag, or
// a tag in a style attribute), so a failure there may be one MJML does not
// have: juice drops a style attribute with a brace in it and goes on.
func holdsTemplateTags(document string) bool {
	return strings.Contains(document, "{{") || strings.Contains(document, "{%") || strings.Contains(document, "<%")
}

const juiceMaxDepth = 2 * parser.MaxNesting

// juiceMaxStylesheetDepth bounds the nesting of the blocks and parentheses
// of the inline mj-styles (and of the style attributes they reach): parsing
// nested at-rules costs more than linear time in their depth (1,900 nested
// @media took 5 s), and a real stylesheet nests a few levels.
const juiceMaxStylesheetDepth = 100

// juiceMaxMatchSteps bounds the work of matching the inline mj-style
// selectors to the document, and of the declarations they apply: each rule
// tested on an element, each element, attribute list or node its selector
// looks at, every 16 bytes it reads and each byte whose case it lowers, 250
// steps and 16 a byte of value for each declaration an element gets, and
// the ancestors and copies of CSS variables (see juicer.MaxMatchSteps and
// cascadia.Budget). Matching costs rules times
// elements times the walk of each selector, so a template made for it can
// take seconds in 70 KB (2,000 rules ".absent1 div" to ".absent2000 div" on
// 900 nested divs: 1.7 billion steps, 4.9 s), and declarations times
// elements hold memory (2,000 declarations on 1,600 elements: 3.2 million).
// The most costly document of the corpora, MJML's official worldly
// template, takes 62,468 steps; 250 million take 1 to 2 s on an idle
// Apple M5, several times that on a loaded one, and hold at most a million
// declarations and 15 MB of their values. A template over the limit is
// refused (docs/differences-from-mjml.md, "Stricter than MJML").
const juiceMaxMatchSteps = 250_000_000
