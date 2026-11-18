// Package juicer inlines CSS into HTML email, following the JavaScript
// library juice (https://github.com/Automattic/juice) v12.
//
// Output matches juice's byte for byte, except where juice disagrees with
// how browsers read the CSS: there the default follows the browser, and
// CompatibilityMode(Juice12) keeps juice's behaviour. CompatibilityMode(Juice9)
// matches juice 9 instead.
//
// The HTML parser and serializer deliberately do not follow the HTML5 tree
// construction spec: juice parses with htmlparser2, and matching its output
// means matching that tree.
package juicer

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Notifuse/safe-mjml/internal/cascadia"
	"github.com/Notifuse/safe-mjml/internal/juicer/htmlparser2"
	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
	"golang.org/x/net/html"
)

// Inliner holds resolved options and is safe for concurrent use. Reuse one
// across documents rather than constructing per call.
type Inliner struct {
	opts options
}

type options struct {
	extraCSS                    string
	insertExtraCSS              bool
	insertExtraCSSInto          string
	insertInto                  cascadia.Matcher // insertExtraCSSInto, compiled by New
	applyStyleTags              bool
	removeStyleTags             bool
	removeInlined               bool
	preserveMediaQueries        bool
	preserveFontFaces           bool
	preserveKeyFrames           bool
	preserveContainerQueries    bool
	preserveLayers              bool
	preservePseudos             bool
	inlineDuplicates            bool
	preservedSelectors          []string
	preserveImportant           bool
	applyWidthAttributes        bool
	applyHeightAttributes       bool
	applyAttributesTableElement bool
	resolveCSSVariables         bool
	inlinePseudoElements        bool
	styleAttributeName          string
	xmlMode                     bool
	codeBlocks                  []CodeBlock
	elementHook                 func(Element) []Attribute

	// keepAll preserves every top-level rule and at-rule; it formats what a
	// juice ignore comment keeps.
	keepAll bool

	compat Compatibility
	quirks // resolved from compat in New
	// juice10: the Juice10 profile, which is Juice9's with CSS variables
	// resolved and a declaration without a name throwing.
	juice10 bool
	// juice11: the Juice11 profile, Juice10's with juice 11's ignore
	// comments, empty rules and textarea.
	juice11 bool

	maxInputSize, maxDepth int // see limits.go
	maxStylesheetDepth     int
	maxMatchSteps          int64
	maxOutputSize          int // SetAttributes only
	maxSelectorBytes       int
	minifiedVoids          bool // SetAttributes only

	// budget is the work left to one Inline call for matching selectors
	// (MaxMatchSteps): process sets it on its own copy of the options.
	budget *cascadia.Budget
	// selectorBytes counts the bytes of selector one call compiled
	// (MaxSelectorBytes), on the same copy of the options.
	selectorBytes *int
}

// defaults mirror juice's getDefaultOptions. Note that preserveImportant and
// inlinePseudoElements are absent from that table and so default to false:
// !important is stripped from the emitted value even though it still wins the
// cascade.
func defaults() options {
	return options{
		insertExtraCSS:              true,
		applyStyleTags:              true,
		removeStyleTags:             true,
		preserveMediaQueries:        true,
		preserveFontFaces:           true,
		preserveKeyFrames:           true,
		preserveContainerQueries:    true,
		preserveLayers:              true,
		preservePseudos:             true,
		applyWidthAttributes:        true,
		applyHeightAttributes:       true,
		applyAttributesTableElement: true,
		resolveCSSVariables:         true,
		styleAttributeName:          "style",
		codeBlocks:                  DefaultCodeBlocks(),
		maxInputSize:                defaultMaxInputSize,
		maxDepth:                    defaultMaxDepth,
	}
}

// Option configures an Inliner. Defaults are juice's: ApplyStyleTags,
// RemoveStyleTags, InsertPreservedExtraCSS, ResolveCSSVariables, the Apply*
// attribute options and the Preserve* options other than PreserveImportant
// are on; every other boolean option is off.
type Option func(*options)

// ExtraCSS is appended to the CSS collected from the document.
func ExtraCSS(css string) Option { return func(o *options) { o.extraCSS = css } }

// InsertPreservedExtraCSS controls whether rules from ExtraCSS that cannot be
// inlined, such as @media blocks, are added to the document in a new <style>
// element, appended to <head>, else <body>, else the document. On by default.
func InsertPreservedExtraCSS(v bool) Option {
	return func(o *options) { o.insertExtraCSS, o.insertExtraCSSInto = v, "" }
}

// InsertPreservedExtraCSSInto appends that <style> element to the first
// element matching selector instead. If nothing matches, it is dropped.
func InsertPreservedExtraCSSInto(selector string) Option {
	return func(o *options) { o.insertExtraCSS, o.insertExtraCSSInto = true, selector }
}

// ApplyStyleTags controls whether CSS is collected from <style> elements.
func ApplyStyleTags(v bool) Option { return func(o *options) { o.applyStyleTags = v } }

// RemoveStyleTags controls whether <style> elements are removed after
// inlining. Rules that cannot be inlined are kept regardless.
func RemoveStyleTags(v bool) Option { return func(o *options) { o.removeStyleTags = v } }

// PreserveImportant keeps the literal !important suffix on inlined values.
func PreserveImportant(v bool) Option { return func(o *options) { o.preserveImportant = v } }

// InlinePseudoElements materializes ::before and ::after as real elements.
func InlinePseudoElements(v bool) Option { return func(o *options) { o.inlinePseudoElements = v } }

// StyleAttributeName writes inlined declarations to an attribute other than style.
func StyleAttributeName(name string) Option {
	return func(o *options) { o.styleAttributeName = name }
}

// CodeBlocks replaces the template delimiters protected from the HTML parser.
// The default set covers Liquid, Handlebars and EJS. juice adds to its default
// set by mutating the global juice.codeBlocks; the Go equivalent is
// appending to DefaultCodeBlocks().
func CodeBlocks(b []CodeBlock) Option { return func(o *options) { o.codeBlocks = slices.Clone(b) } }

// New returns an Inliner configured by opts, or an error wrapping
// ErrInvalidOption if they cannot be honoured.
func New(opts ...Option) (*Inliner, error) {
	o := defaults()
	for _, fn := range opts {
		fn(&o)
	}
	if err := o.validate(); err != nil {
		return nil, err
	}
	resolve := o.resolveCSSVariables
	switch o.compat {
	case Juice10:
		o.compat, o.juice10 = Juice9, true
	case Juice11:
		o.compat, o.juice10, o.juice11 = Juice9, true, true
	}
	o.quirks = o.compat.quirks()
	if o.compat == Juice9 {
		// juice 9 has none of these; it ignores the options.
		o.resolveCSSVariables = false
		o.preserveContainerQueries, o.preserveLayers = false, false
		o.preservedSelectors = nil
		o.removeInlined, o.inlineDuplicates = false, false
	}
	if o.juice10 {
		// juice 10 and 11 are juice 9 with CSS variables resolved, unless
		// the option turns it off.
		o.resolveCSSVariables = resolve
	}
	return &Inliner{opts: o}, nil
}

// validate rejects options that would fail on every document, or never
// finish, and compiles what can be compiled once.
func (o *options) validate() error {
	switch o.compat {
	case Default, Juice12, Juice9, Juice10, Juice11:
	default:
		return fmt.Errorf("%w: unknown Compatibility %d", ErrInvalidOption, o.compat)
	}
	for _, b := range o.codeBlocks {
		if b.Start == "" || b.End == "" {
			return fmt.Errorf("%w: CodeBlocks: delimiters cannot be empty", ErrInvalidOption)
		}
	}
	if o.styleAttributeName == "" {
		return fmt.Errorf("%w: StyleAttributeName cannot be empty", ErrInvalidOption)
	}
	if o.maxInputSize < 0 || o.maxDepth < 0 || o.maxStylesheetDepth < 0 || o.maxMatchSteps < 0 || o.maxSelectorBytes < 0 {
		return fmt.Errorf("%w: MaxInputSize, MaxDepth, MaxStylesheetDepth, MaxMatchSteps and MaxSelectorBytes cannot be negative", ErrInvalidOption)
	}
	if o.insertExtraCSSInto != "" {
		m, err := cascadia.Compile(o.insertExtraCSSInto)
		if err != nil {
			return fmt.Errorf("%w: InsertPreservedExtraCSSInto: %v", ErrInvalidOption, err)
		}
		o.insertInto = m
	}
	return nil
}

// Compatibility names an inliner whose output CompatibilityMode reproduces
// byte for byte, its bugs included.
type Compatibility int

const (
	// Default follows juice 12 except where juice disagrees with how
	// browsers read the CSS. It is what an Inliner uses without
	// CompatibilityMode.
	Default Compatibility = iota

	// Juice12 is juice 12.1.3, keeping juice's behaviour where the default
	// follows the browser instead.
	Juice12

	// Juice9 is juice 9.1.0. juice 9 resolves no CSS variables, matches no
	// :is() or :where(), and has none of juice 12's newer options or control
	// comments and attributes; those options are ignored as juice 9 ignores
	// them. Nested CSS is not flattened, and a nested rule that juice 9
	// throws on makes Inline return an error wrapping ErrJuiceFails.
	Juice9

	// Juice10 is juice 10.0.1, the version MJML 4.18 ran: juice 9 with
	// CSS variables resolved (ResolveCSSVariables, on by default), its only
	// change. Resolving reads the name of every property of an element it
	// styles, so a declaration without one (a word before a semicolon, as
	// in "font-family: &quot;A&quot;") makes Inline return an error
	// wrapping ErrJuiceFails, where juice 9 writes it as "undefined".
	Juice10

	// Juice11 is juice 11.1.1, the version MJML 5.4.1 runs: Juice10 with
	// juice 11's ignore comments in stylesheets ("juice ignore" first,
	// "juice start ignore" to "juice end ignore", "juice ignore next" before
	// a rule, a comment or a declaration, the last in style attributes too),
	// a rule without declarations no longer rewriting the style attribute
	// of the elements it matches, a style attribute read from the first of
	// its rules that keeps a declaration (after a "}" in it, a later one),
	// and the content of a textarea read as raw text (htmlparser2 9). Two
	// differences remain: a style attribute that keeps no declaration, on
	// an element a rule matches, makes juice 11 throw, and is read as
	// juice 10 reads it, ignore comments included (Pierre's decision,
	// 2026-11-17); and the rules preserved in <style> follow juice 10, as
	// do the options juice 11 added, which are ignored. MJML preserves
	// none and sets none of them.
	Juice11
)

// CompatibilityMode reproduces another inliner's output exactly, for output
// that has to stay identical to an existing deployment of it.
func CompatibilityMode(c Compatibility) Option { return func(o *options) { o.compat = c } }

// quirks are the juice behaviours a compatibility mode reproduces. The zero
// value is the default mode.
type quirks struct {
	// juice bugs the default fixes, listed in testdata/skip/default.txt.
	juiceSpecificity     bool // pseudo-classes score as type selectors
	joinStylesheets      bool // every <style> block and extraCss parse as one stylesheet
	ignoreStyleMedia     bool // a <style media> block applies whatever its media
	keepCommentMarkers   bool // "<!--" and "-->" are read as part of a selector
	rawContent           bool // content strings are written as markup
	contentFails         bool // content juice cannot read fails the document
	unstyledMatch        bool // <style> elements are removed before selectors match
	applyInvalid         bool // rules browsers reject, and strings left open, are read as postcss reads them
	juiceScope           bool // :scope, a top-level & included, matches nothing
	dropSupports         bool // @supports blocks are neither inlined nor preserved
	dropExtraPseudos     bool // extraCss rules for :hover and friends are not preserved
	loseIgnored          bool // a rule ignored from an earlier block, or by an unended "start ignore", is lost
	dropContent          bool // content is dropped from every element
	promoteInvalidColors bool // a color browsers drop still becomes a bgcolor
	spacedVar            bool // "var (--x)", with a space, is resolved
	headVars             bool // with duplicates kept, var() reads the first custom property set

	// juice 9.
	mensch          bool // mensch reads stylesheets and style attributes
	juice9Selectors bool // selectors are matched and scored as juice 9 does
	noControlAttrs  bool // data-juice-* attributes are not read
}

func (c Compatibility) quirks() quirks {
	juice12 := quirks{
		juiceSpecificity:     true,
		joinStylesheets:      true,
		ignoreStyleMedia:     true,
		keepCommentMarkers:   true,
		rawContent:           true,
		contentFails:         true,
		unstyledMatch:        true,
		applyInvalid:         true,
		juiceScope:           true,
		dropSupports:         true,
		dropExtraPseudos:     true,
		loseIgnored:          true,
		dropContent:          true,
		promoteInvalidColors: true,
		spacedVar:            true,
		headVars:             true,
	}
	switch c {
	case Juice12:
		return juice12
	case Juice9:
		q := juice12
		q.mensch, q.juice9Selectors, q.noControlAttrs = true, true, true
		return q
	}
	return quirks{}
}

// Inline inlines the document's CSS into style attributes.
func (in *Inliner) Inline(doc string) (string, error) {
	out, err := in.InlineBytes([]byte(doc))
	return string(out), err
}

// InlineBytes is Inline over a byte slice.
func (in *Inliner) InlineBytes(doc []byte) ([]byte, error) { return in.inline(doc, nil) }

// InlineCSS inlines css into doc as juice's inlineDocument does: the
// document's <style> elements are neither read nor changed, and rules from
// css that cannot be inlined are dropped rather than preserved.
func (in *Inliner) InlineCSS(doc, css string) (string, error) {
	out, err := in.inline([]byte(doc), &css)
	return string(out), err
}

// inline inlines doc's own CSS, or css instead when it is not nil.
func (in *Inliner) inline(doc []byte, css *string) ([]byte, error) {
	if err := checkSize(doc, &in.opts); err != nil {
		return nil, err
	}
	src, blocks := encodeCodeBlocks(doc, in.opts.codeBlocks)
	root, err := parseDocument(src, &in.opts)
	if err != nil {
		return nil, err
	}
	budget, err := in.process(root, src, css)
	if err != nil {
		if len(blocks) > 0 {
			// The message may quote a style attribute or a value: it shows
			// the template tags, not their placeholders.
			msg, _ := decodeCodeBlocks([]byte(err.Error()), blocks, nil)
			err = &decodedError{msg: string(msg), err: err}
		}
		return nil, err
	}
	var buf bytes.Buffer
	if in.opts.xmlMode {
		htmlparser2.RenderXML(&buf, root)
	} else {
		htmlparser2.Render(&buf, root)
	}
	out, ok := decodeCodeBlocks(buf.Bytes(), blocks, budget)
	if !ok {
		return nil, fmt.Errorf("%w: writing the template tags back takes more than %d steps", ErrLimitExceeded, in.opts.maxMatchSteps)
	}
	return out, nil
}

// Inline inlines doc's CSS using a default Inliner.
func Inline(doc string, opts ...Option) (string, error) {
	in, err := New(opts...)
	if err != nil {
		return "", err
	}
	return in.Inline(doc)
}

// resolvedEl pairs an element with the declarations resolved for it. The
// property map has to outlive writing the style attribute, because attribute
// promotion reads resolved values rather than reparsing the attribute.
type resolvedEl struct {
	node  *html.Node
	props *propMap
}

// pass holds the state for inlining one document. Inliner itself stays
// immutable so it can be shared across goroutines.
type pass struct {
	o        *options
	resolved []resolvedEl
	byNode   map[*html.Node]*propMap // ancestor lookup for var() resolution
	maps     []propMap               // slab newPropMap hands out from
	props    []property              // slab of slots newPropMap carves from

	css          *string      // InlineCSS's stylesheet, read instead of the document's
	controlAttrs bool         // the document mentions data-juice-* attributes
	juiceErr     error        // the first place juice would fail, wrapping ErrJuiceFails
	joinedCSS    [][]byte     // the <style> blocks applied, in a compatibility mode
	removed      []*html.Node // <style> elements to remove once rules have matched

	// RemoveInlinedSelectors only.
	inlined  map[string]bool     // selectors that matched a visible element
	embedded map[*html.Node]bool // data-embed <style> elements, left alone

	// InlinePseudoElements only.
	pseudos     map[*html.Node]*pseudoMaps
	pseudoOrder []*html.Node
	matches     []matchEvent // recorded only when content reads a counter
}

// process runs the inlining pipeline over an already-parsed tree. It
// returns the budget left, nil when there is no limit.
func (in *Inliner) process(root *html.Node, src []byte, css *string) (*cascadia.Budget, error) {
	if err := checkDepth(root, &in.opts); err != nil {
		return nil, err
	}
	o := &in.opts
	if o.maxMatchSteps > 0 || o.maxSelectorBytes > 0 {
		// The Inliner is shared: the budget goes on a copy of its options.
		own := *o
		if o.maxMatchSteps > 0 {
			own.budget = cascadia.NewBudget(o.maxMatchSteps)
		}
		own.selectorBytes = new(int)
		o = &own
	}
	p := &pass{o: o, css: css, controlAttrs: !in.opts.noControlAttrs && bytes.Contains(src, []byte("data-juice-"))}
	return o.budget, p.run(root)
}

// appendRules adds one stylesheet's rules, keeping their groups distinct from
// those of earlier stylesheets.
func appendRules(dst, src []rule) []rule {
	off := int32(len(dst))
	for i := range src {
		src[i].group += off
	}
	return append(dst, src...)
}

func (in *pass) run(root *html.Node) error {
	o := in.o

	if in.css != nil {
		return in.inlineRules(root, in.parseOnly(*in.css), nil)
	}
	if o.removeInlined && !o.removeStyleTags {
		in.inlined = map[string]bool{}
		in.embedded = map[*html.Node]bool{}
	}
	var st ignoreState
	rules := in.collectCSS(root, &st)
	var keep []preserved
	if extra := strings.TrimSpace(o.extraCSS); extra != "" {
		eo := *o
		eo.preservePseudos = o.preservePseudos && !o.dropExtraPseudos
		r, k, _ := parseWithDirectives([]byte(extra), &eo, uint32(len(rules))<<8, &st)
		rules = appendRules(rules, r)
		keep = append(keep, k...)
	}
	if o.joinStylesheets {
		// juice inlines every <style> block and extraCss as one stylesheet,
		// so an unclosed block runs on into the next. Browsers read each
		// block on its own, as the default does.
		css := append(bytes.Join(in.joinedCSS, []byte("\n")), "\n"+o.extraCSS...)
		if o.selectorBytes != nil && !o.selectorsExceeded() {
			// The same selectors, read again.
			*o.selectorBytes = 0
		}
		rules, _, _ = parseWithDirectives(css, o, 0, &ignoreState{})
	}
	return in.inlineRules(root, rules, keep)
}

// parseOnly reads css for InlineCSS.
func (in *pass) parseOnly(css string) []rule {
	rules, _, _ := parseWithDirectives([]byte(css), in.o, 0, &ignoreState{})
	return rules
}

// inlineRules applies rules to the document and writes the result, keeping
// what could not be inlined.
func (in *pass) inlineRules(root *html.Node, rules []rule, keep []preserved) error {
	o := in.o
	if o.selectorsExceeded() {
		return o.tooManySelectorBytes()
	}
	in.applyRules(root, rules)
	if o.budget.Exceeded() {
		return fmt.Errorf("%w: matching the selectors to the document takes more than %d steps", ErrLimitExceeded, o.maxMatchSteps)
	}
	for _, s := range in.removed {
		detach(s)
	}
	if in.juiceErr != nil {
		return in.juiceErr
	}
	if in.controlAttrs {
		// juice's per-element control attributes never reach the output.
		walk(root, func(n *html.Node) {
			removeAttr(n, "data-juice-important")
			removeAttr(n, "data-juice-duplicates")
		})
	}
	imgs, err := in.materializePseudos(rules)
	if err != nil {
		return err
	}
	if err := in.unnamedSurvivors(); err != nil {
		return err
	}
	if o.resolveCSSVariables {
		in.resolveVariables()
		if o.budget.Exceeded() {
			return fmt.Errorf("%w: resolving the CSS variables of the document takes more than %d steps", ErrLimitExceeded, o.maxMatchSteps)
		}
	}
	in.writeStyles()
	for _, img := range imgs {
		img.apply()
	}
	in.promoteAttributes(root)
	if o.elementHook != nil {
		in.runElementHook(root)
	}
	if err := in.emitPreserved(root, keep); err != nil {
		return err
	}
	if in.inlined != nil {
		in.removeInlinedSelectors(root)
	}
	return nil
}

// collectCSS gathers CSS from <style> elements and disposes of them. Each tag
// is handled on its own, because juice computes the text to preserve per tag
// and replaces that tag's contents with it.
func (in *pass) collectCSS(root *html.Node, st *ignoreState) []rule {
	o := in.o
	var rules []rule
	var ord uint32

	var styles []*html.Node
	walk(root, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" {
			styles = append(styles, n)
		}
	})

	for _, s := range styles {
		// juice ignores a <style> that is not exactly one text node.
		if s.FirstChild == nil || s.FirstChild != s.LastChild || s.FirstChild.Type != html.TextNode {
			if o.removeStyleTags {
				in.remove(s)
			}
			continue
		}
		if _, embedded := getAttr(s, "data-embed"); embedded {
			if in.embedded != nil {
				in.embedded[s] = true
			}
			removeAttr(s, "data-embed")
			continue
		}
		if v, ok := getAttr(s, "media"); ok && !o.ignoreStyleMedia && !screenMedia(v) {
			// juice ignores media; browsers apply these rules only where
			// the query matches, so they stay as written, or go with
			// @media blocks when those are not preserved.
			if o.removeStyleTags && !o.preserveMediaQueries {
				in.remove(s)
			} else if in.embedded != nil {
				in.embedded[s] = true
			}
			continue
		}

		var r []rule
		var k []preserved
		if o.applyStyleTags {
			r, k, ord = parseWithDirectives([]byte(s.FirstChild.Data), o, ord, st)
			rules = appendRules(rules, r)
			if o.joinStylesheets {
				in.joinedCSS = append(in.joinedCSS, []byte(s.FirstChild.Data))
			}
		}

		if !o.removeStyleTags {
			continue
		}
		if len(k) == 0 {
			in.remove(s)
			continue
		}
		// Rules that could not be inlined stay behind in this tag.
		s.FirstChild.Data = string(preservedText(k))
	}
	return rules
}

// remove drops a <style> element once rules have matched, as it is part of
// the tree selectors such as :first-child see. One a browser would move into
// <head> goes at once.
func (in *pass) remove(s *html.Node) {
	if in.o.unstyledMatch || hoisted(s) {
		detach(s)
		return
	}
	in.removed = append(in.removed, s)
}

// headContent is what browsers place in <head> when it comes before the body.
var headContent = map[string]bool{
	"head": true, "title": true, "base": true, "link": true, "meta": true,
	"style": true, "script": true, "noscript": true, "template": true,
}

// hoisted reports whether n sits outside <body> with nothing but head
// content before it, where browsers parse it into <head>.
func hoisted(n *html.Node) bool {
	if p := n.Parent; p.Type != html.DocumentNode && !(p.Type == html.ElementNode && p.Data == "html") {
		return false
	}
	for c := n.PrevSibling; c != nil; c = c.PrevSibling {
		switch {
		case c.Type == html.CommentNode || c.Type == html.DoctypeNode:
		case c.Type == html.TextNode && strings.TrimLeft(c.Data, " \t\n\r\f") == "":
		case c.Type == html.ElementNode && headContent[c.Data]:
		default:
			return false
		}
	}
	return true
}

// applyRules walks the document once in order, applying every matching rule to
// each element. Rules are tested in stylesheet order so that the
// different-selector override moves a property to the end exactly as juice's
// delete-and-reinsert does.
func (in *pass) applyRules(root *html.Node, rules []rule) {
	o := in.o
	track := o.inlinePseudoElements && needCounters(rules)
	if foldRepeated && !o.inlineDuplicates && !in.controlAttrs && !track {
		rules = mergeRepeatedSelectors(rules)
	}
	rs := newRuleSet(rules, root, o.juice9Selectors)
	rs.budget = o.budget
	var sc scratch
	var doc int32
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || nonVisualElements[n.Data] {
			return
		}
		var m *propMap
		applied := int32(-1)
		doc++
		keep := -1 // whether n keeps !important; read on first match
		dup := func() bool {
			if o.noControlAttrs {
				return false
			}
			if v, ok := getAttr(n, "data-juice-duplicates"); ok {
				return v != "false"
			}
			return o.inlineDuplicates
		}
		keepImportant := func() bool {
			if keep < 0 {
				keep = 0
				if v, ok := getAttr(n, "data-juice-important"); ok && !o.noControlAttrs && v != "false" || (!ok || o.noControlAttrs) && o.preserveImportant {
					keep = 1
				}
			}
			return keep == 1
		}
		rs.forEach(n, &sc, func(i int, r *rule) {
			// One selector applies once, however many of its :is()
			// expansions match.
			if r.group == applied {
				return
			}
			applied = r.group
			if r.throws && in.juiceErr == nil {
				in.juiceErr = fmt.Errorf("%w: juice cannot read a declaration of the rule %q", ErrJuiceFails, r.sel)
			}
			if in.inlined != nil {
				in.inlined[r.sel] = true
			}
			if track {
				in.matches = append(in.matches, matchEvent{r.group, doc, n})
			}
			if r.pseudo != selector.None {
				// Declarations for ::before/::after belong to their own
				// element; without materialization they are dropped. The
				// base element is not touched either way.
				if o.inlinePseudoElements {
					addDecls(in.pseudoMap(n, r.pseudo, dup), r, i, keepImportant())
				}
				return
			}
			if m == nil {
				m = in.newPropMap(dup(), len(r.decls))
				// The existing style attribute seeds the map on first match.
				// An element no rule touches is never visited, so its style
				// attribute is left exactly as written.
				if v, ok := getAttr(n, o.styleAttributeName); ok {
					if err := m.seedInline([]byte(v), keepImportant(), o.mensch, o.juice10, o.juice11); err != nil && in.juiceErr == nil {
						in.juiceErr = err
					}
					o.budget.Spend(declarationSteps*len(m.slots) + valueSteps(len(v)))
				}
			}
			// The declarations an element gets are held until the document
			// is written: they cost steps, which bounds that memory and the
			// output, rules times elements times declarations.
			if !o.budget.Spend(declarationSteps*len(r.decls) + valueSteps(r.valueBytes)) {
				return
			}
			addDecls(m, r, i, keepImportant())
		})
		if m == nil {
			return
		}
		m.heads = nil // the index serves the rules being added, now done
		in.resolved = append(in.resolved, resolvedEl{n, m})
		if in.byNode == nil {
			in.byNode = make(map[*html.Node]*propMap)
		}
		in.byNode[n] = m
	})
}

// newPropMap returns an element's map from slabs that double up to 256, with
// room for n declarations carved from a slab of slots. The carve's capacity is
// capped, so a map that outgrows it reallocates rather than overwriting the next.
func (in *pass) newPropMap(dup bool, n int) *propMap {
	if len(in.maps) == cap(in.maps) {
		in.maps = make([]propMap, 0, min(max(2*cap(in.maps), 4), 256))
	}
	n++ // room for a later rule without reallocating
	if cap(in.props)-len(in.props) < n {
		in.props = make([]property, 0, max(n, min(max(2*cap(in.props), 8), 1024)))
	}
	k := len(in.props)
	in.props = in.props[:k+n]
	in.maps = append(in.maps, propMap{dup: dup, slots: in.props[k : k : k+n]})
	return &in.maps[len(in.maps)-1]
}

func addDecls(m *propMap, r *rule, i int, keepImportant bool) {
	for _, d := range r.decls {
		if d.prop == "" || len(d.value) == 0 && !d.keepEmpty {
			continue
		}
		prio := 0
		v := d.value
		if d.important {
			prio = 2
			if keepImportant && d.bang != nil {
				v = d.bang
			}
		}
		at := int32(i)
		if d.from > 0 {
			at = d.from - 1
		}
		m.add(property{
			prop:    d.prop,
			value:   v,
			key:     packKey(prio, r.spec[0], r.spec[1], r.spec[2]),
			ord:     d.ord,
			rule:    int32(i),
			at:      at,
			unnamed: d.unnamed,
			bare:    d.bare,
		})
	}
}

// writeStyles serializes each element's resolved declarations. It runs after
// accumulation because attribute promotion reads the same property map, and
// after variable resolution so the promoted values are the substituted ones.
func (in *pass) writeStyles() {
	var s styleScratch
	var slab []html.Attribute // grown attribute lists, carved with capped capacity
	for _, el := range in.resolved {
		if v := el.props.styleAttr(in.o, &s); v != nil {
			n := el.node
			if _, ok := getAttr(n, in.o.styleAttributeName); !ok && len(n.Attr) == cap(n.Attr) {
				need := len(n.Attr) + 1
				if cap(slab)-len(slab) < need {
					slab = make([]html.Attribute, 0, max(need, min(max(2*cap(slab), 8), 256)))
				}
				k := len(slab)
				slab = append(slab, n.Attr...)
				n.Attr = slab[k : len(slab) : k+need]
				slab = slab[:k+need]
			}
			setAttr(n, in.o.styleAttributeName, string(v))
		}
	}
}

// screenMedia reports whether a media attribute applies to every screen: it
// is empty, or one of its queries is a bare all or screen.
func screenMedia(media string) bool {
	if strings.TrimSpace(media) == "" {
		return true
	}
	for q := range strings.SplitSeq(media, ",") {
		q = strings.ToLower(strings.TrimSpace(q))
		q = strings.TrimSpace(strings.TrimPrefix(q, "only "))
		if q == "all" || q == "screen" {
			return true
		}
	}
	return false
}

func preservedText(keep []preserved) []byte {
	var b bytes.Buffer
	b.WriteByte('\n')
	for _, k := range keep {
		b.Write(k.text)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// emitPreserved appends a <style> holding rules that could not be inlined,
// when no surviving <style> already carries them.
func (in *pass) emitPreserved(root *html.Node, keep []preserved) error {
	if len(keep) == 0 || !in.o.insertExtraCSS {
		return nil
	}
	if m := in.o.insertInto; m != nil {
		if host := cascadia.Query(root, m); host != nil {
			in.appendPreserved(host, keep)
		}
		return nil
	}
	host := findFirst(root, "head")
	if host == nil {
		host = findFirst(root, "body")
	}
	if host == nil {
		host = root
	}
	in.appendPreserved(host, keep)
	return nil
}

// removeInlinedSelectors is juice's updateStyleTags: every <style> left in the
// document, the one added for extraCss included, loses the selectors that
// were inlined.
func (in *pass) removeInlinedSelectors(root *html.Node) {
	var styles []*html.Node
	walk(root, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" && !in.embedded[n] {
			styles = append(styles, n)
		}
	})
	keepAll := *in.o
	keepAll.keepAll = true
	for _, s := range styles {
		if s.FirstChild == nil || s.FirstChild != s.LastChild || s.FirstChild.Type != html.TextNode {
			continue
		}
		sheet := readStylesheet([]byte(s.FirstChild.Data), in.o)
		var parts []string
		for _, n := range sheet.root.Nodes {
			if n.Type == "rule" {
				arms := n.Selectors()
				keepWhole := in.o.preservePseudos && selector.IgnoredPseudo(arms[0]) ||
					slices.ContainsFunc(arms, func(a string) bool { return matchesPreserved([][]byte{[]byte(a)}, in.o.preservedSelectors) })
				if !keepWhole {
					left := slices.DeleteFunc(arms, func(a string) bool { return in.inlined[a] })
					if len(left) == 0 {
						continue
					}
					// postcss rejoins the list with its first separator.
					sep := listSeparator().FindString(n.Selector)
					n = n.Copy()
					n.SetSelector(strings.Join(left, sep))
				}
			}
			if t := sheet.format(n, &keepAll); t != "" {
				parts = append(parts, t)
			}
		}
		if text := strings.Join(parts, "\n"); strings.TrimSpace(text) != "" {
			s.FirstChild.Data = text
		} else {
			detach(s)
		}
	}
}

var listSeparator = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`,\s*`) })

// appendPreserved adds the <style> as juice does, by appending markup: in XML
// mode a "<" in the rules is parsed as a tag.
func (in *pass) appendPreserved(host *html.Node, keep []preserved) {
	if in.o.xmlMode {
		frag, _ := htmlparser2.Parse(slices.Concat([]byte("<style>"), preservedText(keep), []byte("</style>")), true, 0)
		for c := frag.FirstChild; c != nil; c = frag.FirstChild {
			frag.RemoveChild(c)
			host.AppendChild(c)
		}
		return
	}
	s := &html.Node{Type: html.ElementNode, Data: "style"}
	s.AppendChild(&html.Node{Type: html.TextNode, Data: string(preservedText(keep))})
	host.AppendChild(s)
}

// unnamedSurvivors returns the error juice 10 throws when an element it
// styles still holds its "undefined" property, a declaration without a name
// (menschDecls): one a later "undefined" property replaced is gone.
func (in *pass) unnamedSurvivors() error {
	var idx []int32
	for _, el := range in.resolved {
		idx = el.props.live(idx[:0])
		for _, i := range idx {
			if p := &el.props.slots[i]; p.unnamed {
				return fmt.Errorf("%w: juice cannot read the declaration %q without a property name", ErrJuiceFails, p.value)
			}
		}
	}
	return nil
}
