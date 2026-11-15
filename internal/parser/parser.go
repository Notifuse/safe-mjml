// Package parser provides MJML XML parsing functionality.
// It converts MJML markup into an Abstract Syntax Tree (AST) representation
// that can be used by the mjml package for component creation and rendering.
package parser

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/debug"
)

// htmlVoidElements contains HTML elements that do not require closing tags.
//
// Reference: https://html.spec.whatwg.org/\#void-elements
var htmlVoidElements = map[string]struct{}{
	"area":   {},
	"base":   {},
	"br":     {},
	"col":    {},
	"embed":  {},
	"hr":     {},
	"img":    {},
	"input":  {},
	"link":   {},
	"meta":   {},
	"param":  {},
	"source": {},
	"track":  {},
	"wbr":    {},
}

// isVoidHTMLElement reports whether the provided tag name is an HTML void element.
func isVoidHTMLElement(tag string) bool {
	_, ok := htmlVoidElements[strings.ToLower(tag)]
	return ok
}

// MJMLNode represents a node in the MJML AST
type MJMLNode struct {
	XMLName    xml.Name
	Text       string
	Attrs      []xml.Attr
	Children   []*MJMLNode
	LineNumber int
	// StartOffset is the byte offset, in the processed source, of the '<' that
	// opens the element. It orders elements as they appear in the document,
	// including several on one line, and gives LineNumber.
	StartOffset int64
	// MixedContent preserves the interleaving order of text nodes and child elements
	// as they originally appeared in the MJML source. Each entry contains either
	// a text segment or a pointer to a child node.
	MixedContent []MixedContentPart
}

// MixedContentPart represents either a piece of text or a child node in the
// mixed content sequence of an MJML node.
type MixedContentPart struct {
	Text string
	Node *MJMLNode
}

type lineLookup struct {
	lineOffsets []int
	lastOffset  int64
	lastIndex   int
}

func newLineLookup(content string) *lineLookup {
	offsets := make([]int, 1, 1+strings.Count(content, "\n"))
	for i := strings.IndexByte(content, '\n'); i >= 0; {
		offsets = append(offsets, i+1)
		next := strings.IndexByte(content[i+1:], '\n')
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return &lineLookup{lineOffsets: offsets, lastOffset: -1}
}

func (ll *lineLookup) Line(offset int64) int {
	if ll == nil || len(ll.lineOffsets) == 0 {
		return 1
	}
	if offset >= ll.lastOffset {
		idx := ll.lastIndex
		for idx+1 < len(ll.lineOffsets) && ll.lineOffsets[idx+1] <= int(offset) {
			idx++
		}
		ll.lastIndex = idx
		ll.lastOffset = offset
		return idx + 1
	}

	idx := max(sort.Search(len(ll.lineOffsets), func(i int) bool {
		return ll.lineOffsets[i] > int(offset)
	})-1, 0)
	ll.lastIndex = idx
	ll.lastOffset = offset
	return idx + 1
}

// AIDEV-NOTE: mjml-spec-structure; MJML document structure per official spec
// Minimal valid MJML structure:
// <mjml>
//   <mj-body> (required)
//     <!-- at least one component -->
//   </mj-body>
// </mjml>
// The <mj-head> section is OPTIONAL and can be omitted entirely.

// ParseMJML parses an MJML string into an AST
func ParseMJML(mjmlContent string) (*MJMLNode, error) {
	// A byte order mark is not part of the template, as for MJML.
	mjmlContent = strings.TrimPrefix(mjmlContent, "\uFEFF")

	// Comments before <mjml> would stop the XML decoder: drop them.
	processedContent := stripNonMSOComments(mjmlContent)

	// Keep the content of ending tags (mj-text, mj-button, mj-raw…) raw, as
	// MJML does: wrapped in CDATA, it is not parsed as XML and its entities
	// are not decoded.
	processedContent = wrapEndingTagContent(processedContent)

	// Pre-process, outside that content, the HTML entities the XML parser
	// does not know, and escape stray ampersands in attribute values.
	processedContent = outsideCDATA(processedContent, preprocessHTMLEntities)

	// Tag names in lower case, as htmlparser2 gives them to MJML, so that
	// <MJ-SECTION> closes with </mj-section> for the case-sensitive XML
	// decoder. After the ending tags are captured: their content is CDATA,
	// and a close in another case still loses it, as in MJML.
	processedContent = outsideCDATA(processedContent, lowerTagNames)

	// Comments with "--" in them, which htmlparser2 reads up to the first
	// "-->" and the XML decoder refuses, with their dashes encoded; the
	// comment tokens decode them.
	processedContent = outsideCDATA(processedContent, encodeCommentDashes)

	lookup := newLineLookup(processedContent)

	decoder := xml.NewDecoder(strings.NewReader(processedContent))
	// The template is already text: the encoding an XML declaration names
	// (encoding="ISO-8859-1") does not apply to it, as for MJML.
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	root, err := parseNode(decoder, xml.StartElement{}, lookup, 0, 1)
	if err == nil {
		err = checkAfterRoot(processedContent, int(decoder.InputOffset()), lookup)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse MJML: %w", err)
	}
	return root, nil
}

// MalformedError is a template the parser refuses because of its
// structure, as MJML throws "Malformed MJML" for it: something after the
// root element's close tag.
type MalformedError struct {
	Line    int    // the line of what was found
	TagName string // its tag, "" for text, a processing instruction, a declaration or a CDATA section
	Msg     string // what is wrong: "malformed MJML: <mj-section> after the root element"
}

func (e *MalformedError) Error() string { return e.Msg }

// checkAfterRoot returns a MalformedError for the first thing after the
// root element's close tag, which ends at from, that is neither whitespace
// nor a comment (Pierre's rule). mjml-parser-xml takes the last element of
// the document as its root: MJML throws "Malformed MJML" for an element
// after </mjml> (a section pasted below it, or the rest of a document whose
// </mjml> is in an mj-text), and renders the second of two documents.
// safe-mjml refuses both, and text, close tags, processing instructions,
// declarations and CDATA sections there too, which MJML ignores.
func checkAfterRoot(content string, from int, lookup *lineLookup) error {
	for i := from; i < len(content); {
		rest := content[i:]
		var found, tag string
		switch {
		case strings.IndexByte(" \t\n\r\f", content[i]) >= 0:
			i++
			continue
		case strings.HasPrefix(rest, "<!--"):
			if i = CommentEnd(content, i); i < 0 {
				return nil // a comment that never ends, to the end
			}
			continue
		case strings.HasPrefix(rest, cdataStart):
			found = "a CDATA section"
		case strings.HasPrefix(rest, "<!"):
			found = "a declaration"
		case strings.HasPrefix(rest, "<?"):
			found = "a processing instruction"
		case rest[0] != '<':
			found = "text"
		default:
			name, _ := tagNameAt([]byte(rest[:min(len(rest), 256)]), 0)
			tag = strings.ToLower(name)
			switch {
			case tag == "":
				found = "text" // "< 5": a "<" before a letter only opens a tag
			case rest[1] == '/':
				found = "</" + tag + ">"
			default:
				found = "<" + tag + ">"
			}
		}
		line := 0
		if lookup != nil {
			line = lookup.Line(int64(i))
		}
		return &MalformedError{Line: line, TagName: tag, Msg: "malformed MJML: " + found + " after the root element"}
	}
	return nil
}

// preprocessHTMLEntities replaces common HTML entities with Unicode characters
// and properly escapes ampersands in attribute values. Raw ampersands are first
// escaped to &amp; for XML safety, then most entities are replaced with Unicode.
// The &amp; entities are left for the XML parser to handle, preventing re-introduction
// of invalid raw ampersands that would break XML parsing.
//
// The XML predefined entities (&lt; &gt; &quot; &apos;) are a special case: they
// are decoded only *outside* quoted attribute values (see
// decodeXMLEntitiesOutsideAttributes). Inside an attribute value they are left
// for encoding/xml, which decodes them natively; decoding them document-wide
// here used to corrupt correctly-escaped attribute values before the parser saw
// them (e.g. alt="27&quot; monitor" became the unparseable alt="27" monitor").
func preprocessHTMLEntities(content string) string {
	// First, escape raw ampersands in attribute values that aren't part of valid entities
	result := escapeAttributeAmpersands(content)

	// Replace the HTML-only entities that encoding/xml does not understand. These
	// must be decoded document-wide, otherwise the XML parser rejects them.
	// NOTE: &amp; entities are intentionally preserved - the XML parser will convert
	// them to raw ampersands safely after parsing, maintaining XML validity.
	result = strings.ReplaceAll(result, "&copy;", "©")
	result = strings.ReplaceAll(result, "&reg;", "®")
	result = strings.ReplaceAll(result, "&trade;", "™")
	result = strings.ReplaceAll(result, "&nbsp;", "\u00A0") // Unicode non-breaking space
	result = strings.ReplaceAll(result, "&#xA0;", "\u00A0") // Numeric character reference for non-breaking space
	result = strings.ReplaceAll(result, "&#160;", "\u00A0") // Decimal numeric reference for non-breaking space
	result = strings.ReplaceAll(result, "&ndash;", "–")
	result = strings.ReplaceAll(result, "&mdash;", "—")
	result = strings.ReplaceAll(result, "&hellip;", "…")

	// Decode the quote entities only outside quoted attribute values so that
	// correctly-escaped attribute values survive intact for encoding/xml.
	result = decodeXMLEntitiesOutsideAttributes(result)

	return result
}

// decodeXMLEntitiesOutsideAttributes decodes the XML predefined entities
// &quot; and &apos; everywhere except inside quoted attribute values. Inside a
// quoted attribute value the entities are left intact so that encoding/xml
// decodes them natively during parsing - decoding them beforehand would turn a
// valid escaped value such as alt="27&quot; monitor" into the unparseable
// alt="27" monitor".
//
// &lt; and &gt; are never decoded here: before the XML parse, they would turn
// escaped text into markup, so that text placed between components (an
// application's escaped data, &lt;mj-button href=...&gt;) became a component.
// encoding/xml decodes them as text, as MJML's parser does.
//
// The scanner mirrors the tag/quote state machine used by
// escapeAttributeAmpersands so the two passes agree on what is an attribute value.
func decodeXMLEntitiesOutsideAttributes(content string) string {
	if !strings.ContainsRune(content, '&') {
		return content
	}

	var out strings.Builder
	out.Grow(len(content))

	inTag := false
	var quote byte
	afterEquals := false // a quote opens a value only after =, as in htmlparser2

	for i := 0; i < len(content); i++ {
		c := content[i]

		if quote != 0 {
			// Inside a quoted attribute value: copy verbatim and let the XML
			// parser decode the entities in attribute context.
			if c == quote {
				quote = 0
			}
			out.WriteByte(c)
			continue
		}

		quoted := afterEquals
		afterEquals = valueMayStart(c, afterEquals)
		switch c {
		case '<':
			inTag = true
			out.WriteByte(c)
		case '>':
			inTag = false
			out.WriteByte(c)
		case '\'', '"':
			if inTag && quoted {
				quote = c
			}
			out.WriteByte(c)
		case '&':
			if repl, size, ok := matchXMLPredefinedEntity(content[i:]); ok {
				out.WriteString(repl)
				i += size - 1
			} else {
				out.WriteByte(c)
			}
		default:
			out.WriteByte(c)
		}
	}

	return out.String()
}

// matchXMLPredefinedEntity reports whether s begins with one of the XML
// predefined entities &lt; &gt; &quot; &apos; and, if so, returns the decoded
// replacement together with the byte length of the matched entity. &amp; is
// deliberately not matched here; it is preserved for encoding/xml.
func matchXMLPredefinedEntity(s string) (replacement string, size int, ok bool) {
	switch {
	case strings.HasPrefix(s, "&quot;"):
		return `"`, len("&quot;"), true
	case strings.HasPrefix(s, "&apos;"):
		return "'", len("&apos;"), true
	default:
		return "", 0, false
	}
}

// escapeAttributeAmpersands escapes every & and < of the quoted attribute
// values, so that encoding/xml, which decodes attribute values, gives them
// back as written: a raw & of a URL, an entity (&quot;, &amp;copy;, &nbsp;)
// and a < all stay as the template has them, as in MJML.
func escapeAttributeAmpersands(content string) string {
	// Copied only from the first byte that changes: a template with no &
	// or < in its quoted values is returned as it is.
	var out strings.Builder
	copied := 0
	inTag := false
	var quote byte
	afterEquals := false // a quote opens a value only after =, as in htmlparser2
	for i := 0; i < len(content); i++ {
		c := content[i]
		if quote != 0 {
			var repl string
			switch c {
			case quote:
				quote = 0
				continue
			case '&':
				repl = "&amp;"
			case '<':
				repl = "&lt;"
			default:
				continue
			}
			if copied == 0 {
				out.Grow(len(content) + 16)
			}
			out.WriteString(content[copied:i])
			out.WriteString(repl)
			copied = i + 1
			continue
		}
		quoted := afterEquals
		afterEquals = valueMayStart(c, afterEquals)
		switch c {
		case '<':
			inTag = true
		case '>':
			inTag = false
		case '\'', '"':
			if inTag && quoted {
				quote = c
			}
		}
	}
	if copied == 0 {
		return content
	}
	out.WriteString(content[copied:])
	return out.String()
}

// stripNonMSOComments removes comments that appear before the <mjml> root node.
// Comments inside the document are preserved so they can be rendered in the
// output HTML, as MJML keeps them, while the XML decoder starts at the root
// element.
func stripNonMSOComments(content string) string {
	idx := findMjmlTagIndex(content)
	if idx <= 0 {
		return content
	}
	prefix := content[:idx]
	// Drop the comments and the leading whitespace before <mjml>, but keep
	// their line breaks, so that the lines validation warnings report are
	// those of the template.
	var result strings.Builder
	result.Grow(len(content))
	for {
		start := strings.Index(prefix, "<!--")
		if start < 0 {
			break
		}
		end := CommentEnd(prefix, start)
		if end < 0 {
			// Malformed comment: drop everything from it, keeping its lines.
			result.WriteString(prefix[:start])
			result.WriteString(strings.Repeat("\n", strings.Count(prefix[start:], "\n")))
			prefix = ""
			break
		}
		result.WriteString(prefix[:start])
		result.WriteString(strings.Repeat("\n", strings.Count(prefix[start:end], "\n")))
		prefix = prefix[end:]
	}
	result.WriteString(prefix)
	kept := result.String()
	trimmed := trimLeftInPlace(kept)
	return strings.Repeat("\n", strings.Count(kept[:len(kept)-len(trimmed)], "\n")) + trimmed + content[idx:]
}

// trimLeftInPlace efficiently trims leading whitespace without allocating new strings
func trimLeftInPlace(s string) string {
	start := 0
	for start < len(s) {
		c := s[start]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			break
		}
		start++
	}
	return s[start:]
}

// findMjmlTagIndex finds the index of "<mjml", in any case, outside the
// comments before it (<!-- an old <mjml> template --> is not the root), or
// -1. A comment that never ends hides the rest of the document.
func findMjmlTagIndex(content string) int {
	const needle = "<mjml"
	for i := 0; i <= len(content)-len(needle); i++ {
		if strings.HasPrefix(content[i:], "<!--") {
			end := CommentEnd(content, i)
			if end < 0 {
				return -1
			}
			i = end - 1 // the loop adds the last 1
			continue
		}
		if strings.EqualFold(content[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

// endingTags are the MJML tags whose content is raw HTML or text, not MJML:
// mjml-core's endingTag components. Their content is wrapped in CDATA before
// parsing, so that it reaches the component as written, entities and inline
// HTML included, as MJML passes it.
var endingTags = map[string]bool{
	"mj-text": true, "mj-button": true, "mj-table": true, "mj-raw": true,
	"mj-navbar-link": true, "mj-social-element": true,
	"mj-accordion-title": true, "mj-accordion-text": true,
	"mj-title": true, "mj-preview": true, "mj-style": true,
	"mj-breakpoint": true, "mj-carousel-image": true,
}

const (
	cdataStart = "<![CDATA["
	cdataEnd   = "]]>"
	// cdataEndSafe is used to escape CDATA end sequences within CDATA sections.
	// When "]]>" appears in content that will be wrapped in CDATA, it's replaced
	// with "]]]]><![CDATA[>" which effectively closes the current CDATA section,
	// outputs "]]>", then starts a new CDATA section. This prevents XML parsing
	// errors that would occur if "]]>" appeared within a CDATA block.
	// See: https://www.w3.org/TR/xml/#sec-cdata-sect (W3C XML 1.0, section 2.7 CDATA Sections)
	cdataEndSafe = "]]]]><![CDATA[>"
)

// wrapEndingTagContent wraps the inner content of every ending tag
// (endingTags) in a CDATA section, as mjml-parser-xml keeps that content raw. It reads the document tag by tag:
// comments and CDATA sections are copied as they are (a tag named in a
// comment is not a tag), attribute values are skipped with their quotes, and
// the content of an ending tag runs to its matching close tag, written in any
// case and with spaces before its '>' (</mj-text\n>, as Prettier writes it),
// counting the same tags opened inside it.
func wrapEndingTagContent(content string) string {
	if content == "" {
		return ""
	}

	b := []byte(content)
	var out strings.Builder
	out.Grow(len(content) + 64)

	copied := 0       // b[:copied] is in out
	var open tagStack // the elements open at pos
	for pos := 0; pos < len(b); {
		lt := bytes.IndexByte(b[pos:], '<')
		if lt < 0 {
			break
		}
		idx := pos + lt
		if skip := skipCommentOrCDATA(b, idx); skip > idx {
			pos = skip
			continue
		}
		name, _ := tagNameAt(b, idx)
		if name == "" {
			pos = idx + 1
			continue
		}
		if b[idx+1] == '/' {
			// A close tag, as htmlparser2 reads it: its name starts after
			// the spaces that follow "</" and runs to a space or '>', and
			// it ends at the first '>'. The spaces before the name and
			// what follows it are ignored; the XML decoder refuses them,
			// so the close tag of an open element is written without
			// them, its line breaks kept for the lines of what follows.
			closeName, nameEnd := closeTagName(b, idx)
			gt := bytes.IndexByte(b[nameEnd:], '>')
			if gt < 0 {
				break
			}
			closeEnd := nameEnd + gt + 1
			pos = closeEnd
			if lower := strings.ToLower(closeName); open.has(lower) {
				open.close(lower)
				spaced := nameEnd-len(closeName) > idx+2
				if junk := b[nameEnd : closeEnd-1]; spaced || len(bytes.TrimSpace(junk)) > 0 {
					out.Write(b[copied:idx])
					out.WriteString("</" + closeName + strings.Repeat("\n", bytes.Count(b[idx:closeEnd], []byte("\n"))) + ">")
					copied = closeEnd
				}
			}
			continue
		}
		tagEnd, selfClosing := findTagEnd(b, idx)
		if tagEnd < 0 {
			break
		}
		pos = tagEnd
		lower := strings.ToLower(name)
		switch {
		case selfClosing:
			continue
		case !endingTags[lower]:
			open.push(lower)
			continue
		}
		closeStart, closeEnd, ancestor := findEndingTagClose(b, tagEnd, lower, &open)
		if closeStart < 0 {
			// Nothing closes it, or its close is inside a comment that
			// never ends: as in mjml-parser-xml, the content runs to the
			// end of the document, where htmlparser2 closes every open
			// element. It is kept up to the last "</name"; the rest, the
			// elements after it included, is lost.
			inner := b[tagEnd:]
			if at := bytes.LastIndex(inner, []byte("</"+lower)); at >= 0 {
				inner = inner[:at]
			} else {
				inner = nil
			}
			out.Write(b[copied:tagEnd])
			writeCDATA(&out, inner)
			out.WriteString("</" + name + ">")
			for i := len(open.names) - 1; i >= 0; i-- {
				out.WriteString("</" + open.names[i] + ">")
			}
			return out.String()
		}
		if ancestor >= 0 {
			// Closed by the close tag of an element around it
			// (<mj-text>Hello</mj-column>): htmlparser2 closes it there,
			// and mjml-parser-xml keeps its content, which runs to the end
			// of that close tag's name (htmlparser2's endIndex), up to the
			// last "</name" (usually nothing): never past the close tag's
			// start, as an accepted close tag's name holds no "/". The
			// ending tag and the elements open inside the ancestor close
			// before the ancestor's close tag, which the scan reads next;
			// the line breaks of the lost content go into the ending
			// tag's close, as below.
			_, nameEnd := closeTagName(b, closeStart)
			inner := b[tagEnd:nameEnd]
			if at := bytes.LastIndex(inner, []byte("</"+lower)); at >= 0 {
				inner = inner[:at]
			} else {
				inner = nil
			}
			lostLines := bytes.Count(b[tagEnd:closeStart], []byte("\n")) - bytes.Count(inner, []byte("\n"))
			out.Write(b[copied:tagEnd])
			writeCDATA(&out, inner)
			out.WriteString("</" + name + strings.Repeat("\n", lostLines) + ">")
			for len(open.names) > ancestor+1 {
				top := open.names[len(open.names)-1]
				out.WriteString("</" + top + ">")
				open.close(top)
			}
			copied = closeStart
			pos = closeStart
			continue
		}

		// The content as mjml-parser-xml keeps it: up to the end of the
		// close tag's name (htmlparser2's endIndex: what follows the name,
		// up to the '>', is not content), then up to the last "</name",
		// with the name in lower case, as written there. A close tag
		// written in another case (</MJ-TEXT>) is not found, and MJML
		// loses the content (Pierre's choice: as MJML).
		_, nameEnd := closeTagName(b, closeStart)
		inner := b[tagEnd:nameEnd]
		if at := bytes.LastIndex(inner, []byte("</"+lower)); at >= 0 {
			inner = inner[:at]
		} else {
			inner = nil
		}
		// The line breaks of what is not kept (the lost content, and the
		// close tag itself) go into the close tag written below, so that
		// the lines of the elements after it stay those of the template.
		lostLines := bytes.Count(b[tagEnd:closeEnd], []byte("\n")) - bytes.Count(inner, []byte("\n"))
		out.Write(b[copied:tagEnd])
		writeCDATA(&out, inner)
		// The close tag with the open tag's name, as the XML decoder is
		// case-sensitive (</MJ-TEXT> closes <mj-text> for MJML), without
		// what htmlparser2 ignores after the name.
		out.WriteString("</" + name + strings.Repeat("\n", lostLines) + ">")
		copied = closeEnd
		pos = closeEnd
	}
	out.Write(b[copied:])
	return out.String()
}

// writeCDATA writes content as a CDATA section, its own "]]>" made safe.
func writeCDATA(out *strings.Builder, content []byte) {
	if bytes.Contains(content, []byte(cdataEnd)) {
		content = bytes.ReplaceAll(content, []byte(cdataEnd), []byte(cdataEndSafe))
	}
	out.WriteString(cdataStart)
	out.Write(content)
	out.WriteString(cdataEnd)
}

// tagStack is the stack of open elements, as htmlparser2 keeps it, with
// their names in lower case. count gives the number of open elements of each
// name, so that a close tag no element of its name is open for (a stray
// close) is ignored without scanning the stack: the scan stays linear.
type tagStack struct {
	names []string
	count map[string]int
}

func (s *tagStack) push(lower string) {
	if s.count == nil {
		s.count = make(map[string]int)
	}
	s.names = append(s.names, lower)
	s.count[lower]++
}

// has reports whether an element named lower is open.
func (s *tagStack) has(lower string) bool { return s.count[lower] > 0 }

// close closes the open element named lower and those open inside it, as
// htmlparser2 closes a close tag's element; a close tag with no open element
// of its name changes nothing. Each element is popped once, so a document's
// closes cost no more than its opens.
func (s *tagStack) close(lower string) {
	if !s.has(lower) {
		return
	}
	for {
		top := s.names[len(s.names)-1]
		s.names = s.names[:len(s.names)-1]
		s.count[top]--
		if top == lower {
			return
		}
	}
}

// skipCommentOrCDATA returns the index after the comment (CommentEnd) or
// CDATA section that starts at i, or i when there is none. An unterminated
// one runs to the end of b.
func skipCommentOrCDATA(b []byte, i int) int {
	switch {
	case bytes.HasPrefix(b[i:], []byte("<!--")):
		if end := CommentEnd(b, i); end >= 0 {
			return end
		}
		return len(b)
	case bytes.HasPrefix(b[i:], []byte(cdataStart)):
		if end := bytes.Index(b[i+len(cdataStart):], []byte(cdataEnd)); end >= 0 {
			return i + len(cdataStart) + end + len(cdataEnd)
		}
		return len(b)
	}
	return i
}

// CommentEnd returns the index after the end of the comment whose "<!--" is
// at i, or -1 when it never ends. As htmlparser2 reads a comment, it ends at
// the first "-->" counted from the dashes of its "<!--": "<!-->" and
// "<!--->" are whole comments, empty ones.
func CommentEnd[T string | []byte](s T, i int) int {
	for j := i + len("<!--"); j < len(s); j++ {
		if s[j] == '>' && s[j-1] == '-' && s[j-2] == '-' {
			return j + 1
		}
	}
	return -1
}

// closeTagName returns the name of the close tag starting with the "</" at
// i, as htmlparser2 reads it (after the spaces that follow "</", up to a
// space or '>', '/' included), and the index after it.
func closeTagName(b []byte, i int) (string, int) {
	start := i + 2
	for start < len(b) && isHTMLSpace(b[start]) {
		start++
	}
	end := start
	for end < len(b) && b[end] != '>' && !isHTMLSpace(b[end]) {
		end++
	}
	return string(b[start:end]), end
}

// isHTMLSpace reports whether c is a space for htmlparser2.
func isHTMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// bogusCommentEnd returns the index after the '>' that ends the comment
// htmlparser2 reads from the '<' at i, or i when it reads none there: "</"
// followed, after its spaces, by anything but a letter or '>' (a doubled
// "</</mj-text>", or "</, shipped.") opens a comment that ends at the next
// '>', whosever it is. One that never ends runs to the end of b.
func bogusCommentEnd(b []byte, i int) int {
	if i+1 >= len(b) || b[i+1] != '/' {
		return i
	}
	j := i + 2
	for j < len(b) && isHTMLSpace(b[j]) {
		j++
	}
	if j == len(b) || b[j] == '>' || asciiLetter(b[j]) {
		return i
	}
	if gt := bytes.IndexByte(b[j:], '>'); gt >= 0 {
		return j + gt + 1
	}
	return len(b)
}

func asciiLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// tagNameAt returns the name of the opening or closing tag starting with the
// '<' at i, and the index after it, or "" when there is no tag there.
func tagNameAt(b []byte, i int) (string, int) {
	start := i + 1
	if start < len(b) && b[start] == '/' {
		// The name of a close tag starts after the spaces that follow
		// "</" (</ mj-text>), as htmlparser2 reads it.
		start++
		for start < len(b) && isHTMLSpace(b[start]) {
			start++
		}
	}
	// As htmlparser2, a tag name starts with an ASCII letter ("<3 you" or
	// "<- back" is text) and runs to a space, '/' or '>': "a_b", "a.b" and
	// non-ASCII names are names too.
	if start >= len(b) || !asciiLetter(b[start]) {
		return "", start
	}
	end := start
	for end < len(b) && !isTagNameEnd(b[end]) {
		end++
	}
	return string(b[start:end]), end
}

func isTagNameEnd(c byte) bool {
	return c == '>' || c == '/' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// findEndingTagClose reads the content of the ending tag lower, from from,
// as htmlparser2 does, up to the close tag that closes it: its own, or the
// first close of an element of open (an element around it). It returns the
// start and end of that close tag and, for an element of open, its index in
// open.names (-1 for its own close); start is -1 when nothing closes it
// before the end of the document. The elements opened in the content (the
// same ending tag included) are closed by their own close tags first;
// comments (with those of a "</" that opens no tag, bogusCommentEnd), CDATA
// sections and the text of <style> or <script> are skipped; other close
// tags are ignored.
func findEndingTagClose(b []byte, from int, lower string, open *tagStack) (start, end, ancestor int) {
	var inner tagStack
	for pos := from; pos < len(b); {
		lt := bytes.IndexByte(b[pos:], '<')
		if lt < 0 {
			break
		}
		i := pos + lt
		if skip := skipCommentOrCDATA(b, i); skip > i {
			pos = skip
			continue
		}
		if skip := bogusCommentEnd(b, i); skip > i {
			pos = skip
			continue
		}
		tag, _ := tagNameAt(b, i)
		if tag == "" {
			pos = i + 1
			continue
		}
		tagLower := strings.ToLower(tag)
		if b[i+1] == '/' {
			// A close tag ends at its first '>', quotes included, and
			// its name runs to a space or '>' (</mj-text foo> closes
			// mj-text, </mj-text/> closes nothing), as htmlparser2 reads
			// it.
			closeName, nameEnd := closeTagName(b, i)
			tagLower = strings.ToLower(closeName)
			gt := bytes.IndexByte(b[nameEnd:], '>')
			if gt < 0 {
				break
			}
			pos = nameEnd + gt + 1
			switch {
			case inner.has(tagLower):
				inner.close(tagLower)
			case tagLower == lower:
				return i, pos, -1
			case open.has(tagLower):
				for at := len(open.names) - 1; ; at-- {
					if open.names[at] == tagLower {
						return i, pos, at
					}
				}
			}
			continue
		}
		// An open tag is skipped whole, so that a quoted value
		// (<a title="<mj-text>">) is not read as markup.
		tagEnd, selfClosing := findTagEnd(b, i)
		if tagEnd < 0 {
			break
		}
		pos = tagEnd
		switch {
		case selfClosing || voidElements[tagLower]:
		case rawTextElements[tagLower]:
			// The content of <style> or <script> is text up to its own
			// close tag: a "<mj-raw>" in a CSS string is not an open.
			textEnd := indexFoldASCII(b[tagEnd:], "</"+tag)
			if textEnd < 0 {
				return -1, -1, -1
			}
			pos = tagEnd + textEnd
		default:
			inner.push(tagLower)
		}
	}
	return -1, -1, -1
}

// voidElements are the HTML elements htmlparser2 never keeps open.
var voidElements = map[string]bool{
	"area": true, "base": true, "basefont": true, "br": true, "col": true, "command": true,
	"embed": true, "frame": true, "hr": true, "image": true, "img": true, "input": true,
	"keygen": true, "link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true,
}

// rawTextElements are the HTML elements whose content htmlparser2 reads as
// text up to their own close tag.
var rawTextElements = map[string]bool{"script": true, "style": true, "textarea": true, "title": true}

// indexFoldASCII returns the index of the first occurrence of needle in b,
// ignoring ASCII case, or -1.
func indexFoldASCII(b []byte, needle string) int {
	n := []byte(needle)
	for i := 0; i+len(n) <= len(b); i++ {
		if bytes.EqualFold(b[i:i+len(n)], n) {
			return i
		}
	}
	return -1
}

// lowerTagNames returns s with the name of every open and close tag in
// lower case, comments, declarations (<!...>, <?...?>) and quoted values
// left as they are. s is copied only from its first change.
func lowerTagNames(s string) string {
	b := []byte(s)
	var out strings.Builder
	copied := 0
	for pos := 0; pos < len(b); {
		lt := bytes.IndexByte(b[pos:], '<')
		if lt < 0 {
			break
		}
		idx := pos + lt
		if skip := skipCommentOrCDATA(b, idx); skip > idx {
			pos = skip
			continue
		}
		name, nameEnd := tagNameAt(b, idx)
		if name == "" {
			pos = idx + 1
			continue
		}
		if lower := strings.ToLower(name); lower != name {
			if copied == 0 {
				out.Grow(len(s))
			}
			out.Write(b[copied : nameEnd-len(name)])
			out.WriteString(lower)
			copied = nameEnd
		}
		tagEnd, _ := findTagEnd(b, idx)
		if tagEnd < 0 {
			// The tag never ends: as in htmlparser2, the rest of the
			// document is in it, and no other tag follows. Scanning on
			// from its name would search the rest of the document for a
			// '>' again at each later '<'.
			break
		}
		pos = tagEnd
	}
	if copied == 0 {
		return s
	}
	out.Write(b[copied:])
	return out.String()
}

// commentEscape starts an encoded character in a comment: a private-use
// character, followed by commentDash for '-' or by commentEscapeSelf for
// itself.
const (
	commentEscape     = "\uE000"
	commentDash       = commentEscape + "\uE001"
	commentEscapeSelf = commentEscape + "\uE002"
)

// encodeCommentDashes returns s with the dashes encoded in every comment the
// XML decoder would refuse: one with "--" in it or ending with '-'
// (<!-- ---- HEADER ---- -->, <!-- a --->), which htmlparser2 reads up to
// its first "-->", as MJML keeps it. A comment with commentEscape in it is
// encoded too, so that decodeCommentDashes gives every comment back as
// written. The empty comments "<!-->" and "<!--->" (CommentEnd) are written
// "<!---->", as MJML writes them. A comment that never ends is left to the
// decoder's error. s is copied only from its first change.
func encodeCommentDashes(s string) string {
	var out strings.Builder
	copied := 0
	for pos := 0; ; {
		at := strings.Index(s[pos:], "<!--")
		if at < 0 {
			break
		}
		at += pos
		next := CommentEnd(s, at)
		if next < 0 {
			break
		}
		pos = next
		start, end := at+len("<!--"), next-len("-->")
		if end < start {
			// <!--> or <!--->, an empty comment for htmlparser2: written
			// as one the XML decoder reads.
			if copied == 0 {
				out.Grow(len(s) + 16)
			}
			out.WriteString(s[copied:at])
			out.WriteString("<!---->")
			copied = next
			continue
		}
		body := s[start:end]
		if !strings.Contains(body, "--") && !strings.HasSuffix(body, "-") && !strings.Contains(body, commentEscape) {
			continue
		}
		if copied == 0 {
			out.Grow(len(s) + 2*len(body))
		}
		out.WriteString(s[copied:start])
		out.WriteString(commentDashReplacer.Replace(body))
		copied = end
	}
	if copied == 0 {
		return s
	}
	out.WriteString(s[copied:])
	return out.String()
}

var (
	commentDashReplacer   = strings.NewReplacer(commentEscape, commentEscapeSelf, "-", commentDash)
	commentDecodeReplacer = strings.NewReplacer(commentEscapeSelf, commentEscape, commentDash, "-")
)

// decodeCommentDashes gives back a comment encodeCommentDashes encoded.
func decodeCommentDashes(comment string) string {
	if !strings.Contains(comment, commentEscape) {
		return comment
	}
	return commentDecodeReplacer.Replace(comment)
}

// outsideCDATA applies fn to the parts of content that are not CDATA
// sections, and keeps the CDATA sections as they are.
func outsideCDATA(content string, fn func(string) string) string {
	if !strings.Contains(content, cdataStart) {
		return fn(content)
	}
	// The builder starts at the first part fn changes: most templates need
	// no change outside their CDATA sections.
	var out strings.Builder
	changed := false
	emit := func(start, end int) { // the outside part content[start:end]
		part := content[start:end]
		result := fn(part)
		if !changed && result == part {
			return
		}
		if !changed {
			changed = true
			out.Grow(len(content))
			out.WriteString(content[:start])
		}
		out.WriteString(result)
	}
	segment := 0 // start of the part not yet written
	for pos := 0; pos < len(content); {
		lt := strings.IndexByte(content[pos:], '<')
		if lt < 0 {
			break
		}
		i := pos + lt
		switch {
		case strings.HasPrefix(content[i:], "<!--"):
			// A comment stays in the outside part, CDATA markers in it
			// included.
			if end := CommentEnd(content, i); end < 0 {
				pos = len(content)
			} else {
				pos = end
			}
		case strings.HasPrefix(content[i:], cdataStart):
			end := strings.Index(content[i:], cdataEnd)
			if end < 0 {
				pos = len(content)
				continue
			}
			end += i + len(cdataEnd)
			emit(segment, i)
			if changed {
				out.WriteString(content[i:end])
			}
			segment, pos = end, end
		default:
			pos = i + 1
		}
	}
	emit(segment, len(content))
	if !changed {
		return content
	}
	return out.String()
}

// findTagEnd returns the index *after* the '>' of the start tag at 'start'
// and whether it was self-closing (<.../>). Respects single/double quotes.
func findTagEnd(b []byte, start int) (end int, selfClosing bool) {
	i := start
	inQuote := byte(0)
	afterEquals := false // a quote opens a value only after =, as in htmlparser2
	for i < len(b) {
		c := b[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			i++
			continue
		}
		quoted := afterEquals
		afterEquals = valueMayStart(c, afterEquals)
		switch c {
		case '"', '\'':
			if quoted {
				inQuote = c
			}
			i++
		case '>':
			selfClosing = i > start && previousNonSpace(b, i-1) == '/'
			return i + 1, selfClosing
		default:
			i++
		}
	}
	return -1, false
}

// valueMayStart returns whether, after c, a quote would open an attribute
// value: right after an =, spaces apart, as htmlparser2 reads a tag.
// Elsewhere a quote is part of an attribute name (<b then 'x>), and the
// tag ends at its first >.
func valueMayStart(c byte, afterEquals bool) bool {
	switch c {
	case '=':
		return true
	case ' ', '\t', '\n', '\r', '\f':
		return afterEquals
	}
	return false
}

// previousNonSpace returns the previous non-space byte at or before idx; 0 if none.
func previousNonSpace(b []byte, idx int) byte {
	for i := idx; i >= 0; i-- {
		if b[i] != ' ' && b[i] != '\t' && b[i] != '\n' && b[i] != '\r' {
			return b[i]
		}
	}
	return 0
}

// syntaxError returns an XML syntax error at offset, with its line, as the
// decoder's own errors have one.
func syntaxError(lookup *lineLookup, offset int64, msg string) *xml.SyntaxError {
	line := 0
	if lookup != nil {
		line = lookup.Line(offset)
	}
	return &xml.SyntaxError{Msg: msg, Line: line}
}

// MaxNesting is the deepest nesting of elements a template may have, the
// root being at depth 1. Parsing, building the component tree and rendering
// recurse once per level: a deeper template is refused with a NestingError
// instead of exhausting the goroutine's stack, which would end the program.
// The content of ending tags (mj-text, mj-raw…) is not parsed, so its HTML
// does not count. MJML itself, whose render recurses too, fails long before
// such depths are useful.
const MaxNesting = 1000

// NestingError is returned for an element nested deeper than MaxNesting.
type NestingError struct {
	TagName string
	Line    int
}

func (e *NestingError) Error() string {
	return fmt.Sprintf("element <%s> is nested deeper than %d levels", e.TagName, MaxNesting)
}

// parseNode recursively parses XML nodes
// tagStart is the offset of the '<' opening the element, depth its nesting
// level (1 for the root).
func parseNode(decoder *xml.Decoder, start xml.StartElement, lookup *lineLookup, tagStart int64, depth int) (*MJMLNode, error) {
	node := &MJMLNode{
		XMLName:      start.Name,
		Attrs:        firstAttributes(start.Attr),
		Children:     make([]*MJMLNode, 0),
		MixedContent: make([]MixedContentPart, 0),
	}

	// If this is called with empty start element, get the first element
	if start.Name.Local == "" {
		for {
			before := decoder.InputOffset()
			tok, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			if se, ok := tok.(xml.StartElement); ok {
				node.XMLName = se.Name
				node.Attrs = firstAttributes(se.Attr)
				tagStart = before
				break
			}
			// What may come before the root, as MJML accepts it: an XML
			// declaration, processing instructions, a DOCTYPE and comments.
			switch tok.(type) {
			case xml.ProcInst, xml.Directive, xml.Comment:
				continue
			}
			// The line breaks kept before the root element.
			if cd, ok := tok.(xml.CharData); !ok || len(bytes.TrimSpace(cd)) != 0 {
				// At the first character that is not a line break or a
				// space, for the line of the error.
				at := before
				if ok {
					at += int64(len(cd) - len(bytes.TrimLeft(cd, " \t\r\n")))
				}
				return nil, syntaxError(lookup, at, "expected start element")
			}
		}
	}

	// The name as written closes the element; the node gets it in lower
	// case, as htmlparser2 (lowerCaseTags) gives it to MJML: <MJ-SECTION> is
	// an mj-section.
	openName := node.XMLName
	node.XMLName.Local = strings.ToLower(node.XMLName.Local)

	// Every element gets its line: validation warnings report it, for tags
	// without attributes too.
	node.StartOffset = tagStart
	if lookup != nil {
		node.LineNumber = lookup.Line(tagStart)
	}

	var textBuilder strings.Builder
	var segmentBuilder strings.Builder

	flushSegment := func() {
		if segmentBuilder.Len() > 0 {
			text := segmentBuilder.String()
			node.MixedContent = append(node.MixedContent, MixedContentPart{Text: text})
			segmentBuilder.Reset()
		}
	}

	for {
		tokenStart := decoder.InputOffset()
		tok, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			flushSegment()
			if depth >= MaxNesting {
				line := 0
				if lookup != nil {
					line = lookup.Line(tokenStart)
				}
				return nil, &NestingError{TagName: strings.ToLower(t.Name.Local), Line: line}
			}
			child, err := parseNode(decoder, t, lookup, tokenStart, depth+1)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
			node.MixedContent = append(node.MixedContent, MixedContentPart{Node: child})

		case xml.EndElement:
			if t.Name == openName {
				flushSegment()
				node.Text = textBuilder.String()
				return node, nil
			}
			return nil, syntaxError(lookup, tokenStart, "unexpected end element: "+t.Name.Local)

		case xml.CharData:
			textBuilder.Write(t)
			segmentBuilder.Write(t)
		case xml.Comment:
			// Preserve comments as part of text content
			comment := decodeCommentDashes(string(t))
			textBuilder.WriteString("<!--")
			textBuilder.WriteString(comment)
			textBuilder.WriteString("-->")
			segmentBuilder.WriteString("<!--")
			segmentBuilder.WriteString(comment)
			segmentBuilder.WriteString("-->")
		}
	}
}

// firstAttributes returns attrs with only the first occurrence of each
// name, as htmlparser2 gives an element's attributes to MJML: a repeated
// attribute (font-size="12px" ... font-size="14px") keeps its first value,
// for the component, its definitions in mj-attributes and its validation.
// attrs is returned as it is when no name repeats.
func firstAttributes(attrs []xml.Attr) []xml.Attr {
	if !hasRepeatedName(attrs) {
		return attrs
	}
	out := make([]xml.Attr, 0, len(attrs))
	seen := make(map[xml.Name]bool, len(attrs))
	for _, a := range attrs {
		if !seen[a.Name] {
			seen[a.Name] = true
			out = append(out, a)
		}
	}
	return out
}

// hasRepeatedName reports whether two attributes have the same name, without
// allocating for the few attributes of a usual tag.
func hasRepeatedName(attrs []xml.Attr) bool {
	if len(attrs) <= 16 {
		for i := 1; i < len(attrs); i++ {
			for j := range i {
				if attrs[i].Name == attrs[j].Name {
					return true
				}
			}
		}
		return false
	}
	seen := make(map[xml.Name]bool, len(attrs))
	for _, a := range attrs {
		if seen[a.Name] {
			return true
		}
		seen[a.Name] = true
	}
	return false
}

// GetAttribute retrieves an attribute value by name
func (n *MJMLNode) GetAttribute(name string) string {
	for _, attr := range n.Attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

// LookupAttribute returns the value of the attribute written on the tag and
// whether it is written there, an empty value included.
func (n *MJMLNode) LookupAttribute(name string) (string, bool) {
	for _, attr := range n.Attrs {
		if attr.Name.Local == name {
			return attr.Value, true
		}
	}
	return "", false
}

// GetTagName returns the local name of the XML tag
func (n *MJMLNode) GetTagName() string {
	return n.XMLName.Local
}

// GetLineNumber returns the starting line number for this node in the processed MJML source.
func (n *MJMLNode) GetLineNumber() int {
	if n == nil || n.LineNumber <= 0 {
		return 1
	}
	return n.LineNumber
}

// GetTextContent returns the trimmed text content
func (n *MJMLNode) GetTextContent() string {
	return strings.TrimSpace(n.Text)
}

// GetMixedContent returns the full mixed content including HTML child elements
// This reconstructs the original content like "Share <b>test</b> hi" from the AST
func (n *MJMLNode) GetMixedContent() string {
	if debug.Enabled() {
		debug.DebugLogWithData("parser", "mixed-content", "Processing mixed content", map[string]any{
			"tag_name":       n.XMLName.Local,
			"plain_text":     n.Text,
			"children_count": len(n.Children),
			"has_children":   len(n.Children) > 0,
		})
	}

	if len(n.MixedContent) == 0 {
		result := strings.TrimSpace(n.Text)
		if debug.Enabled() {
			debug.DebugLogWithData("parser", "text-only", "Returning plain text content", map[string]any{
				"content": result,
			})
		}
		return result
	}

	var result strings.Builder
	for i, part := range n.MixedContent {
		if part.Node != nil {
			tag := part.Node.XMLName.Local
			result.WriteString("<")
			result.WriteString(tag)
			for _, attr := range part.Node.Attrs {
				result.WriteString(" ")
				result.WriteString(attr.Name.Local)
				result.WriteString("=\"")
				result.WriteString(attr.Value)
				result.WriteString("\"")
			}
			if isVoidHTMLElement(tag) {
				result.WriteString(" />")
				continue
			}
			result.WriteString(">")
			result.WriteString(part.Node.GetMixedContent())
			result.WriteString("</")
			result.WriteString(tag)
			result.WriteString(">")
		} else {
			text := part.Text
			if i == 0 {
				text = strings.TrimLeft(text, " \n\r\t")
			}
			if i == len(n.MixedContent)-1 {
				text = strings.TrimRight(text, " \n\r\t")
			}
			result.WriteString(text)
		}
	}

	finalResult := strings.TrimSpace(result.String())
	if debug.Enabled() {
		debug.DebugLogWithData("parser", "mixed-complete", "Mixed content reconstructed", map[string]any{
			"original_text":     n.Text,
			"final_content":     finalResult,
			"children_rendered": len(n.Children),
		})
	}
	return finalResult
}

// HasRenderableMixedContent reports whether GetMixedContent would return a
// non-empty string, without paying the cost of building it. A child node
// part always renders at least its own tag markup, so its presence alone is
// enough; text parts must contain non-whitespace to count.
func (n *MJMLNode) HasRenderableMixedContent() bool {
	if len(n.MixedContent) == 0 {
		return strings.TrimSpace(n.Text) != ""
	}
	for _, part := range n.MixedContent {
		if part.Node != nil {
			return true
		}
		if strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}

// FindFirstChild finds the first child with the given tag name
func (n *MJMLNode) FindFirstChild(tagName string) *MJMLNode {
	for _, child := range n.Children {
		if child.GetTagName() == tagName {
			return child
		}
	}
	return nil
}

// FindAllChildren finds all children with the given tag name
func (n *MJMLNode) FindAllChildren(tagName string) []*MJMLNode {
	var result []*MJMLNode
	for _, child := range n.Children {
		if child.GetTagName() == tagName {
			result = append(result, child)
		}
	}
	return result
}
