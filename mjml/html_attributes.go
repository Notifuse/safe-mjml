package mjml

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/juicer"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// htmlAttributeRule is an mj-selector of mj-html-attributes, with the node
// that first gave its path, for a warning.
type htmlAttributeRule struct {
	juicer.AttributeRule
	node *parser.MJMLNode
}

// htmlAttributeRules returns the rules of the head's mj-html-attributes, as
// mjml-head-html-attributes registers them in mjml-core's htmlAttributes:
// one per path, in the order paths first appear (a later mj-selector with the
// same path adds to it, its attributes replacing those of the same name in
// place), each with the attributes of its mj-html-attribute children that
// have a name, in order, the value being the element's text as MJML's
// parser reads it (textContent). A selector without path selects
// "undefined", as in MJML. A path or a name "__proto__" is left out: in the
// plain objects of mjml-core, setting it sets no key.
func (c *rootComponent) htmlAttributeRules() []htmlAttributeRule {
	if c.Head == nil {
		return nil
	}
	var rules []htmlAttributeRule
	byPath := map[string]int{}
	byName := []map[string]int{} // where each rule has each attribute
	for _, child := range c.Head.GetChildren() {
		block, ok := child.(*components.MJHtmlAttributesComponent)
		if !ok {
			continue
		}
		for _, sel := range block.Node.Children {
			if sel.GetTagName() != "mj-selector" {
				continue
			}
			path, ok := sel.LookupAttribute("path")
			if !ok {
				path = "undefined"
			}
			if path == "__proto__" {
				continue
			}
			i, seen := byPath[path]
			if !seen {
				i = len(rules)
				byPath[path] = i
				rules = append(rules, htmlAttributeRule{AttributeRule: juicer.AttributeRule{Selector: path}, node: sel})
				byName = append(byName, map[string]int{})
			}
			for _, attr := range sel.Children {
				name := attr.GetAttribute("name")
				if attr.GetTagName() != "mj-html-attribute" || name == "" || name == "__proto__" {
					continue
				}
				rules[i].Attributes = setRuleAttribute(rules[i].Attributes, byName[i], html.Attribute{Key: name, Val: textContent(attr.Text)})
			}
		}
	}
	return rules
}

// textContent is the content of an mj-html-attribute, written raw, as
// MJML's parser (mjml-parser-xml, on htmlparser2) reads it: not as an
// ending tag, so a tag in it is an element of its own, whose text is not the
// attribute's, and a comment, a processing instruction or a declaration is
// left out. htmlparser2 gives the text in chunks, a new one at each "<", and
// the content is each chunk trimmed, joined and trimmed again, with
// JavaScript's trim. A CDATA section is a chunk of its own, its content.
//
// MJML trims the content again after each chunk, which removes nothing: the
// content and the chunk are trimmed already, and a chunk is not empty. The
// chunks are appended in linear time.
func textContent(raw string) string {
	var content strings.Builder
	add := func(chunk string) {
		content.WriteString(jsTrim(chunk))
	}
	var open []string          // the elements open in the content
	opened := map[string]int{} // how many of open have each name
	start := 0                 // where the current chunk of text starts
	for i := 0; i < len(raw); {
		if raw[i] != '<' {
			i++
			continue
		}
		end, kind, name := markupAt(raw, i)
		if kind == markupNone {
			// Text, in a chunk that starts here.
			if len(open) == 0 {
				add(raw[start:i])
			}
			start = i
			i++
			continue
		}
		if len(open) == 0 {
			add(raw[start:i])
			if kind == markupCDATA {
				add(raw[i+len("<![CDATA[") : max(i+len("<![CDATA["), end-len("]]>"))])
			}
		}
		switch kind {
		case markupOpen:
			open = append(open, name)
			opened[name]++
		case markupClose:
			// A close tag of no open element is skipped without a walk, so
			// that stray close tags under deep nesting stay linear.
			if opened[name] == 0 {
				break
			}
			for k := len(open) - 1; k >= 0; k-- {
				opened[open[k]]--
				if open[k] == name {
					open = open[:k]
					break
				}
			}
		}
		i, start = end, end
	}
	if len(open) == 0 {
		add(raw[start:])
	}
	return content.String()
}

const (
	markupNone = iota
	markupOpen
	markupClose
	markupSelfClosing // or a void element
	markupComment     // a comment, a processing instruction, a declaration
	markupCDATA
)

// markupAt reads the markup at raw[i], a "<", as htmlparser2 reads it: what
// kind it is, where it ends and, for a tag, its name, lower-cased.
func markupAt(raw string, i int) (end, kind int, name string) {
	rest := raw[i+1:]
	switch {
	case strings.HasPrefix(rest, "!--"):
		if k := strings.Index(raw[i+4:], "-->"); k >= 0 {
			return i + 4 + k + 3, markupComment, ""
		}
		return len(raw), markupComment, ""
	case strings.HasPrefix(rest, "![CDATA["):
		if k := strings.Index(raw[i+9:], "]]>"); k >= 0 {
			return i + 9 + k + 3, markupCDATA, ""
		}
		return len(raw), markupComment, ""
	case strings.HasPrefix(rest, "!") || strings.HasPrefix(rest, "?"):
		if k := strings.IndexByte(rest, '>'); k >= 0 {
			return i + 1 + k + 1, markupComment, ""
		}
		return len(raw), markupComment, ""
	}
	closing := strings.HasPrefix(rest, "/")
	if closing {
		rest = rest[1:]
	}
	if rest == "" || !isTagStart(rest[0]) {
		return 0, markupNone, ""
	}
	n := 0
	for n < len(rest) && !strings.ContainsRune(" \t\n\r\f/>", rune(rest[n])) {
		n++
	}
	name = strings.ToLower(rest[:n])
	// The tag ends at the first ">" outside a quoted attribute value.
	var quote byte
	k := n
	for ; k < len(rest); k++ {
		c := rest[k]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			// A value's quote, after its "=".
			if p := strings.TrimRight(rest[:k], " \t\n\r\f"); strings.HasSuffix(p, "=") {
				quote = c
			}
		case c == '>':
			end = len(raw) - len(rest) + k + 1
			switch {
			case closing:
				return end, markupClose, name
			case rest[k-1] == '/' || voidTags[name]:
				return end, markupSelfClosing, name
			}
			return end, markupOpen, name
		}
	}
	// A tag left open takes the rest.
	return len(raw), markupComment, ""
}

func isTagStart(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// voidTags are the elements htmlparser2 closes as it opens them.
var voidTags = map[string]bool{
	"area": true, "base": true, "basefont": true, "br": true, "col": true, "command": true, "embed": true,
	"frame": true, "hr": true, "image": true, "img": true, "input": true, "isindex": true, "keygen": true,
	"link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true,
}

// jsTrim is JavaScript's String.prototype.trim: it removes ECMAScript's
// white space and line terminators, U+FEFF included and U+0085 not.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
			return true
		}
		return 0x2000 <= r && r <= 0x200A
	})
}

// setRuleAttribute sets a in attrs as an object spread does: in place when
// attrs has its name, last otherwise. at is where attrs has each name.
func setRuleAttribute(attrs []html.Attribute, at map[string]int, a html.Attribute) []html.Attribute {
	if i, ok := at[a.Key]; ok {
		attrs[i].Val = a.Val
		return attrs
	}
	at[a.Key] = len(attrs)
	return append(attrs, a)
}

// applyHTMLAttributes sets the attributes of rules on the elements of body,
// the rendered mj-body, as mjml-core does before it writes the skeleton:
// with cheerio in XML mode, which writes the body back, an element without
// children closing itself (<div/>), or, minified, written as html-minifier
// writes that (<div>). A selector safe-mjml cannot read sets
// nothing, with a warning; MJML fails on most of them.
func applyHTMLAttributes(body string, rules []htmlAttributeRule, minified bool, opts *options.RenderOpts) (string, error) {
	attributeRules := make([]juicer.AttributeRule, len(rules))
	for i, r := range rules {
		attributeRules[i] = r.AttributeRule
	}
	juicerOptions := []juicer.Option{
		// Template tags kept intact, as inline mj-style keeps them.
		juicer.CodeBlocks(juicer.DefaultCodeBlocks()),
		juicer.MaxInputSize(0),
		juicer.MaxDepth(attributesMaxDepth),
		juicer.MaxMatchSteps(limitOf(opts.Limits.WorkSteps, juiceMaxMatchSteps)),
		juicer.MaxSelectorBytes(limitOf(opts.Limits.SelectorBytes, maxSelectorBytes)),
		juicer.MaxOutputSize(outputLimit(opts)),
	}
	if minified {
		// safe-mjml's output is MJML's as html-minifier leaves it.
		juicerOptions = append(juicerOptions, juicer.MinifiedVoids())
	}
	out, invalid, err := juicer.SetAttributes(body, attributeRules, juicerOptions...)
	if errors.Is(err, juicer.ErrOutputTooLarge) {
		return "", outputTooLarge(outputLimit(opts))
	}
	if err != nil {
		return "", fmt.Errorf("mj-html-attributes: %w", err)
	}
	nodes := make(map[string]*parser.MJMLNode, len(rules))
	for _, r := range rules {
		nodes[r.Selector] = r.node
	}
	for _, sel := range invalid {
		opts.Warn(options.NewWarning(options.WarningInvalidAttribute, nodes[sel], fmt.Sprintf("mj-selector path %q is not a selector safe-mjml reads: it sets no attribute", sel)))
	}
	return out, nil
}

// attributePassMarker is the marker the components write, while the body is
// written for the attribute pass, where MJML's markup has whitespace: U+FFFF,
// a character no template can hold (the parser refuses it), and that no
// selector a template writes reads as text, unless escaped (\ffff).
const attributePassMarker = "\uffff"

// attributesMaxDepth bounds the nesting of the body as the attribute pass
// reads it, in XML, where every element left open nests what follows it in
// its parent: an mj-raw of a few thousand <br> nests as deep. It leaves room
// for any real body while bounding the recursion of :has() and :contains().
const attributesMaxDepth = 100_000
