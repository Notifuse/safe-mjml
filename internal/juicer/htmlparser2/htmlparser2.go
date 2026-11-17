// Package htmlparser2 parses HTML as htmlparser2 does and serializes it as
// dom-serializer does, which is what juice uses (cheerio is loaded with
// decodeEntities:false).
//
// This deliberately skips HTML5 tree construction: no implied <tbody>, no
// <html>/<head>/<body> synthesis, no foster parenting, no reconstruction of
// active formatting elements. x/net/html's own Parse would do all of those and
// would decode entities besides, and a mail client can see the difference.
//
// The tree is built out of *html.Node so cascadia can match against it
// directly, but it is populated by hand and differs from what html.Parse
// produces in two ways worth knowing:
//
//   - Attr[i].Val and text hold RAW, UNDECODED source bytes. "&nbsp;" stays
//     "&nbsp;". This is also what cheerio matches against, so selectors behave
//     the same.
//   - Comment and Doctype nodes keep their full source text in Data,
//     delimiters included, because bogus constructs like <!decl> and <?pi?>
//     have to round-trip verbatim.
//
// DataAtom is set on elements whose name is an atom, as html.Parse sets it,
// for the pseudo-classes that read it.
package htmlparser2

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var voidElements = map[string]bool{
	"area": true, "base": true, "basefont": true, "br": true, "col": true,
	"command": true, "embed": true, "frame": true, "hr": true, "img": true,
	"input": true, "isindex": true, "keygen": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// openImpliesClose mirrors htmlparser2's table: opening the key tag pops the
// stack while the element on top is in the value set. It tests only the top of
// the stack, which is why <ul><li>a<ul><li>b</ul> nests rather than closing
// the outer <li>.
var openImpliesClose = map[string]map[string]bool{
	"tr":       {"tr": true, "th": true, "td": true},
	"th":       {"th": true},
	"td":       {"thead": true, "th": true, "td": true},
	"body":     {"head": true, "link": true, "script": true},
	"li":       {"li": true},
	"option":   {"option": true},
	"optgroup": {"optgroup": true, "option": true},
	"dd":       {"dt": true, "dd": true},
	"dt":       {"dt": true, "dd": true},
	"rt":       {"rt": true, "rp": true},
	"rp":       {"rt": true, "rp": true},
	"tbody":    {"thead": true, "tbody": true},
	"tfoot":    {"thead": true, "tbody": true},
}

var formTags = map[string]bool{
	"input": true, "option": true, "optgroup": true, "select": true,
	"button": true, "datalist": true, "textarea": true,
}

func init() {
	for _, t := range []string{"select", "input", "output", "button", "datalist", "textarea"} {
		openImpliesClose[t] = formTags
	}
	pTag := map[string]bool{"p": true}
	for _, t := range []string{
		"p", "h1", "h2", "h3", "h4", "h5", "h6",
		"address", "article", "aside", "blockquote", "details", "div", "dl",
		"fieldset", "figcaption", "figure", "footer", "form", "header",
		"hr", "main", "nav", "ol", "pre", "section", "table", "ul",
	} {
		openImpliesClose[t] = pTag
	}
}

// rawTextEnds are the close sequences of the elements htmlparser2's
// tokenizer reads as raw text, up to their close tag. x/net/html's tokenizer
// reads more elements so (iframe, noembed, noframes, noscript, plaintext,
// textarea, xmp), which htmlparser2 reads as markup: Parse never uses its raw
// text.
var rawTextEnds = map[string]string{"script": "</script", "style": "</style", "title": "</title"}

// textareaEnd is the close sequence of a textarea, which htmlparser2 reads
// as raw text since 9.0 (juice 11), with Options.RawTextarea.
const textareaEnd = "</textarea"

// Options are the options of ParseWith.
type Options struct {
	// XML parses in htmlparser2's XML mode: names keep their case, any tag
	// can close itself with "/>", and no element is void, raw text, or
	// closed by another's opening.
	XML bool
	// MaxDepth, when positive, stops the parse with ErrTooDeep at an
	// element nested deeper.
	MaxDepth int
	// RawTextarea reads the content of a textarea as raw text, up to its
	// close tag, as htmlparser2 9 does (juice 11); htmlparser2 8 (juice 10)
	// reads it as markup.
	RawTextarea bool
}

// htmlIntegrationElements are htmlparser2's: in svg or math, they return to
// HTML, where "/>" closes nothing.
var htmlIntegrationElements = map[string]bool{
	"mi": true, "mo": true, "mn": true, "ms": true, "mtext": true,
	"annotation-xml": true, "foreignobject": true, "desc": true, "title": true,
}

func isForeign(tag string) bool { return tag == "svg" || tag == "math" }

// ErrTooDeep is the error of Parse for elements nested deeper than its
// maxDepth.
var ErrTooDeep = errors.New("htmlparser2: elements nest deeper than the limit")

// Parse builds an htmlparser2-shaped tree, in its XML mode when xml is set:
// names keep their case, any tag can close itself with "/>", and no element
// is void, raw text, or closed by another's opening. The returned root is a
// synthetic container whose children are the document's top-level nodes.
// When maxDepth is positive, an element nested deeper stops the parse with
// ErrTooDeep.
func Parse(src []byte, xml bool, maxDepth int) (*html.Node, error) {
	return ParseWith(src, Options{XML: xml, MaxDepth: maxDepth})
}

// ParseWith is Parse with its options, RawTextarea among them.
func ParseWith(src []byte, opts Options) (*html.Node, error) {
	xml, maxDepth := opts.XML, opts.MaxDepth
	root := &html.Node{Type: html.DocumentNode}
	cur := root
	// Nodes come from slabs that double up to 256, so a small document pays
	// for little and a large one allocates once per 256 nodes.
	// ponytail: a slab stays alive while any of its nodes does; per-node allocation if detached subtrees must free early.
	var slab []html.Node
	newNode := func(n html.Node) *html.Node {
		if len(slab) == cap(slab) {
			slab = make([]html.Node, 0, min(max(2*cap(slab), 8), 256))
		}
		slab = append(slab, n)
		return &slab[len(slab)-1]
	}
	// Text, attribute values and XML names are substrings of one copy of src.
	text := string(src)
	// Attributes come from slabs too. Each carve's capacity is capped, so an
	// append reallocates rather than overwriting the next element's.
	var scratch, attrSlab []html.Attribute
	var seen map[string]bool
	startTag := func(b string, lower bool) (string, []html.Attribute, bool) {
		name, a, selfClosing := scanStartTag(b, lower, scratch[:0], &seen)
		scratch = a
		if len(a) == 0 {
			return name, nil, selfClosing
		}
		if cap(attrSlab)-len(attrSlab) < len(a) {
			attrSlab = make([]html.Attribute, 0, max(len(a), min(max(2*cap(attrSlab), 8), 256)))
		}
		k := len(attrSlab)
		attrSlab = append(attrSlab, a...)
		return name, attrSlab[k:len(attrSlab):len(attrSlab)], selfClosing
	}
	// The open elements, htmlparser2's stack, with the number open of each
	// name, so that a close tag knows at once whether it closes one: walking
	// the stack for each stray close tag made a deep document quadratic.
	var stack []*html.Node
	open := map[string]int{}
	push := func(n *html.Node) {
		stack = append(stack, n)
		open[n.Data]++
		cur = n
	}
	pop := func() {
		open[cur.Data]--
		stack[len(stack)-1] = nil
		stack = stack[:len(stack)-1]
		cur = root
		if len(stack) > 0 {
			cur = stack[len(stack)-1]
		}
	}
	// appendElement adds an element to the open one, unless it would nest
	// deeper than maxDepth.
	tooDeep := false
	appendElement := func(name string, attrs []html.Attribute) *html.Node {
		if maxDepth > 0 && len(stack) >= maxDepth {
			tooDeep = true
			return nil
		}
		n := newNode(html.Node{Type: html.ElementNode, DataAtom: atom.Lookup([]byte(name)), Data: name, Attr: attrs})
		cur.AppendChild(n)
		return n
	}
	// foreign is htmlparser2's foreignContext: svg and math push true, an
	// HTML integration point false, whether the element closes itself or
	// not, and a close tag of one of those names pops, whether it closes an
	// element or not. "/>" closes an element when the top is true.
	var foreign []bool
	z := html.NewTokenizer(bytes.NewReader(src))
	pos := 0 // offset in src of the tokenizer's next byte

	closeTag := func(name string) {
		if (isForeign(name) || htmlIntegrationElements[name]) && len(foreign) > 0 {
			foreign = foreign[:len(foreign)-1]
		}
		if open[name] > 0 {
			for cur.Data != name {
				pop()
			}
			pop()
		} else if !xml && (name == "br" || name == "p") {
			// htmlparser2 turns an unmatched </br> into <br> and an
			// unmatched </p> into an empty <p></p>.
			appendElement(name, nil)
		}
		// Any other unmatched end tag is dropped.
	}
	// resume restarts the tokenizer where htmlparser2 says a construct ends,
	// when x/net/html ended it elsewhere.
	resume := func(end int) {
		if end != pos {
			pos = end
			z = html.NewTokenizer(bytes.NewReader(src[end:]))
		}
	}

	for !tooDeep {
		if quotedCloseTag(src, pos) {
			// x/net/html reads the quoted values of a close tag: a quoted
			// ">" does not end it, and a quote nothing closes runs to the
			// end of the input. htmlparser2 ends a close tag at its first
			// ">". Such close tags are read here, a run of them at once, so
			// that the tokenizer never reads one to the end of the run and
			// restarts once after it.
			end := pos
			for !tooDeep {
				if quotedCloseTag(src, end) {
					var name string
					_, name, end = closeTagLike(src, text, end, xml)
					if name != "" {
						closeTag(foldName(name, !xml))
					}
					continue
				}
				// The text between two such close tags is read here too
				// (what is left of </b a='>'> after its first ">"): a
				// tokenizer restarted after each tag allocated its 4 KB
				// buffer again, 140 MB for 32,000 tags. It holds no "<",
				// so x/net/html would make it one text token.
				k := bytes.IndexByte(src[end:], '<')
				if k <= 0 || !quotedCloseTag(src, end+k) {
					break
				}
				cur.AppendChild(newNode(html.Node{Type: html.TextNode, Data: text[end : end+k]}))
				end += k
			}
			resume(end)
			continue
		}
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		// Slice the token out of src rather than z.Raw(), which points into a
		// buffer the tokenizer reuses.
		start := pos
		pos += len(z.Raw())
		raw := src[start:pos]

		switch tt {
		case html.TextToken:
			// In XML mode a tag name can start with any character but
			// whitespace, "/" and ">", where x/net/html wants a letter.
			k := -1
			if xml {
				k = xmlTagStart(raw)
			}
			if k >= 0 {
				if k > 0 {
					cur.AppendChild(newNode(html.Node{Type: html.TextNode, Data: text[start : start+k]}))
				}
				s := start + k
				e := tagEnd(src, s)
				if e < 0 {
					// htmlparser2 drops a tag left open at the end of input.
					resume(len(src))
					continue
				}
				name, attrs, selfClosing := startTag(text[s:e], false)
				if n := appendElement(name, attrs); n != nil && !selfClosing {
					push(n)
				}
				resume(e)
				continue
			}
			cur.AppendChild(newNode(html.Node{Type: html.TextNode, Data: text[start:pos]}))

		case html.CommentToken, html.DoctypeToken:
			var n *html.Node
			var end int
			if bytes.HasPrefix(raw, []byte("</")) {
				var name string
				n, name, end = closeTagLike(src, text, start, xml)
				if name != "" {
					closeTag(foldName(name, !xml))
				}
			} else {
				n, end = markupDecl(src, start, xml)
			}
			if n != nil {
				cur.AppendChild(n)
			}
			resume(end)

		case html.StartTagToken, html.SelfClosingTagToken:
			// x/net/html's raw text is not htmlparser2's: the element's
			// content is read as markup, or below as htmlparser2 reads raw
			// text.
			z.NextIsNotRawText()
			name, attrs, selfClosing := startTag(text[start:pos], !xml)
			if name == "" {
				continue
			}
			if xml {
				if n := appendElement(name, attrs); n != nil && !selfClosing {
					push(n)
				}
				continue
			}
			if set := openImpliesClose[name]; set != nil {
				for cur != root && set[cur.Data] {
					pop()
				}
			}
			n := appendElement(name, attrs)
			if n == nil || voidElements[name] {
				continue
			}
			push(n)
			if isForeign(name) {
				foreign = append(foreign, true)
			} else if htmlIntegrationElements[name] {
				foreign = append(foreign, false)
			}
			// Outside foreign content htmlparser2 ignores the solidus, so
			// <div/> opens a div rather than closing it, and the content of
			// a <style/> is markup.
			if selfClosing {
				if len(foreign) > 0 && foreign[len(foreign)-1] {
					pop()
				}
				continue
			}
			seq, ok := rawTextEnds[name]
			if !ok && opts.RawTextarea && name == "textarea" {
				seq, ok = textareaEnd, true
			}
			if ok {
				end := rawTextEnd(src, pos, seq)
				if end > pos {
					cur.AppendChild(newNode(html.Node{Type: html.TextNode, Data: text[pos:end]}))
				}
				if end == len(src) {
					resume(end)
					continue
				}
				// The close tag's name is the sequence's, and the tag runs
				// to the next ">".
				closeTag(name)
				end += len(seq)
				if k := bytes.IndexByte(src[end:], '>'); k >= 0 {
					resume(end + k + 1)
				} else {
					resume(len(src))
				}
			}

		case html.EndTagToken:
			// The tag runs to the first ">", in a quoted value or not.
			_, name, end := closeTagLike(src, text, start, xml)
			if name != "" {
				closeTag(foldName(name, !xml))
			}
			resume(end)
		}
	}
	if tooDeep {
		return nil, ErrTooDeep
	}
	mergeText(root)
	// Anything still open at EOF stays open; the serializer closes it.
	return root, nil
}

// rawTextEnd is where the text of a script, style, title or textarea element that
// starts at src[from] ends, as htmlparser2's tokenizer finds it: at the first
// seq ("</style" for a style) followed by ">" or whitespace, comparing each
// byte ORed with 0x20 to seq's, or at the end of src.
func rawTextEnd(src []byte, from int, seq string) int {
	idx := 0
	for i := from; i < len(src); i++ {
		c := src[i]
		if idx == len(seq) {
			if c == '>' || isSpace(c) {
				return i - len(seq)
			}
			idx = 0
		}
		switch {
		case c|0x20 == seq[idx]:
			idx++
		case idx == 0:
			// Outside a title, htmlparser2 skips to the next "<".
			if seq != "</title" {
				k := bytes.IndexByte(src[i+1:], '<')
				if k < 0 {
					return len(src)
				}
				i += k + 1
				idx = 1
			}
		default:
			idx = 0
			if c == '<' {
				idx = 1
			}
		}
	}
	return len(src)
}

// mergeText joins adjacent text nodes, as htmlparser2's DOM handler does.
func mergeText(root *html.Node) {
	Traverse(root, func(n *html.Node) bool {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.TextNode || c.NextSibling == nil || c.NextSibling.Type != html.TextNode {
				continue
			}
			var b strings.Builder
			b.WriteString(c.Data)
			for d := c.NextSibling; d != nil && d.Type == html.TextNode; d = c.NextSibling {
				b.WriteString(d.Data)
				n.RemoveChild(d)
			}
			c.Data = b.String()
		}
		return true
	}, func(*html.Node) {})
}

// markupDecl reads the "<!" or "<?" construct at src[p:] the way htmlparser2's
// tokenizer does, which is not the HTML5 spec x/net/html follows. It returns
// the node produced, if any, and where the construct ends.
//
//   - <!-- ends at the first "-->", counting the opening dashes, so <!--> is
//     an empty comment and "--!>" ends nothing.
//   - <![CDATA[ (exact case) ends at "]]>" and becomes a comment, since there
//     is no CDATA outside XML mode.
//   - Anything else ends at the next ">", but the character right after "<!"
//     or "<!-" is never that ">".
//   - Unterminated at end of input, a comment or CDATA is closed, and
//     anything else becomes text minus its "<!" or "<?".
func markupDecl(src []byte, p int, xml bool) (*html.Node, int) {
	n := len(src)
	tail := func() (*html.Node, int) {
		if p+2 >= n {
			return nil, n
		}
		return &html.Node{Type: html.TextNode, Data: string(src[p+2:])}, n
	}
	untilGt := func(from int) (*html.Node, int) {
		k := bytes.IndexByte(src[min(from, n):], '>')
		if k < 0 {
			return tail()
		}
		end := from + k + 1
		typ := html.CommentNode
		if len(src)-p >= 9 && bytes.EqualFold(src[p:p+9], []byte("<!doctype")) {
			typ = html.DoctypeNode
		}
		return &html.Node{Type: typ, Data: string(src[p:end])}, end
	}

	i := p + 2
	switch {
	case src[p+1] == '?':
		return untilGt(i)
	case i >= n:
		return tail()
	case src[i] == '[':
		const seq = "CDATA["
		j := i + 1
		for j < n && j-i-1 < len(seq) && src[j] == seq[j-i-1] {
			j++
		}
		if j-i-1 == len(seq) {
			if xml {
				return commentLike(src, j, "]]>", 0, "<![CDATA[", "]]>")
			}
			return commentLike(src, j, "]]>", 0, "<!--[CDATA[", "]]-->")
		}
		if j >= n {
			return tail()
		}
		return untilGt(j) // the mismatched character is reconsumed
	case src[i] == '-':
		if i+1 >= n {
			return tail()
		}
		if src[i+1] == '-' {
			return commentLike(src, i+2, "-->", 2, "<!--", "-->")
		}
		return untilGt(i + 2)
	default:
		return untilGt(i + 1)
	}
}

// quotedCloseTag reports whether src[p:] is a close tag x/net/html's
// tokenizer reads past its first ">": "</", a letter, and a quote before that
// ">", or before the end of src when there is none.
func quotedCloseTag(src []byte, p int) bool {
	if p+2 >= len(src) || src[p] != '<' || src[p+1] != '/' {
		return false
	}
	if c := src[p+2] | 0x20; c < 'a' || c > 'z' {
		return false
	}
	end := len(src)
	if k := bytes.IndexByte(src[p:], '>'); k >= 0 {
		end = p + k
	}
	return bytes.ContainsAny(src[p:end], `'"`)
}

// closeTagLike reads the "</" construct at src[p:] the way htmlparser2 does,
// where x/net/html makes a bogus comment of anything not starting with a
// letter: whitespace after the slash is skipped, "</>" stays as text, a
// letter starts an end tag running to the next ">", and anything else is a
// comment up to it. It returns a node or an end tag name, a substring of
// text, the string of src, and where the construct ends.
func closeTagLike(src []byte, text string, p int, xml bool) (n *html.Node, name string, end int) {
	i := p + 2
	for i < len(src) && isSpace(src[i]) {
		i++
	}
	if i >= len(src) {
		return &html.Node{Type: html.TextNode, Data: string(src[p:])}, "", len(src)
	}
	k := bytes.IndexByte(src[i:], '>')
	switch c := src[i]; {
	case c == '>':
		// Back to text without moving the text start, so it is kept verbatim.
		return &html.Node{Type: html.TextNode, Data: string(src[p : i+1])}, "", i + 1
	case 'a' <= c|0x20 && c|0x20 <= 'z' || xml:
		if k < 0 {
			return nil, "", len(src)
		}
		j := i
		for j < i+k && !isSpace(src[j]) {
			j++
		}
		return nil, text[i:j], i + k + 1
	case k < 0:
		return &html.Node{Type: html.TextNode, Data: string(src[i:])}, "", len(src)
	default:
		return &html.Node{Type: html.CommentNode, Data: "<!--" + string(src[i:i+k]) + "-->"}, "", i + k + 1
	}
}

// commentLike scans from the start of a comment's or CDATA section's content
// for its end sequence, with htmlparser2's matching: the sequence may already
// be partly matched (idx), and a run of its first character is allowed, as in
// "--->". The content is re-emitted between open and close.
func commentLike(src []byte, from int, seq string, idx int, open, close string) (*html.Node, int) {
	node := func(data []byte) *html.Node {
		return &html.Node{Type: html.CommentNode, Data: open + string(data) + close}
	}
	for k := from; k < len(src); k++ {
		switch c := src[k]; {
		case c == seq[idx]:
			if idx++; idx == len(seq) {
				return node(src[from:max(from, k-2)]), k + 1
			}
		case idx > 0 && c != seq[idx-1]:
			idx = 0
		}
	}
	if from >= len(src) {
		return nil, len(src)
	}
	return node(src[from:]), len(src)
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// lowerASCII returns s lowercased: atom's static string for a known tag or
// attribute name, s itself when it has no uppercase, or else a new string.
func lowerASCII(s string) string {
	var stack [32]byte
	l, upper := stack[:0], false
	if len(s) > len(stack) {
		l = make([]byte, 0, len(s))
	}
	l = l[:len(s)]
	for i := range len(s) {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
			upper = true
		}
		l[i] = c
	}
	if a := atom.Lookup(l); a != 0 {
		return a.String()
	}
	if !upper {
		return s
	}
	return string(l)
}

// scanStartTag reads a tag name and attributes from a raw start tag, keeping
// values exactly as written and appending them to attrs. x/net/html's TagAttr
// decodes entities, which loses the difference between "&amp;" and a literal "&".
// It reports whether the tag closes itself, as htmlparser2's tokenizer reads
// it: its ">" follows a "/" and whitespace, outside a value. seen is a set of
// names kept between calls, used for a tag with many attributes.
func scanStartTag(b string, lower bool, attrs []html.Attribute, seen *map[string]bool) (string, []html.Attribute, bool) {
	i := 1 // skip '<'
	start := i
	for i < len(b) && !isSpace(b[i]) && b[i] != '>' && b[i] != '/' {
		i++
	}
	name := foldName(b[start:i], lower)
	if name == "" {
		return "", attrs, false
	}

	selfClosing := false
	for i < len(b) {
		for i < len(b) && (isSpace(b[i]) || b[i] == '/') {
			selfClosing = selfClosing || b[i] == '/'
			i++
		}
		if i >= len(b) || b[i] == '>' {
			break
		}
		selfClosing = false
		// Any first character starts a name, "=" included, as in htmlparser2.
		ns := i
		i++
		for i < len(b) && !isSpace(b[i]) && b[i] != '=' && b[i] != '>' && b[i] != '/' {
			i++
		}
		an := foldName(b[ns:i], lower)
		for i < len(b) && isSpace(b[i]) {
			i++
		}
		var val string
		if i < len(b) && b[i] == '=' {
			i++
			for i < len(b) && isSpace(b[i]) {
				i++
			}
			if i < len(b) && (b[i] == '"' || b[i] == '\'') {
				q := b[i]
				i++
				vs := i
				for i < len(b) && b[i] != q {
					i++
				}
				val = b[vs:i]
				if i < len(b) {
					i++
				}
			} else {
				vs := i
				for i < len(b) && !isSpace(b[i]) && b[i] != '>' {
					i++
				}
				val = b[vs:i]
			}
		}
		if an == "" {
			continue
		}
		// htmlparser2 keeps the first occurrence of a repeated attribute.
		if !repeatedAttribute(attrs, an, seen) {
			attrs = append(attrs, html.Attribute{Key: an, Val: val})
		}
	}
	return name, attrs, selfClosing
}

// manyAttributes is the number of attributes past which a tag's names are
// looked up in a set rather than in the list: one start tag with 100,000
// attributes took seconds.
const manyAttributes = 16

// repeatedAttribute reports whether attrs, the attributes of one tag so far,
// has name. Past manyAttributes it keeps the names in *seen, cleared for each
// tag.
func repeatedAttribute(attrs []html.Attribute, name string, seen *map[string]bool) bool {
	if len(attrs) < manyAttributes {
		for k := range attrs {
			if attrs[k].Key == name {
				return true
			}
		}
		return false
	}
	if len(attrs) == manyAttributes {
		if *seen == nil {
			*seen = map[string]bool{}
		}
		clear(*seen)
		for k := range attrs {
			(*seen)[attrs[k].Key] = true
		}
	}
	if (*seen)[name] {
		return true
	}
	(*seen)[name] = true
	return false
}

// xmlTagStart finds a "<" in text that htmlparser2's XML mode reads as the
// start of a tag, or -1.
func xmlTagStart(text []byte) int {
	for i := 0; i+1 < len(text); i++ {
		if c := text[i+1]; text[i] == '<' && !isSpace(c) && c != '/' && c != '>' {
			return i
		}
	}
	return -1
}

// tagEnd finds the end of the start tag at src[s], past its ">", skipping
// quoted attribute values, or -1 at the end of input.
func tagEnd(src []byte, s int) int {
	for i := s + 1; i < len(src); i++ {
		switch src[i] {
		case '>':
			return i + 1
		case '=':
			j := i + 1
			for j < len(src) && isSpace(src[j]) {
				j++
			}
			if j < len(src) && (src[j] == '"' || src[j] == '\'') {
				k := bytes.IndexByte(src[j+1:], src[j])
				if k < 0 {
					return -1
				}
				i = j + 1 + k
			}
		}
	}
	return -1
}

// foldName lowercases a name outside XML mode, as JavaScript's toLowerCase does.
func foldName(b string, lower bool) string {
	if !lower {
		return b
	}
	for i := range len(b) {
		if b[i] >= utf8.RuneSelf {
			return lowerJS(b)
		}
	}
	return lowerASCII(b)
}

// lowerJS is toLowerCase for non-ASCII: Go's simple mappings plus SpecialCasing's
// unconditional İ and final sigma.
func lowerJS(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i, r := range s {
		switch {
		case r == 'İ':
			sb.WriteString("i\u0307")
		case r == 'Σ' && finalSigma(s, i):
			sb.WriteRune('ς')
		default:
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return sb.String()
}

// finalSigma reports whether the Σ at s[i] ends a word: a cased letter before it
// and none after, skipping case-ignorable runes.
func finalSigma(s string, i int) bool {
	before := false
	for j := i; j > 0; {
		r, n := utf8.DecodeLastRuneInString(s[:j])
		j -= n
		if !caseIgnorable(r) {
			before = cased(r)
			break
		}
	}
	if !before {
		return false
	}
	for _, r := range s[i+len("Σ"):] {
		if !caseIgnorable(r) {
			return !cased(r)
		}
	}
	return true
}

func cased(r rune) bool { return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r) }

// ponytail: approximates Unicode's Case_Ignorable by category; use the derived property if a name ever needs it.
func caseIgnorable(r rune) bool {
	return strings.ContainsRune("'.:^`·", r) || unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}

// Render serializes the tree the way dom-serializer does with
// decodeEntities:false: text is written untouched and only the double quote is
// escaped inside attribute values.
func Render(buf *bytes.Buffer, root *html.Node) {
	// Inside svg and math, up to an integration point, dom-serializer is in foreign
	// mode: it restores mixed-case names, keeps empty attributes' ="", and closes
	// childless elements itself.
	type frame struct {
		name    string
		foreign bool
	}
	var open []frame
	Traverse(root, func(n *html.Node) bool {
		switch n.Type {
		case html.TextNode, html.DoctypeNode, html.CommentNode:
			// Comments hold their markup as htmlparser2 would serialize it.
			buf.WriteString(n.Data)
			return false
		case html.ElementNode:
			f := frame{name: n.Data}
			if len(open) > 0 && open[len(open)-1].foreign {
				f.name = cmp.Or(foreignElementNames[n.Data], n.Data)
				f.foreign = !foreignIntegrationPoints[open[len(open)-1].name]
			}
			f.foreign = f.foreign || n.Data == "svg" || n.Data == "math"
			open = append(open, f)
			buf.WriteByte('<')
			buf.WriteString(f.name)
			for _, a := range jsKeyOrder(n.Attr) {
				buf.WriteByte(' ')
				if f.foreign {
					buf.WriteString(cmp.Or(foreignAttributeNames[a.Key], a.Key))
				} else {
					buf.WriteString(a.Key)
				}
				if a.Val != "" || f.foreign {
					buf.WriteString(`="`)
					writeAttrValue(buf, a.Val)
					buf.WriteByte('"')
				}
			}
			if f.foreign && n.FirstChild == nil {
				buf.WriteString("/>")
				return false
			}
			buf.WriteByte('>')
		}
		return true
	}, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		f := open[len(open)-1]
		open = open[:len(open)-1]
		// Only a materialized pseudo-element gives a void element children;
		// dom-serializer writes them after the tag, unclosed.
		if f.foreign && n.FirstChild == nil || !f.foreign && voidElements[n.Data] {
			return
		}
		buf.WriteString("</")
		buf.WriteString(f.name)
		buf.WriteByte('>')
	})
}

func writeAttrValue(buf *bytes.Buffer, v string) {
	for {
		i := strings.IndexByte(v, '"')
		if i < 0 {
			buf.WriteString(v)
			return
		}
		buf.WriteString(v[:i])
		buf.WriteString("&quot;")
		v = v[i+1:]
	}
}

// RenderXML serializes as dom-serializer does in XML mode: an element without
// children closes itself, and an empty attribute keeps its ="".
func RenderXML(buf *bytes.Buffer, root *html.Node) {
	Traverse(root, func(n *html.Node) bool {
		switch n.Type {
		case html.DocumentNode:
			return true
		case html.ElementNode:
		default:
			Render(buf, n)
			return false
		}
		buf.WriteByte('<')
		buf.WriteString(n.Data)
		for _, a := range jsKeyOrder(n.Attr) {
			buf.WriteByte(' ')
			buf.WriteString(a.Key)
			buf.WriteString(`="`)
			writeAttrValue(buf, a.Val)
			buf.WriteByte('"')
		}
		if n.FirstChild == nil {
			buf.WriteString("/>")
			return false
		}
		buf.WriteByte('>')
		return true
	}, func(n *html.Node) {
		if n.Type == html.ElementNode && n.FirstChild != nil {
			buf.WriteString("</")
			buf.WriteString(n.Data)
			buf.WriteByte('>')
		}
	})
}

// jsKeyOrder orders attributes the way juice emits them from a JavaScript
// object: names that are array indices first, ascending, then the rest in
// insertion order.
func jsKeyOrder(attrs []html.Attribute) []html.Attribute {
	if !slices.ContainsFunc(attrs, func(a html.Attribute) bool { _, ok := arrayIndex(a.Key); return ok }) {
		return attrs
	}
	out := slices.Clone(attrs)
	slices.SortStableFunc(out, func(a, b html.Attribute) int {
		x, xok := arrayIndex(a.Key)
		y, yok := arrayIndex(b.Key)
		switch {
		case xok && yok:
			return cmp.Compare(x, y)
		case xok:
			return -1
		case yok:
			return 1
		}
		return 0
	})
	return out
}

// arrayIndex reports whether s is a canonical JavaScript array index.
func arrayIndex(s string) (uint64, bool) {
	if s == "" || len(s) > 10 || s[0] < '0' || s[0] > '9' || (s[0] == '0' && s != "0") {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	return v, err == nil && v <= 1<<32-2
}

// Traverse visits root and its descendants in document order, calling enter
// on the way down and leave on the way back up, once each per node. enter
// reports whether to visit the node's children. It keeps no stack, so an
// arbitrarily deep tree, which unclosed tags readily produce, cannot overflow
// the goroutine's.
func Traverse(root *html.Node, enter func(*html.Node) bool, leave func(*html.Node)) {
	n := root
	for {
		if enter(n) && n.FirstChild != nil {
			n = n.FirstChild
			continue
		}
		leave(n)
		for n != root && n.NextSibling == nil {
			n = n.Parent
			leave(n)
		}
		if n == root {
			return
		}
		n = n.NextSibling
	}
}
