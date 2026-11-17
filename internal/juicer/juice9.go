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
	nodes := mensch.Parse(mensch.Lex(string(src)))
	var inline []bool
	if opts.juice11 {
		inline = juice11Inlined(nodes)
	}
	for i, n := range nodes {
		if n.Type != "rule" {
			if preserveKind(n.Type, opts) {
				keep = append(keep, preserved{n.Type, mensch.Stringify(n)})
			}
			continue
		}
		if opts.juice11 {
			// juice 11 leaves out the declarations its comments ignore, and
			// a rule without declarations left, which so no longer rewrites
			// the style attribute of the elements it matches.
			n.Decls = juice11Decls(n.Decls)
			if !inline[i] || len(n.Decls) == 0 {
				// What is preserved follows juice 10 (see Juice11).
				if opts.preservePseudos && selector.Juice9IgnoredPseudo(n.Selectors[0]) {
					keep = append(keep, preserved{"pseudo", mensch.Stringify(n)})
				}
				continue
			}
		}
		var decls []decl
		var throws bool
		decls, throws, ord = menschDecls(n, ord, opts.juice10, false)
		// An empty rule still counts: juice 9 and 10 rewrite the style
		// attribute of every element a rule matches.
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

// juice11Inlined reports, for each node of a stylesheet, whether juice
// 11.1.1's parseCSS inlines it, by its comments: none when the stylesheet
// opens with "juice ignore"; none between "juice start ignore" and "juice
// end ignore" (or the end); and not the node after "juice ignore next",
// whatever it is, a comment included. The stylesheet is the whole stream
// juice reads, every <style> block then extraCss (joinStylesheets).
func juice11Inlined(nodes []mensch.Node) []bool {
	inline := make([]bool, len(nodes))
	if len(nodes) > 0 && juice11Directive(nodes[0]) == "juice ignore" {
		return inline
	}
	ignoring := false
	for i := 0; i < len(nodes); i++ {
		if nodes[i].Type == "comment" {
			switch juice11Directive(nodes[i]) {
			case "juice start ignore":
				ignoring = true
			case "juice end ignore":
				ignoring = false
			case "juice ignore next":
				i++
			}
			continue
		}
		inline[i] = !ignoring
	}
	return inline
}

// juice11Decls returns the properties of a rule that juice 11 inlines: a
// "juice ignore next" comment leaves out the next property, the comments
// themselves are dropped.
func juice11Decls(decls []mensch.Node) []mensch.Node {
	var out []mensch.Node
	skipNext := false
	for _, d := range decls {
		if d.Type == "comment" && juice11Directive(d) == "juice ignore next" {
			skipNext = true
			continue
		}
		if d.Type != "property" {
			continue
		}
		if skipNext {
			skipNext = false
			continue
		}
		out = append(out, d)
	}
	return out
}

// juice11Directive returns a comment's text trimmed as JavaScript trims it,
// "" for any other node.
func juice11Directive(n mensch.Node) string {
	if n.Type != "comment" {
		return ""
	}
	return strings.TrimFunc(n.Text, mensch.JSSpace)
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
// body of a "*" rule; see menschDecls for unnamed, and juice11Decls.
func menschStyle(style []byte, unnamed, juice11 bool) ([]decl, error) {
	for _, n := range mensch.Parse(mensch.Lex("* { " + string(style) + " } ")) {
		if n.Type != "rule" {
			continue
		}
		if juice11 {
			// juice 11 reads the attribute with parseCSS, whose "juice
			// ignore next" leaves out a property here too. An attribute
			// left with no declaration makes juice 11 throw (it reads the
			// first rule parseCSS no longer returns); safe-mjml reads it as
			// juice 10 does (Pierre's decision, 2026-11-17).
			n.Decls = juice11Decls(n.Decls)
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
