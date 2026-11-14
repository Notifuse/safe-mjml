package juicer

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/juicer/mensch"
	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
)

// Where the Juice9 profile reads stylesheets differently from juice 12.

// parseMensch is parseStylesheet for the Juice9 profile: juice 9 reads
// stylesheets with mensch and preserves rules through its stringifier.
func parseMensch(src []byte, opts *options, ord uint32) ([]rule, []preserved, uint32) {
	var rules []rule
	var keep []preserved
	for _, n := range mensch.Parse(mensch.Lex(string(src))) {
		if n.Type != "rule" {
			if preserveKind(n.Type, opts) {
				keep = append(keep, preserved{n.Type, mensch.Stringify(n)})
			}
			continue
		}
		var decls []decl
		var throws bool
		decls, throws, ord = menschDecls(n, ord, opts.juice10, false)
		// An empty rule still counts: juice 9 rewrites the style attribute of
		// every element a rule matches.
		for _, sel := range n.Selectors {
			if sel == "" {
				continue
			}
			armRules, _, _ := compileRule([]byte(sel), decls, opts)
			for k := range armRules {
				armRules[k].group = int32(len(rules))
				armRules[k].throws = throws
			}
			rules = append(rules, armRules...)
		}
		if opts.preservePseudos && selector.Juice9IgnoredPseudo(n.Selectors[0]) {
			keep = append(keep, preserved{"pseudo", mensch.Stringify(n)})
		}
	}
	return rules, keep, ord
}

// menschDecls reads a rule's declarations as juice 9's addProps does. bang
// holds the value as written, which juice 9 keeps when preserving !important.
// A declaration without a name is juice's "undefined" property: juice 9
// writes it, and with unnamed (juice 10) it is marked, for the render to
// fail if it survives the cascade (see unnamedSurvivors). In a style
// attribute (inStyle), one made only of protected template blocks is kept
// as written (bare): a Liquid tag ending a style, {% endif %}, was dropped.
func menschDecls(n mensch.Node, ord uint32, unnamed, inStyle bool) (decls []decl, throws bool, _ uint32) {
	for _, d := range n.Decls {
		if d.Type != "property" {
			continue
		}
		if d.Value == "" {
			// mensch reads "a{b{" as a declaration b with no value, which
			// juice 9 reads with value.match and throws on.
			throws = true
			continue
		}
		v, important := strings.CutSuffix(d.Value, "!important")
		if important {
			v = strings.TrimRightFunc(v, mensch.JSSpace)
		} else {
			v = d.Value
		}
		name := d.Name
		if name == "" && inStyle && onlyPlaceholders(d.Value) {
			// A template block alone between semicolons of a style
			// attribute ({% endif %}, {{ extra }}): kept where it is, as
			// written, under a name no property has (Pierre's decision;
			// juice drops it, or throws on it before a semicolon).
			decls = append(decls, decl{prop: "\x00" + d.Value, value: []byte(d.Value), bare: true, ord: ord, keepEmpty: true})
			ord++
			continue
		}
		var nameless bool
		if name == "" {
			// A word with no colon, which juice writes as the property
			// "undefined". juice 10 reads the name of every property of an
			// element it styles to resolve CSS variables, and throws on
			// this one if it is still there.
			name, nameless = "undefined", unnamed
		}
		decls = append(decls, decl{prop: name, value: []byte(v), important: important, bang: []byte(d.Value), ord: ord, keepEmpty: true, unnamed: nameless})
		ord++
	}
	return decls, throws, ord
}

// menschStyle reads a style attribute as juice 9 does, through mensch as the
// body of a "*" rule; see menschDecls for unnamed.
func menschStyle(style []byte, unnamed bool) ([]decl, error) {
	for _, n := range mensch.Parse(mensch.Lex("* { " + string(style) + " } ")) {
		if n.Type != "rule" {
			continue
		}
		decls, throws, ord := menschDecls(n, 0, unnamed, true)
		if throws {
			break
		}
		// mensch drops a word after the last semicolon; a template block
		// there ({% endif %} after "font-size:20px;") is kept as written.
		tail := style[bytes.LastIndexByte(style, ';')+1:]
		if onlyPlaceholders(string(tail)) {
			t := strings.TrimFunc(string(tail), mensch.JSSpace)
			decls = append(decls, decl{prop: "\x00" + t, value: []byte(t), bare: true, ord: ord, keepEmpty: true})
		}
		return decls, nil
	}
	return nil, fmt.Errorf("%w: juice cannot read the style attribute %q", ErrJuiceFails, style)
}

// onlyPlaceholders reports whether s is made of code block placeholders
// (encodeCodeBlocks) and spaces only.
func onlyPlaceholders(s string) bool {
	s = strings.TrimFunc(s, mensch.JSSpace)
	if s == "" {
		return false
	}
	for s != "" {
		rest, ok := strings.CutPrefix(s, placeholderPrefix)
		if !ok {
			return false
		}
		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits == 0 || digits == len(rest) || rest[digits] != '_' {
			return false
		}
		s = strings.TrimLeftFunc(rest[digits+1:], mensch.JSSpace)
	}
	return true
}
