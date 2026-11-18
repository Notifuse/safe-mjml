package juicer

import (
	"bytes"
	"fmt"
	"math"
	"slices"

	"golang.org/x/net/html"

	"github.com/Notifuse/safe-mjml/internal/cascadia"
	"github.com/Notifuse/safe-mjml/internal/juicer/htmlparser2"
	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
)

// AttributeRule is a selector and the attributes to set, in order, on every
// element it matches: an mj-selector of mj-html-attributes.
type AttributeRule struct {
	Selector   string
	Attributes []html.Attribute
}

// ErrOutputTooLarge is the error of SetAttributes when the document it
// writes would be larger than MaxOutputSize.
var ErrOutputTooLarge = fmt.Errorf("%w: the document would be larger than MaxOutputSize", ErrLimitExceeded)

// MaxOutputSize limits the document SetAttributes writes, in bytes: past it,
// SetAttributes fails with ErrOutputTooLarge as soon as the attributes it
// sets would take it there. The default, 0, has no limit.
func MaxOutputSize(n int) Option { return func(o *options) { o.maxOutputSize = n } }

// MinifiedVoids makes SetAttributes write the document as html-minifier
// writes cheerio's: an element without children as its start tag alone
// (<br>, <div>), not closed (<br/>), and the end tag of a br as <br>
// (htmlparser2.RenderXMLMinified).
func MinifiedVoids() Option { return func(o *options) { o.minifiedVoids = true } }

// SetAttributes does what mjml-core does with the mj-html-attributes of a
// template:
//
//	each(htmlAttributes, (data, selector) => each(data, (value, name) =>
//	  $(selector).each(function () { $(this).attr(name, value || '') })))
//
// It reads doc as cheerio reads it with xmlMode and without decoding
// entities (htmlparser2's XML mode). For each attribute of each rule, in
// order, it selects again what the rule's selector matches, as cheerio's $()
// does (cheerio-select over css-select in XML mode: names and values keep
// their case, and the positional pseudo-classes :first, :eq()… select by
// position in the elements matched), so that an attribute just set changes
// what the next one selects, and sets the attribute on each, replacing the
// value of one the element already has. It writes the document back as
// cheerio does in XML mode: an element without children closes itself. A
// rule whose selector cheerio cannot read sets nothing and is returned in
// invalid (MJML throws on it), unless it has no attribute (MJML never reads
// it). Template tags are protected as Inline protects them (CodeBlocks).
//
// MaxMatchSteps bounds the work: each node a selection walks, each element a
// selector tests, and, for each attribute set, declarationSteps, the
// element's attributes it looks through and stepsPerValueByte for each byte
// of its name and value. MaxOutputSize bounds the document written.
func SetAttributes(doc string, rules []AttributeRule, opts ...Option) (out string, invalid []string, err error) {
	in, err := New(append(opts, CompatibilityMode(Juice11))...)
	if err != nil {
		return "", nil, err
	}
	o := in.opts
	o.xmlMode = true
	if err := checkSize([]byte(doc), &o); err != nil {
		return "", nil, err
	}
	src, blocks := encodeCodeBlocks([]byte(doc), o.codeBlocks)
	root, err := parseDocument(src, &o)
	if err != nil {
		return "", nil, err
	}
	if o.maxMatchSteps > 0 {
		o.budget = cascadia.NewBudget(o.maxMatchSteps)
	}
	o.selectorBytes = new(int)
	for _, r := range rules {
		if len(r.Attributes) > 0 && !o.spendSelector(len(r.Selector)) {
			return "", nil, o.tooManySelectorBytes()
		}
	}
	tooCostly := func() error {
		return fmt.Errorf("%w: matching the selectors of mj-html-attributes takes more than %d steps", ErrLimitExceeded, o.maxMatchSteps)
	}
	s := &cheerioSelection{root: root, budget: o.budget, compiled: map[tokensKey]compiledMatchers{}, derived: map[derivedKey][]selector.Token{}, ahead: map[*selector.Token]tokensAhead{}, args: map[*selector.Token]positionalArgument{}}
	size := len(doc)
	for _, r := range rules {
		if len(r.Attributes) == 0 {
			continue
		}
		q, ok := s.compile(r.Selector)
		// A compile cut short by the budget is no invalid selector.
		if o.budget.Exceeded() {
			return "", nil, tooCostly()
		}
		if !ok {
			invalid = append(invalid, r.Selector)
			continue
		}
		for _, a := range r.Attributes {
			nodes := s.selectAll(q)
			if o.budget.Exceeded() {
				return "", nil, tooCostly()
			}
			for _, n := range nodes {
				if !o.budget.Spend(declarationSteps + len(n.Attr) + valueSteps(len(a.Key)+len(a.Val))) {
					return "", nil, tooCostly()
				}
				size += setAttribute(n, a)
				if o.maxOutputSize > 0 && size > o.maxOutputSize {
					return "", nil, ErrOutputTooLarge
				}
			}
		}
	}
	var buf bytes.Buffer
	if o.minifiedVoids {
		htmlparser2.RenderXMLMinified(&buf, root)
	} else {
		htmlparser2.RenderXML(&buf, root)
	}
	result, ok := decodeCodeBlocks(buf.Bytes(), blocks, o.budget)
	if !ok {
		return "", nil, fmt.Errorf("%w: writing the template tags back takes more than %d steps", ErrLimitExceeded, o.maxMatchSteps)
	}
	if o.maxOutputSize > 0 && len(result) > o.maxOutputSize {
		return "", nil, ErrOutputTooLarge
	}
	return string(result), invalid, nil
}

// setAttribute sets a on n as cheerio's attr(name, value): in place when n
// has it, last otherwise. It returns how many bytes it adds to the document
// written.
func setAttribute(n *html.Node, a html.Attribute) int {
	for i := range n.Attr {
		if n.Attr[i].Key == a.Key {
			grow := writtenLen(a.Val) - writtenLen(n.Attr[i].Val)
			n.Attr[i].Val = a.Val
			return grow
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: a.Key, Val: a.Val})
	return len(` =""`) + len(a.Key) + writtenLen(a.Val)
}

// writtenLen is the length of an attribute value as RenderXML writes it, a
// quote as &quot;.
func writtenLen(v string) int {
	return len(v) + (len("&quot;")-1)*bytes.Count([]byte(v), []byte{'"'})
}

// cheerioSelection selects in a document as cheerio's $() does.
type cheerioSelection struct {
	root   *html.Node
	budget *cascadia.Budget
	// A selector is evaluated again for every attribute, and its parts for
	// every positional pseudo-class: what each evaluation derives from the
	// tokens is kept, by where the tokens are, so that an evaluation reads
	// a part once and compiles nothing.
	compiled map[tokensKey]compiledMatchers
	derived  map[derivedKey][]selector.Token
	ahead    map[*selector.Token]tokensAhead
	args     map[*selector.Token]positionalArgument
}

// positionalArgument is the number a positional pseudo-class is given, as
// parseInt reads its argument, and whether it is one.
type positionalArgument struct {
	num    int64
	finite bool
}

// argument is the number of t's argument, read once for a step per 16 bytes.
func (s *cheerioSelection) argument(t *selector.Token) (int64, bool) {
	if a, ok := s.args[t]; ok {
		return a.num, a.finite
	}
	arg := t.Argument()
	s.budget.Spend(1 + len(arg)/16)
	num, finite := parseIntSaturating(arg)
	s.args[t] = positionalArgument{num, finite}
	return num, finite
}

// tokensAhead is what follows a token in its list, itself included: how far
// the next positional pseudo-class is (-1 for none), and whether a
// combinator is there.
type tokensAhead struct {
	positional int
	combinator bool
}

// ahead returns what follows toks[0] in toks, read once for every token of
// a list, from its end: a chain of positional pseudo-classes is read in
// linear time, not once per pseudo-class.
func (s *cheerioSelection) aheadOf(toks []selector.Token) tokensAhead {
	if len(toks) == 0 {
		return tokensAhead{positional: -1}
	}
	if a, ok := s.ahead[&toks[0]]; ok {
		return a
	}
	next := tokensAhead{positional: -1}
	for i := len(toks) - 1; i >= 0; i-- {
		if a, ok := s.ahead[&toks[i]]; ok {
			next = a
			continue
		}
		cur := tokensAhead{positional: -1, combinator: next.combinator || toks[i].Combinator()}
		switch {
		case toks[i].Positional():
			cur.positional = 0
		case next.positional >= 0:
			cur.positional = next.positional + 1
		}
		s.ahead[&toks[i]] = cur
		next = cur
	}
	return next
}

// tokensKey names a list of tokens by where it is: tokens are not changed
// once parsed, and the lists derived from them are kept.
type tokensKey struct {
	first *selector.Token
	n     int
}

func keyOf(toks []selector.Token) tokensKey {
	if len(toks) == 0 {
		return tokensKey{}
	}
	return tokensKey{&toks[0], len(toks)}
}

// derivedKey names a list derived from tokens: how, and from which.
type derivedKey struct {
	from tokensKey
	how  int
}

const (
	derivedBeforeFilter = iota // beforeFilter
	derivedFromAny             // "*" and the tokens
	derivedSiblings            // matchFrom's "* ~ x" for "* + x"
	derivedMarked              // matchFrom's selector with the anchors' mark
)

// derive returns build(toks), built once for these tokens and how, for a
// step per token.
func (s *cheerioSelection) derive(toks []selector.Token, how int, build func([]selector.Token) []selector.Token) []selector.Token {
	k := derivedKey{keyOf(toks), how}
	if d, ok := s.derived[k]; ok {
		return d
	}
	s.budget.Spend(len(toks))
	d := build(toks)
	s.derived[k] = d
	return d
}

func fromAny(toks []selector.Token) []selector.Token {
	return append([]selector.Token{{Text: []byte("*"), Kind: selector.Universal}}, toks...)
}

type compiledMatchers struct {
	matchers []cascadia.Matcher
	ok       bool
}

// cheerioQuery is a selector list as cheerio-select splits it: the plain
// selectors, which css-select matches in one walk of the document, and the
// filtered ones, which hold a positional pseudo-class or start with a
// combinator, applied one by one.
type cheerioQuery struct {
	plain    []cascadia.Matcher
	filtered [][]selector.Token
}

// compile reads a selector list as cheerio does, or reports false when
// cheerio throws on it: css-what reads it (an empty selector in a list is an
// error, an empty list selects nothing), and css-select compiles each part
// of it.
func (s *cheerioSelection) compile(path string) (*cheerioQuery, bool) {
	arms, ok := splitList([]byte(path))
	if !ok {
		return nil, false
	}
	q := &cheerioQuery{}
	for _, arm := range arms {
		toks := selector.Parse(arm).Tokens
		if positionals(toks) > maxPositionals {
			return nil, false
		}
		if slices.ContainsFunc(toks, func(t selector.Token) bool { return t.Positional() }) || toks[0].Combinator() {
			if !s.validFiltered(toks) {
				return nil, false
			}
			q.filtered = append(q.filtered, toks)
			continue
		}
		matchers, ok := s.matchers(toks)
		if !ok {
			return nil, false
		}
		q.plain = append(q.plain, matchers...)
	}
	return q, true
}

// maxSelectorNesting bounds how deeply the argument lists of a selector of
// mj-html-attributes nest, as inline mj-style bounds its stylesheet:
// compiling and evaluating a selector costs more than linear time in its
// nesting, reading it recurses, and MJML's own stack overflows past a few
// thousand.
const maxSelectorNesting = 100

// maxPositionals bounds how many positional pseudo-classes a selector of
// mj-html-attributes holds, its arguments included: each is evaluated by a
// call of its own, nested in the one before it, and MJML's own stack
// overflows past a few thousand.
const maxPositionals = 100

// positionals counts the positional pseudo-classes of toks and of their
// arguments.
func positionals(toks []selector.Token) int {
	n := 0
	for i := range toks {
		if toks[i].Positional() {
			n++
		}
		for _, arg := range toks[i].Args {
			n += positionals(arg.Tokens)
		}
	}
	return n
}

// splitList splits a selector list at its top-level commas, or reports false
// when one of its selectors is empty, as css-what does ("Empty
// sub-selector"), or it nests deeper than maxSelectorNesting. A list with
// nothing but whitespace has no selector.
func splitList(sel []byte) ([][]byte, bool) {
	if len(bytes.TrimSpace(sel)) == 0 {
		return nil, true
	}
	parts, ok := selector.SplitList(sel, maxSelectorNesting)
	if !ok {
		return nil, false
	}
	for i, part := range parts {
		parts[i] = bytes.TrimSpace(part)
		if len(parts[i]) == 0 {
			return nil, false
		}
	}
	return parts, true
}

// text is the selector the tokens are, with a trailing combinator matching
// any element after it, as css-select reads "td >".
func text(toks []selector.Token) []byte {
	var b []byte
	for i := range toks {
		b = append(b, toks[i].Text...)
	}
	for k := len(toks) - 1; k >= 0; k-- {
		if !toks[k].Combinator() {
			break
		}
		if !isSpace(toks[k].Text) {
			return append(b, '*')
		}
	}
	return b
}

func isSpace(b []byte) bool { return len(bytes.TrimSpace(b)) == 0 }

// matchers compiles tokens without positional pseudo-classes as css-select
// does in XML mode, or reports false when it throws.
// Each is compiled once, for a step per byte of its text.
func (s *cheerioSelection) matchers(toks []selector.Token) ([]cascadia.Matcher, bool) {
	k := keyOf(toks)
	if c, ok := s.compiled[k]; ok && k.first != nil {
		return c.matchers, c.ok
	}
	sel := text(toks)
	if !s.budget.Spend(len(sel)) {
		return nil, false
	}
	c := compiledMatchers{}
	c.matchers, c.ok = s.compileMatchers(sel)
	s.compiled[k] = c
	return c.matchers, c.ok
}

func (s *cheerioSelection) compileMatchers(sel []byte) ([]cascadia.Matcher, bool) {
	// :scope is the document's root, which no element is (JuiceScope).
	c := selector.Compile(sel, selector.Mode{Cheerio: true, Lenient: true, JuiceScope: true})
	if c.Invalid {
		return nil, false
	}
	var out []cascadia.Matcher
	for _, v := range c.Variants {
		m, err := cascadia.ParseForCheerioXML(v.Text, s.budget)
		if err != nil {
			return nil, false
		}
		out = append(out, m)
	}
	return out, true
}

// validFiltered reports whether css-select compiles each part of a filtered
// selector, between its positional pseudo-classes, and the arguments of a
// :not() that holds one.
func (s *cheerioSelection) validFiltered(toks []selector.Token) bool {
	start := 0
	part := func(end int) bool {
		p := trimSpace(toks[start:end])
		if len(p) == 0 {
			return true
		}
		if p[0].Combinator() {
			p = fromAny(p)
		}
		_, ok := s.matchers(p)
		return ok
	}
	for k := range toks {
		t := &toks[k]
		if !t.Positional() {
			continue
		}
		if !part(k) {
			return false
		}
		start = k + 1
		if t.Name == "not" {
			for _, arg := range t.Args {
				if len(arg.Tokens) == 0 || !s.validArgument(arg.Tokens) {
					return false
				}
			}
		}
	}
	return part(len(toks))
}

// validArgument reports whether a selector of a :not() list compiles.
func (s *cheerioSelection) validArgument(toks []selector.Token) bool {
	toks = trimSpace(toks)
	if slices.ContainsFunc(toks, func(t selector.Token) bool { return t.Positional() }) {
		return s.validFiltered(toks)
	}
	_, ok := s.matchers(toks)
	return ok
}

// beforeFilter is the part of a selector before a positional pseudo-class,
// as css-select reads it: a combinator at its end, whitespace included
// ("td :first"), matches any element under or after what precedes it.
func beforeFilter(toks []selector.Token) []selector.Token {
	toks = trimSpace(append(toks[:len(toks):len(toks)], selector.Token{Text: []byte("*"), Kind: selector.Universal}))
	if n := len(toks); n == 1 || !toks[n-2].Combinator() {
		// No combinator before the pseudo-class: the universal selector
		// added is dropped.
		toks = toks[:n-1]
	}
	return toks
}

// blank reports whether t is whitespace: a combinator token is a run of
// whitespace or one character.
func blank(t *selector.Token) bool {
	return t.Combinator() && isCSSSpace(t.Text[0])
}

// trimSpace drops the whitespace tokens at either end.
func trimSpace(toks []selector.Token) []selector.Token {
	for len(toks) > 0 && blank(&toks[0]) {
		toks = toks[1:]
	}
	for len(toks) > 0 && blank(&toks[len(toks)-1]) {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// selectAll is cheerio-select's select: the elements the query selects, in
// document order, each once.
func (s *cheerioSelection) selectAll(q *cheerioQuery) []*html.Node {
	var found [][]*html.Node
	for _, toks := range q.filtered {
		found = append(found, s.findFilter(nil, toks, true, false, math.MaxInt))
	}
	if len(q.plain) > 0 {
		found = append(found, s.find(func(n *html.Node) bool {
			for _, m := range q.plain {
				if m.Match(n) {
					return true
				}
			}
			return false
		}))
	}
	switch len(found) {
	case 0:
		return nil
	case 1:
		return found[0]
	}
	return s.uniqueSort(found)
}

// find walks the document and returns the elements match accepts, in
// document order. Each node walked costs a step.
func (s *cheerioSelection) find(match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	htmlparser2.Traverse(s.root, func(n *html.Node) bool {
		if !s.budget.Spend(1) {
			return false
		}
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
		}
		return true
	}, func(*html.Node) {})
	return out
}

// uniqueSort joins lists of elements in document order, each once.
func (s *cheerioSelection) uniqueSort(lists [][]*html.Node) []*html.Node {
	in := map[*html.Node]bool{}
	for _, l := range lists {
		for _, n := range l {
			in[n] = true
		}
	}
	return s.find(func(n *html.Node) bool { return in[n] })
}

// elementChildren returns the elements directly under n.
func elementChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

// hasTraversal reports whether the tokens hold a combinator.
func hasTraversal(toks []selector.Token) bool {
	return slices.ContainsFunc(toks, func(t selector.Token) bool { return t.Combinator() })
}

// matchFrom returns the elements of the document that toks matches, in
// document order. With anchors, as css-select's rootFunc option has it,
// only those whose leftmost compound is one of them, found among the anchors
// and their descendants, and, when siblings is set (cheerio-select's
// selector that went on with a sibling combinator after a positional
// pseudo-class), their next siblings and the descendants of these; there,
// "* + x" selects as "* ~ x" does in cheerio-select. A selector that starts
// with a combinator is relative to the document's root, which no element
// is: it selects nothing.
func (s *cheerioSelection) matchFrom(anchors []*html.Node, toks []selector.Token, siblings bool) []*html.Node {
	toks = trimSpace(toks)
	if len(toks) == 0 || toks[0].Combinator() {
		return nil
	}
	if anchors == nil {
		matchers, ok := s.matchers(toks)
		if !ok {
			return nil
		}
		return s.find(func(n *html.Node) bool { return anyMatch(matchers, n) })
	}
	if len(anchors) == 0 {
		return nil
	}
	// css-select's prepareContext drops the anchors nested in another one
	// (domutils' removeSubsets), and the rootFunc reads that same list.
	anchors = s.removeSubsets(anchors)
	if siblings && toks[0].Kind == selector.Universal {
		toks = s.derive(toks, derivedSiblings, func(toks []selector.Token) []selector.Token {
			for k := 1; k < len(toks) && toks[k].Combinator(); k++ {
				if bytes.Equal(bytes.TrimSpace(toks[k].Text), []byte("+")) {
					toks = slices.Clone(toks)
					toks[k] = selector.Token{Text: []byte("~"), Kind: toks[k].Kind}
					break
				}
			}
			return toks
		})
	}
	region := s.region(anchors, siblings)
	// The anchors carry an attribute of a name the document cannot have
	// for the time of the match, and the selector starts with it. One name
	// serves every match, which do not nest, so that the selector is
	// compiled once.
	mark := fmt.Sprintf("data-safe-mjml-anchor-%p", s)
	for _, n := range anchors {
		n.Attr = append(n.Attr, html.Attribute{Key: mark})
	}
	defer func() {
		for _, n := range anchors {
			n.Attr = n.Attr[:len(n.Attr)-1]
		}
	}()
	// The attribute goes after the type selector of the leftmost compound,
	// in place of a universal one.
	marked := s.derive(toks, derivedMarked, func(toks []selector.Token) []selector.Token {
		head := selector.Token{Text: []byte("[" + mark + "]"), Kind: selector.Attr}
		switch toks[0].Kind {
		case selector.Universal:
			return append([]selector.Token{head}, toks[1:]...)
		case selector.Type:
			return append([]selector.Token{toks[0], head}, toks[1:]...)
		}
		return append([]selector.Token{head}, toks...)
	})
	matchers, ok := s.matchers(marked)
	if !ok {
		return nil
	}
	return s.find(func(n *html.Node) bool { return region[n] && anyMatch(matchers, n) })
}

// removeSubsets is domutils' removeSubsets: the nodes of list that are not
// inside another node of it, each once, in the order of list. Each ancestor
// looked at costs a step.
func (s *cheerioSelection) removeSubsets(list []*html.Node) []*html.Node {
	in := make(map[*html.Node]bool, len(list))
	for _, n := range list {
		in[n] = true
	}
	out := make([]*html.Node, 0, len(list))
	kept := make(map[*html.Node]bool, len(list))
	for _, n := range list {
		if kept[n] {
			continue
		}
		nested := false
		for p := n.Parent; p != nil && !nested; p = p.Parent {
			if !s.budget.Spend(1) {
				return out
			}
			nested = in[p]
		}
		if !nested {
			kept[n] = true
			out = append(out, n)
		}
	}
	return out
}

// region returns the anchors and their descendants, with siblings their
// next siblings and the descendants of these too: where cheerio-select
// looks for what a selector selects from them. Each node costs a step.
func (s *cheerioSelection) region(anchors []*html.Node, siblings bool) map[*html.Node]bool {
	in := map[*html.Node]bool{}
	add := func(top *html.Node) {
		if in[top] {
			return
		}
		htmlparser2.Traverse(top, func(n *html.Node) bool {
			if in[n] || !s.budget.Spend(1) {
				return false
			}
			in[n] = true
			return true
		}, func(*html.Node) {})
	}
	for _, a := range anchors {
		add(a)
		if siblings {
			for n := a.NextSibling; n != nil && !in[n]; n = n.NextSibling {
				add(n)
			}
		}
	}
	return in
}

func anyMatch(matchers []cascadia.Matcher, n *html.Node) bool {
	for _, m := range matchers {
		if m.Match(n) {
			return true
		}
	}
	return false
}

// filterList returns the elements of list that toks, without combinator,
// matches.
func (s *cheerioSelection) filterList(list []*html.Node, toks []selector.Token) []*html.Node {
	toks = trimSpace(toks)
	if len(toks) == 0 {
		return list
	}
	matchers, ok := s.matchers(toks)
	if !ok {
		return nil
	}
	var out []*html.Node
	for _, n := range list {
		if s.budget.Spend(1) && anyMatch(matchers, n) {
			out = append(out, n)
		}
	}
	return out
}

// findFilter is cheerio-select's findFilterElements: it selects what toks,
// which holds a positional pseudo-class, selects from the document (list
// nil) or among the elements of list, the part before the pseudo-class
// searched for in the whole document when query is set or the part has a
// combinator, and filtered otherwise.
func (s *cheerioSelection) findFilter(list []*html.Node, toks []selector.Token, query, siblings bool, totalLimit int) []*html.Node {
	k := s.aheadOf(toks).positional
	// The tokens before the pseudo-class are read, a step each, and what is
	// derived from them looked up: filterSteps.
	if !s.budget.Spend(filterSteps + max(k, 0)) {
		return nil
	}
	if k < 0 {
		// A selector that starts with a combinator, and nothing positional.
		return s.matchFrom(list, toks, false)
	}
	sub, filter := s.derive(toks[:k], derivedBeforeFilter, beforeFilter), &toks[k]
	partLimit := math.MaxInt
	if k == len(toks)-1 {
		partLimit = totalLimit
	}
	num, finite := s.argument(filter)
	limit := positionalLimit(filter.Name, num, finite, partLimit)
	if limit == 0 {
		return nil
	}
	var elems []*html.Node
	switch {
	case len(sub) == 0 && list == nil:
		elems = elementChildren(s.root)
	case len(sub) == 0:
		elems = list
	case query || hasTraversal(sub) || list == nil:
		elems = s.matchFrom(list, sub, siblings)
	default:
		elems = s.filterList(list, sub)
	}
	elems = jsSliceTo(elems, limit)
	result := s.byPosition(filter, elems)
	rest := toks[k+1:]
	if len(result) == 0 || len(trimSpace(rest)) == 0 {
		return result
	}
	ahead := s.aheadOf(rest)
	positional := ahead.positional >= 0
	if ahead.combinator {
		siblings := false
		if rest[0].Combinator() {
			// A sibling combinator looks at the next siblings too.
			for k := 0; k < len(rest) && rest[k].Combinator() && s.budget.Spend(1); k++ {
				c := rest[k].Text
				siblings = siblings || len(c) == 1 && (c[0] == '+' || c[0] == '~')
			}
			rest = s.derive(rest, derivedFromAny, fromAny)
		}
		if positional {
			return s.findFilter(result, rest, false, siblings, totalLimit)
		}
		return s.matchFrom(result, rest, siblings)
	}
	if positional {
		return s.findFilter(result, rest, false, false, totalLimit)
	}
	return s.filterList(result, rest)
}

// filterSteps is what a positional pseudo-class costs each time a selector
// is evaluated, besides the elements it reads: the lookups of what its
// evaluation derives, as long as a few hundred nodes walked.
const filterSteps = 250

// argumentSteps is what an argument of a :not() that holds a positional
// pseudo-class costs each time it is evaluated, besides the elements it
// reads, and elementSteps what reading an element there costs, with the
// lookups and copies it takes: a step is about what visiting a node in a
// search takes.
const (
	argumentSteps = 100
	elementSteps  = 8
)

// positionalLimit is cheerio-select's getLimit: how many of the elements
// matched before a positional pseudo-class it can need.
func positionalLimit(name string, num int64, finite bool, partLimit int) int {
	switch name {
	case "first":
		return 1
	case "nth", "eq":
		switch {
		case !finite:
			return 0
		case num >= 0:
			return int(min(num, math.MaxInt-1)) + 1
		}
	case "lt":
		switch {
		case !finite:
			return 0
		case num >= 0:
			return int(min(num, int64(partLimit)))
		}
	case "gt":
		if !finite {
			return 0
		}
	case "odd":
		return satMul2(partLimit)
	case "even":
		return satMul2(partLimit) - 1
	}
	return math.MaxInt
}

// jsSliceTo is JavaScript's elems.slice(0, end): a negative end counts from
// the end of the list.
func jsSliceTo(elems []*html.Node, end int) []*html.Node {
	if end < 0 {
		end = max(len(elems)+end, 0)
	}
	return elems[:min(end, len(elems))]
}

func satMul2(n int) int {
	if n > math.MaxInt/2 {
		return math.MaxInt
	}
	return 2 * n
}

// byPosition is cheerio-select's filterByPosition. Reading the elements
// costs a step each, and each argument of a :not() a step more, so that an
// argument list or a chain of pseudo-classes is paid for each time the
// selector is evaluated, an empty selection included.
func (s *cheerioSelection) byPosition(t *selector.Token, elems []*html.Node) []*html.Node {
	num, finite := s.argument(t)
	n := int64(len(elems))
	switch t.Name {
	case "last":
		if len(elems) > 0 {
			return elems[len(elems)-1:]
		}
		return elems
	case "nth", "eq":
		if !finite || abs(num) >= n {
			return nil
		}
		if num < 0 {
			num += n
		}
		return elems[num : num+1]
	case "gt":
		if !finite {
			return nil
		}
		// elems.slice(num + 1)
		if num >= n {
			return nil
		}
		from := num + 1
		if from < 0 {
			from = max(n+from, 0)
		}
		return elems[from:]
	case "even", "odd":
		if !s.budget.Spend(elementSteps * len(elems)) {
			return nil
		}
		var out []*html.Node
		for i, e := range elems {
			if (i%2 == 1) == (t.Name == "odd") {
				out = append(out, e)
			}
		}
		return out
	case "not":
		drop := map[*html.Node]bool{}
		for _, arg := range t.Args {
			if !s.budget.Spend(argumentSteps) {
				return nil
			}
			dropped := s.filterArgument(trimSpace(arg.Tokens), elems)
			if !s.budget.Spend(elementSteps * len(dropped)) {
				return nil
			}
			for _, e := range dropped {
				drop[e] = true
			}
		}
		if !s.budget.Spend(elementSteps * len(elems)) {
			return nil
		}
		var out []*html.Node
		for _, e := range elems {
			if !drop[e] {
				out = append(out, e)
			}
		}
		return out
	}
	// first and lt: the limit did it.
	return elems
}

// filterArgument returns the elements of elems that a selector of a :not()
// list selects (filterParsed, filterBySelector).
func (s *cheerioSelection) filterArgument(toks []selector.Token, elems []*html.Node) []*html.Node {
	ahead := s.aheadOf(toks)
	if ahead.positional < 0 {
		return s.filterList(elems, toks)
	}
	if !ahead.combinator {
		return s.findFilter(elems, toks, false, false, len(elems))
	}
	// Selected from the whole document, then kept when among elems
	// (:scope with elems as the context).
	if !s.budget.Spend(elementSteps * len(elems)) {
		return nil
	}
	in := map[*html.Node]bool{}
	for _, e := range elems {
		in[e] = true
	}
	var out []*html.Node
	for _, e := range s.findFilter(nil, toks, true, false, math.MaxInt) {
		if in[e] {
			out = append(out, e)
		}
	}
	return out
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// parseIntSaturating is JavaScript's parseInt(s, 10), and whether it is finite (not
// NaN): leading whitespace, a sign, then the digits up to the first other
// character. A value too large for int64 saturates.
func parseIntSaturating(s []byte) (int64, bool) {
	i := 0
	for i < len(s) && isJSSpace(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	start := i
	var n int64
	for i < len(s) && '0' <= s[i] && s[i] <= '9' {
		if n < math.MaxInt64/10 {
			n = n*10 + int64(s[i]-'0')
		} else {
			n = math.MaxInt64
		}
		i++
	}
	if i == start {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

func isJSSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}
