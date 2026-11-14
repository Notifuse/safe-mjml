package mjml

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// displayedTextDifference compares the text of two documents as a client
// displays it, in document order, and returns the first difference, or "".
//
// The per-element comparison of compareDOMTrees reads the text of each
// element on its own: it cannot see a space lost between an element and the
// word after it, or a word moved across an inline child. This check reads
// the whole document as html-minifier's collapseWhitespace leaves it, which
// is what the MJML references went through:
//
//   - outside <pre>, <textarea>, <script>, <style> and an inline
//     white-space:pre*, each run of whitespace is one space, and runs merge
//     across inline elements (a, b, span, img…);
//   - no space is kept at the edge of a block element (div, p, td, br…), at
//     the start of a run that follows a space, or at the end of the body;
//   - inside the elements that keep their whitespace, the text is compared
//     as written.
//
// Each token records where it is: the text, and the inline and block
// elements around it, so a word on the other side of a tag is a
// difference. A comment is not a boundary: the text on either side of it
// is one token, as a client displays it.
func displayedTextDifference(expected, actual string) string {
	e, err := displayedTokens(expected)
	if err != nil {
		return ""
	}
	a, err := displayedTokens(actual)
	if err != nil {
		return ""
	}
	for i := 0; i < len(e) || i < len(a); i++ {
		if i < len(e) && i < len(a) && e[i] == a[i] {
			continue
		}
		return fmt.Sprintf("displayed text differs at token %d:\n  expected: %s\n  actual:   %s", i, tokenContext(e, i), tokenContext(a, i))
	}
	return ""
}

func tokenContext(tokens []string, i int) string {
	from, to := max(0, i-3), min(len(tokens), i+4)
	parts := make([]string, 0, to-from)
	for k := from; k < to; k++ {
		if k == i {
			parts = append(parts, "»"+tokens[k]+"«")
		} else {
			parts = append(parts, tokens[k])
		}
	}
	if i >= len(tokens) {
		parts = append(parts, "»(end)«")
	}
	return strings.Join(parts, " ")
}

// inlineElements are html-minifier's inline tags: whitespace around them is
// collapsed, not removed.
var inlineElements = map[string]bool{
	"a": true, "abbr": true, "acronym": true, "b": true, "bdi": true, "bdo": true, "big": true, "button": true,
	"cite": true, "code": true, "del": true, "dfn": true, "em": true, "font": true, "i": true, "img": true,
	"input": true, "ins": true, "kbd": true, "label": true, "mark": true, "math": true, "nobr": true,
	"object": true, "q": true, "rp": true, "rt": true, "rtc": true, "ruby": true, "s": true, "samp": true,
	"select": true, "small": true, "span": true, "strike": true, "strong": true, "sub": true, "sup": true,
	"svg": true, "textarea": true, "time": true, "tt": true, "u": true, "var": true,
}

type textToken struct {
	text     string // text, with whitespace collapsed unless raw
	raw      bool   // text whose whitespace is displayed as written
	marker   string // "<b>", "</b>" for inline elements, "|div" for a block boundary
	isMarker bool
}

func displayedTokens(document string) ([]string, error) {
	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return nil, err
	}
	var tokens []textToken
	var walk func(n *html.Node, raw bool)
	walk = func(n *html.Node, raw bool) {
		switch n.Type {
		case html.TextNode:
			text := n.Data
			if !raw {
				text = strings.Join(strings.FieldsFunc(text, isCollapsible), " ")
				if strings.IndexFunc(n.Data, isCollapsible) == 0 && text != "" {
					text = " " + text
				} else if text == "" && n.Data != "" {
					text = " "
				}
				if last := strings.LastIndexFunc(n.Data, isCollapsible); last == len(n.Data)-1 && n.Data != "" && text != " " {
					text += " "
				}
			}
			// A comment is not displayed: the text after it continues the
			// text before it, as a client shows them. Joined, the
			// whitespace html-minifier moves across a comment ("A <!--x-->B"
			// becomes "A<!--x--> B") reads the same.
			if last := len(tokens) - 1; last >= 0 && !tokens[last].isMarker && tokens[last].raw == raw {
				if !raw && strings.HasSuffix(tokens[last].text, " ") {
					text = strings.TrimPrefix(text, " ")
				}
				tokens[last].text += text
				return
			}
			tokens = append(tokens, textToken{text: text, raw: raw})
			return
		case html.ElementNode:
			name := n.Data
			if name == "script" || name == "style" || name == "head" {
				if name == "head" {
					// The title is displayed by the client.
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						if c.Type == html.ElementNode && c.Data == "title" {
							tokens = append(tokens, textToken{marker: "|title", isMarker: true})
							walk(c, false)
							tokens = append(tokens, textToken{marker: "|/title", isMarker: true})
						}
					}
				}
				return
			}
			childRaw := raw || name == "pre" || name == "textarea" || whiteSpacePre(n)
			if inlineElements[name] {
				tokens = append(tokens, textToken{marker: "<" + name + ">", isMarker: true})
			} else {
				tokens = append(tokens, textToken{marker: "|" + name, isMarker: true})
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, childRaw)
			}
			if inlineElements[name] {
				tokens = append(tokens, textToken{marker: "</" + name + ">", isMarker: true})
			} else {
				tokens = append(tokens, textToken{marker: "|/" + name, isMarker: true})
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, raw)
		}
	}
	walk(root, false)

	// Trim as collapseWhitespace does: a space after a block boundary, the
	// start of the document or another space goes, and so does a space
	// before a block boundary or the end.
	prevBlockOrSpace := true
	for i := range tokens {
		t := &tokens[i]
		if t.isMarker {
			if strings.HasPrefix(t.marker, "|") {
				prevBlockOrSpace = true
			} else if displaysContent(t.marker, "</") {
				// After an image, the space before the next word shows.
				prevBlockOrSpace = false
			}
			continue
		}
		if t.raw {
			prevBlockOrSpace = false
			continue
		}
		if prevBlockOrSpace {
			t.text = strings.TrimPrefix(t.text, " ")
		}
		if t.text != "" {
			prevBlockOrSpace = strings.HasSuffix(t.text, " ")
		}
	}
	nextBlock := true
	for i := len(tokens) - 1; i >= 0; i-- {
		t := &tokens[i]
		if t.isMarker {
			if strings.HasPrefix(t.marker, "|") {
				nextBlock = true
			} else if displaysContent(t.marker, "<") {
				// Before an image, the space after the previous word shows.
				nextBlock = false
			}
			continue
		}
		if t.raw {
			nextBlock = false
			continue
		}
		if nextBlock {
			t.text = strings.TrimSuffix(t.text, " ")
		}
		if t.text != "" {
			nextBlock = false
		}
	}
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		switch {
		case t.isMarker:
			out = append(out, t.marker)
		case t.text != "":
			out = append(out, fmt.Sprintf("%q", t.text))
		}
	}
	return out, nil
}

// contentElements are the inline elements that display something without
// text (an image, a form control, an SVG): a space next to one shows, as
// between two words; next to an empty <span> it does not.
var contentElements = map[string]bool{
	"img": true, "input": true, "button": true, "select": true, "textarea": true,
	"svg": true, "object": true, "math": true,
}

// displaysContent reports whether marker is the marker prefix ("<" for an
// open, "</" for a close) of one of contentElements.
func displaysContent(marker, prefix string) bool {
	if !strings.HasPrefix(marker, prefix) || !strings.HasSuffix(marker, ">") {
		return false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(marker, prefix), ">")
	return !strings.HasPrefix(name, "/") && contentElements[name]
}

func isCollapsible(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}

// whiteSpacePre reports whether n's inline style displays its whitespace as
// written.
func whiteSpacePre(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "style" {
			v := strings.ToLower(attr.Val)
			for _, decl := range strings.Split(v, ";") {
				name, value, ok := strings.Cut(decl, ":")
				if ok && strings.TrimSpace(name) == "white-space" {
					value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!important"))
					if strings.HasPrefix(value, "pre") || value == "break-spaces" {
						return true
					}
				}
			}
		}
	}
	return false
}

// preambleDifference compares what comes before <html>: the doctype, which
// the HTML parser and the DOM comparison drop, and anything MJML writes
// before it (mj-raw position="file-start").
func preambleDifference(expected, actual string) string {
	e, a := preamble(expected), preamble(actual)
	if e == a {
		return ""
	}
	return fmt.Sprintf("  expected: %q\n  actual:   %q", e, a)
}

func preamble(document string) string {
	end := strings.Index(strings.ToLower(document), "<html")
	if end < 0 {
		end = len(document)
	}
	// Whitespace collapsed, and dropped next to a tag: MJML writes a line
	// break between the file-start content and the doctype, which
	// html-minifier removes from the references.
	collapsed := strings.Join(strings.Fields(document[:end]), " ")
	return strings.NewReplacer(" <", "<", "> ", ">").Replace(collapsed)
}
