package cascadia

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// The css-select mode of the parser (ParseForJuice): selectors match as
// css-select 5.2.2 matches them, the engine of juice, cheerio and so of
// MJML, where cascadia follows the browsers or its own extensions:
//
//   - attribute values are compared without case for the attributes HTML
//     lists as case-insensitive (type, lang, dir…), unless the selector has
//     the s flag, which is read;
//   - an empty value never matches ^=, $= and *=, ~= follows css-select's
//     regular expression, and [a!=""] matches a non-empty value;
//   - classes are split on JavaScript's \s, the no-break space included;
//   - :contains() is case-sensitive, :icontains() is not, and they read the
//     text as domutils does (a <br> is a line break);
//   - :empty counts any text, whitespace included;
//   - css-select's aliases (:header, :checked, :disabled, :button…) are the
//     selectors css-select expands them to;
//   - cascadia's own attribute operator #= is an error, as css-select has
//     none.

// caseInsensitiveAttributes are the attributes whose values css-select
// compares without case in HTML (html.spec.whatwg.org, case-sensitivity of
// selectors).
var caseInsensitiveAttributes = map[string]bool{
	"accept": true, "accept-charset": true, "align": true, "alink": true, "axis": true, "bgcolor": true,
	"charset": true, "checked": true, "clear": true, "codetype": true, "color": true, "compact": true,
	"declare": true, "defer": true, "dir": true, "direction": true, "disabled": true, "enctype": true,
	"face": true, "frame": true, "hreflang": true, "http-equiv": true, "lang": true, "language": true,
	"link": true, "media": true, "method": true, "multiple": true, "nohref": true, "noresize": true,
	"noshade": true, "nowrap": true, "readonly": true, "rel": true, "rev": true, "rules": true,
	"scope": true, "scrolling": true, "selected": true, "shape": true, "target": true, "text": true,
	"type": true, "valign": true, "valuetype": true, "vlink": true,
}

// cssSelectAliases are css-select's pseudo-classes expressed as selectors,
// written as this parser reads them (:has(> x) is :haschild(x), and :is()
// is a selector list).
var cssSelectAliases = map[string]string{
	"any-link": "a[href], area[href], link[href]",
	"disabled": cssSelectDisabled,
	"enabled":  "*:not(" + cssSelectDisabled + ")",
	"checked":  "input[type=radio][checked], input[type=checkbox][checked], " + cssSelectSelected,
	"required": "input[required], select[required], textarea[required]",
	"optional": "input:not([required]), select:not([required]), textarea:not([required])",
	"selected": cssSelectSelected,
	"checkbox": "[type=checkbox]",
	"file":     "[type=file]",
	"password": "[type=password]",
	"radio":    "[type=radio]",
	"reset":    "[type=reset]",
	"image":    "[type=image]",
	"submit":   "[type=submit]",
	"parent":   "*:not(:empty)",
	"header":   "h1, h2, h3, h4, h5, h6",
	"button":   "button, input[type=button]",
	"input":    "input, textarea, select, button",
	"text":     "input:not([type!='']), input[type=text]",
}

const (
	cssSelectDisabled = "button[disabled], input[disabled], select[disabled], textarea[disabled], optgroup[disabled], option[disabled], " +
		"optgroup[disabled] > option, fieldset[disabled]:not(fieldset[disabled] legend:first-of-type *)"
	cssSelectSelected = "option[selected], select:not([multiple]):not(:haschild(option[selected])) > option:first-of-type"
)

// isJSSpace reports whether r is in JavaScript's \s.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// IsJSSpace is isJSSpace, for the class index of juicer, which must split
// a class attribute as the css-select mode does.
func IsJSSpace(r rune) bool { return isJSSpace(r) }

// firstAttr returns the value of n's first attribute named key, as
// htmlparser2 keeps the first of repeated attributes.
func firstAttr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// cssAttrSelector is an attribute, class or id selector in css-select's
// semantics. op is "" (exists), "=", "~=", "|=", "^=", "$=", "*=" or "!=".
type cssAttrSelector struct {
	key, val, op string
	insensitive  bool
	// lower is val in lower case, and spaced whether it holds a space,
	// computed once: a long value was lowered or scanned again for every
	// element tested.
	lower  string
	spaced bool
	spec   Specificity
	text   string
	budget *Budget
}

// newCSSAttrSelector returns a cssAttrSelector with what it computes once.
func newCSSAttrSelector(s cssAttrSelector) cssAttrSelector {
	s.lower = strings.ToLower(s.val)
	s.spaced = strings.IndexFunc(s.val, isJSSpace) >= 0
	return s
}

func (s cssAttrSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode || !s.budget.Spend(1+len(n.Attr)) {
		return false
	}
	attr, ok := firstAttr(n, s.key)
	cost := len(attr) / bytesPerStep
	if s.insensitive {
		cost += len(attr) // strings.ToLower
	}
	if !s.budget.Spend(cost) {
		return false
	}
	// eq compares part of the attribute with the value; the lengths first,
	// before lowering anything.
	eq := func(a string) bool {
		if s.insensitive {
			return len(a) == len(s.val) && strings.ToLower(a) == s.lower
		}
		return a == s.val
	}
	switch s.op {
	case "":
		return ok
	case "=":
		return ok && eq(attr)
	case "!=":
		if s.val == "" {
			return attr != ""
		}
		return !ok || !eq(attr)
	case "~=":
		return ok && !s.spaced && len(attr) >= len(s.val) && containsWord(attr, s.val, s.lower, s.insensitive)
	case "|=":
		return ok && (len(attr) == len(s.val) || len(attr) > len(s.val) && attr[len(s.val)] == '-') &&
			len(attr) >= len(s.val) && eq(attr[:len(s.val)])
	case "^=":
		return ok && s.val != "" && len(attr) >= len(s.val) && eq(attr[:len(s.val)])
	case "$=":
		return ok && s.val != "" && len(attr) >= len(s.val) && eq(attr[len(attr)-len(s.val):])
	case "*=":
		if !ok || s.val == "" || len(attr) < len(s.val) {
			return false
		}
		if s.insensitive {
			return strings.Contains(strings.ToLower(attr), s.lower)
		}
		return strings.Contains(attr, s.val)
	}
	return false
}

// containsWord is css-select's ~=: attr matches (?:^|\s)word(?:$|\s), with
// JavaScript's \s. lower is word in lower case, for insensitive; the caller
// rules out a word holding a space, which matches nothing.
func containsWord(attr, word, lower string, insensitive bool) bool {
	if insensitive {
		attr, word = strings.ToLower(attr), lower
	}
	for i := 0; i+len(word) <= len(attr); {
		j := strings.Index(attr[i:], word)
		if j < 0 {
			return false
		}
		at := i + j
		before, _ := utf8.DecodeLastRuneInString(attr[:at])
		after, _ := utf8.DecodeRuneInString(attr[at+len(word):])
		if (at == 0 || isJSSpace(before)) && (at+len(word) == len(attr) || isJSSpace(after)) {
			return true
		}
		if word == "" {
			// The empty word sits between any two characters: try the
			// next position.
			_, size := utf8.DecodeRuneInString(attr[at:])
			i = at + max(size, 1)
			continue
		}
		i = at + 1
	}
	return false
}

func (s cssAttrSelector) Specificity() Specificity { return s.spec }
func (s cssAttrSelector) PseudoElement() string    { return "" }
func (s cssAttrSelector) String() string           { return s.text }

// cssSelectSimple turns an attribute, class or id selector into its
// css-select form, with flag the attribute selector's flag (0, 'i' or 's').
func (p *parser) cssSelectSimple(ns Sel, flag byte) (Sel, error) {
	switch t := ns.(type) {
	case classSelector:
		return newCSSAttrSelector(cssAttrSelector{key: "class", val: t.class, op: "~=", spec: t.Specificity(), text: t.String(), budget: p.budget}), nil
	case idSelector:
		return newCSSAttrSelector(cssAttrSelector{key: "id", val: t.id, op: "=", spec: t.Specificity(), text: t.String(), budget: p.budget}), nil
	case attrSelector:
		if t.operation == "#=" {
			return nil, fmt.Errorf("attribute operator #= is not css-select's")
		}
		insensitive := flag == 'i' || flag == 0 && caseInsensitiveAttributes[t.key]
		return newCSSAttrSelector(cssAttrSelector{key: t.key, val: t.val, op: t.operation, insensitive: insensitive, spec: t.Specificity(), text: t.String(), budget: p.budget}), nil
	}
	return ns, nil
}

// cssSelectPseudo parses the pseudo-classes css-select reads differently
// from cascadia, after their name: ok is false for the others.
func (p *parser) cssSelectPseudo(name string) (out Sel, ok bool, err error) {
	if expansion, isAlias := cssSelectAliases[name]; isAlias {
		sub := &parser{s: expansion, cssSelect: true, budget: p.budget, depth: p.depth}
		group, err := sub.parseSelectorGroup()
		if err != nil || sub.i < len(sub.s) {
			return nil, true, fmt.Errorf("css-select alias :%s: %v", name, err)
		}
		return aliasSelector{name: name, group: group, budget: p.budget}, true, nil
	}
	switch name {
	case "empty":
		return cssEmptySelector{abstractPseudoClass: p.pseudo()}, true, nil
	case "contains", "icontains":
		// Not consumeParenthesis, which skips the spaces after "(": they
		// are part of the text, as css-what reads it.
		if p.i >= len(p.s) || p.s[p.i] != '(' {
			return nil, true, errExpectedParenthesis
		}
		p.i++
		// As css-what's readValueWithParenthesis: up to the ')' that
		// closes the '(', counting the unescaped ones in between, with the
		// spaces; quotes go only when they open and close the value.
		// A backslash escapes the byte after it: skipping that byte reads
		// each byte once, where counting the backslashes before each byte
		// costs the square of a run of them.
		start, depth := p.i, 1
		for ; depth > 0 && p.i < len(p.s); p.i++ {
			switch p.s[p.i] {
			case '\\':
				p.i++
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		if depth > 0 {
			return nil, true, errExpectedClosingParenthesis
		}
		value := unescapeCSS(p.s[start : p.i-1])
		if n := len(value); n >= 2 && (value[0] == '"' || value[0] == '\'') && value[n-1] == value[0] {
			value = value[1 : n-1]
		}
		value = unescapeCSS(value)
		return cssContainsSelector{abstractPseudoClass: p.pseudo(), value: value, lower: strings.ToLower(value), insensitive: name == "icontains"}, true, nil
	}
	return nil, false, nil
}

// aliasSelector is a css-select alias: the element matches one of group.
type aliasSelector struct {
	name   string
	group  SelectorGroup
	budget *Budget
}

func (s aliasSelector) Match(n *html.Node) bool {
	return n.Type == html.ElementNode && s.budget.Spend(1) && s.group.Match(n)
}
func (s aliasSelector) Specificity() Specificity { return Specificity{0, 1, 0} }
func (s aliasSelector) PseudoElement() string    { return "" }
func (s aliasSelector) String() string           { return ":" + s.name }

// cssEmptySelector is css-select's :empty: no element child, and no text,
// whitespace included.
type cssEmptySelector struct {
	abstractPseudoClass
}

func (s cssEmptySelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !s.budget.Spend(1) {
			return false
		}
		if c.Type == html.ElementNode || c.Type == html.TextNode && c.Data != "" {
			return false
		}
	}
	return true
}
func (s cssEmptySelector) String() string { return ":empty" }

// cssContainsSelector is css-select's :contains() and :icontains().
type cssContainsSelector struct {
	abstractPseudoClass
	value       string
	lower       string // value in lower case, for :icontains
	insensitive bool
}

func (s cssContainsSelector) Match(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	var b strings.Builder
	if !domutilsText(s.budget, n, &b) {
		return false
	}
	if s.insensitive {
		if !s.budget.Spend(b.Len()) {
			return false
		}
		return strings.Contains(strings.ToLower(b.String()), s.lower)
	}
	return strings.Contains(b.String(), s.value)
}

func (s cssContainsSelector) String() string {
	if s.insensitive {
		return fmt.Sprintf(":icontains(%q)", s.value)
	}
	return fmt.Sprintf(":contains(%q)", s.value)
}

// domutilsText writes n's text as domutils' getText reads it: the text of
// its text descendants, with a line break for each <br>. It returns false
// when the budget is spent.
func domutilsText(b *Budget, n *html.Node, out *strings.Builder) bool {
	if !b.Spend(1 + len(n.Data)/bytesPerStep) {
		return false
	}
	switch n.Type {
	case html.TextNode:
		out.WriteString(n.Data)
	case html.ElementNode:
		if n.Data == "br" {
			out.WriteByte('\n')
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if !domutilsText(b, c, out) {
				return false
			}
		}
	}
	return true
}

// unescapeCSS is css-what's unescapeCSS: a backslash and up to six hex
// digits (and one space after them) is that code point, and a backslash
// before another character is that character.
func unescapeCSS(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && j < i+7 && isHexDigit(s[j]) {
			j++
		}
		if j == i+1 {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		var cp rune
		for _, c := range s[i+1 : j] {
			cp = cp*16 + rune(hexValue(byte(c)))
		}
		if cp == 0 || cp > 0x10ffff || cp >= 0xd800 && cp <= 0xdfff {
			cp = 0xfffd
		}
		b.WriteRune(cp)
		if j < len(s) && s[j] == ' ' {
			j++
		}
		i = j - 1
	}
	return b.String()
}

func isHexDigit(c byte) bool { return c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'f' }

func hexValue(c byte) int {
	if c <= '9' {
		return int(c - '0')
	}
	return int(c|0x20-'a') + 10
}
