package testutils

import (
	"fmt"
	"slices"
	"strings"
)

// CSSRule is one rule of a stylesheet, as written: a qualified rule
// (`selector { declarations }`), an at-rule with a block (`@media … { rules }`,
// `@font-face { declarations }`) or a statement at-rule (`@import …;`).
type CSSRule struct {
	// Prelude is the selector list or the at-rule header, whitespace-normalized.
	Prelude string
	// Declarations of a rule whose block holds declarations, in source order.
	Declarations []CSSDeclaration
	// Rules of an at-rule whose block holds rules (@media, @supports,
	// @keyframes…), in source order.
	Rules []CSSRule
	// Statement is true for an at-rule without a block, such as @import, and
	// for the strayBrace marker.
	Statement bool
}

// strayBrace is the prelude of the statement recorded for a closing brace that
// closes nothing, so that the rest of the stylesheet is still parsed and the
// brace itself shows up as a difference.
const strayBrace = "<unmatched }>"

// CSSDeclaration is one `property: value` pair, whitespace-normalized.
type CSSDeclaration struct {
	Property string
	Value    string
}

func (d CSSDeclaration) String() string {
	return d.Property + ":" + d.Value
}

// String serializes the rule in a canonical form, used in difference reports.
func (r CSSRule) String() string {
	if r.Statement {
		return r.Prelude + ";"
	}
	var b strings.Builder
	b.WriteString(r.Prelude)
	b.WriteString(" { ")
	for _, d := range r.Declarations {
		b.WriteString(d.String())
		b.WriteString("; ")
	}
	for _, nested := range r.Rules {
		b.WriteString(nested.String())
		b.WriteString(" ")
	}
	b.WriteString("}")
	return b.String()
}

// declarationAtRules are the at-rules whose block contains declarations. Every
// other at-rule block (@media, @supports, @keyframes, @container,
// @-moz-document…) is parsed as a list of rules.
var declarationAtRules = map[string]bool{
	"@font-face":           true,
	"@page":                true,
	"@property":            true,
	"@counter-style":       true,
	"@font-palette-values": true,
	"@viewport":            true,
	"@-ms-viewport":        true,
}

// ParseStylesheet parses the content of a <style> element into its rules, in
// source order. It is a small, tolerant parser meant for comparing compiler
// output in tests, not a full CSS implementation: comments are dropped,
// whitespace outside strings is normalized, and strings and parentheses are
// respected when looking for the braces and semicolons that delimit rules and
// declarations. A closing brace that closes nothing is kept as a strayBrace
// statement, and parsing goes on after it.
func ParseStylesheet(css string) []CSSRule {
	rules, _ := parseRules(stripComments(css), 0, true)
	return rules
}

// parseRules reads rules from css starting at pos until the end of input or,
// unless topLevel, the closing brace of the enclosing block; it returns them
// with the position after that brace.
func parseRules(css string, pos int, topLevel bool) ([]CSSRule, int) {
	var rules []CSSRule
	for {
		start := pos
		end, delim := scanUntil(css, pos, "{;}")
		raw := css[start:end]
		prelude := normalizePrelude(raw)
		if delim != '{' && prelude != "" {
			rules = append(rules, CSSRule{Prelude: prelude, Statement: true})
		}
		switch delim {
		case 0: // end of input
			return rules, end
		case '}':
			if !topLevel {
				return rules, end + 1
			}
			rules = append(rules, CSSRule{Prelude: strayBrace, Statement: true})
			pos = end + 1
		case ';':
			pos = end + 1
		case '{':
			rule := CSSRule{Prelude: prelude}
			if keyword := atRuleKeyword(raw); keyword != "" && !declarationAtRules[keyword] {
				rule.Rules, pos = parseRules(css, end+1, false)
			} else {
				rule.Declarations, pos = parseDeclarations(css, end+1)
			}
			rules = append(rules, rule)
		}
	}
}

// atRuleKeyword returns the lowercase at-keyword a prelude starts with (such as
// "@media" for `@media (min-width:480px)`), or "" for a qualified rule.
func atRuleKeyword(prelude string) string {
	prelude = strings.TrimSpace(prelude)
	if !strings.HasPrefix(prelude, "@") {
		return ""
	}
	end := strings.IndexFunc(prelude[1:], func(r rune) bool {
		return r == '(' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	})
	if end == -1 {
		return strings.ToLower(prelude)
	}
	return strings.ToLower(prelude[:end+1])
}

// parseDeclarations reads declarations from css starting at pos until the
// closing brace of the block, and returns them with the position after it.
func parseDeclarations(css string, pos int) ([]CSSDeclaration, int) {
	var declarations []CSSDeclaration
	for {
		start := pos
		end, delim := scanUntil(css, pos, ";}")
		if decl, ok := parseDeclaration(css[start:end]); ok {
			declarations = append(declarations, decl)
		}
		switch delim {
		case 0:
			return declarations, end
		case '}':
			return declarations, end + 1
		default:
			pos = end + 1
		}
	}
}

func parseDeclaration(text string) (CSSDeclaration, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return CSSDeclaration{}, false
	}
	colon := strings.IndexByte(text, ':')
	if colon == -1 {
		return CSSDeclaration{Property: normalizeValue(text)}, true
	}
	return CSSDeclaration{
		Property: strings.ToLower(strings.TrimSpace(text[:colon])),
		Value:    normalizeValue(text[colon+1:]),
	}, true
}

// scanUntil returns the index of the first byte of css at or after pos that is
// one of delims, outside strings and parentheses, and that byte; or len(css)
// and 0 when there is none.
func scanUntil(css string, pos int, delims string) (int, byte) {
	depth := 0
	var quote byte
	for i := pos; i < len(css); i++ {
		c := css[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0 && strings.IndexByte(delims, c) != -1:
			return i, c
		}
	}
	return len(css), 0
}

// stripComments removes /* … */ comments outside strings.
func stripComments(css string) string {
	var b strings.Builder
	b.Grow(len(css))
	var quote byte
	for i := 0; i < len(css); i++ {
		c := css[i]
		switch {
		case quote != 0:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(css) {
				i++
				b.WriteByte(css[i])
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			b.WriteByte(c)
		case c == '/' && i+1 < len(css) && css[i+1] == '*':
			end := strings.Index(css[i+2:], "*/")
			if end == -1 {
				return b.String()
			}
			i += end + 3
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

// collapseSpace trims text and replaces each run of whitespace outside
// strings by one space; whitespace inside strings is kept as written.
func collapseSpace(text string) string {
	text = strings.TrimSpace(text)
	var b strings.Builder
	b.Grow(len(text))
	var quote byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote != 0:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(text) {
				i++
				b.WriteByte(text[i])
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			b.WriteByte(c)
		case isSpace(c):
			for i+1 < len(text) && isSpace(text[i+1]) {
				i++
			}
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// normalizePrelude collapses whitespace in a selector list or at-rule header
// and, outside strings, removes the whitespace that carries no meaning there:
// around commas and the >, + and ~ combinators, just inside parentheses, and
// after the colon of a media feature. The whitespace before an opening or
// after a closing parenthesis is kept: in `.a:not(.b) .c` it is a descendant
// combinator.
func normalizePrelude(text string) string {
	text = collapseSpace(text)
	var b strings.Builder
	b.Grow(len(text))
	var quote byte
	depth := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote != 0:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(text) {
				i++
				b.WriteByte(text[i])
			} else if c == quote {
				quote = 0
			}
			continue
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == ' ':
			var prev, next byte
			if out := b.String(); out != "" {
				prev = out[len(out)-1]
			}
			if i+1 < len(text) {
				next = text[i+1]
			}
			if strings.IndexByte(",>+~(", prev) != -1 && prev != 0 ||
				strings.IndexByte(",>+~)", next) != -1 && next != 0 ||
				depth > 0 && prev == ':' {
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// normalizeValue collapses whitespace in a declaration value, outside strings,
// and writes `!important` without a space before it.
func normalizeValue(text string) string {
	text = collapseSpace(text)
	text = strings.ReplaceAll(text, " !important", "!important")
	text = strings.ReplaceAll(text, "! important", "!important")
	return text
}

// CompareStylesheets compares two stylesheets rule by rule and declaration by
// declaration, order included, and returns a description of each difference
// (empty when they are equal). Prefix names the stylesheet in the report.
func CompareStylesheets(expected, actual []CSSRule, prefix string) []string {
	var diffs []string
	n := max(len(expected), len(actual))
	for i := 0; i < n; i++ {
		where := fmt.Sprintf("%srule %d", prefix, i+1)
		switch {
		case i >= len(expected):
			diffs = append(diffs, fmt.Sprintf("%s: extra rule %s", where, actual[i]))
			continue
		case i >= len(actual):
			diffs = append(diffs, fmt.Sprintf("%s: missing rule %s", where, expected[i]))
			continue
		}
		e, a := expected[i], actual[i]
		if e.Prelude != a.Prelude || e.Statement != a.Statement {
			diffs = append(diffs, fmt.Sprintf("%s: expected %s, got %s", where, e, a))
			continue
		}
		if len(e.Rules) > 0 || len(a.Rules) > 0 {
			diffs = append(diffs, CompareStylesheets(e.Rules, a.Rules, where+" ("+e.Prelude+") > ")...)
		}
		if !slices.Equal(e.Declarations, a.Declarations) {
			diffs = append(diffs, fmt.Sprintf("%s (%s): expected { %s }, got { %s }",
				where, e.Prelude, joinDeclarations(e.Declarations), joinDeclarations(a.Declarations)))
		}
	}
	return diffs
}

func joinDeclarations(decls []CSSDeclaration) string {
	parts := make([]string, len(decls))
	for i, d := range decls {
		parts[i] = d.String()
	}
	return strings.Join(parts, "; ")
}
