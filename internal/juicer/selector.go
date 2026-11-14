package juicer

import (
	"github.com/Notifuse/safe-mjml/internal/cascadia"
	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
)

// cascadia is used only as a match predicate: its Specificity() scores :hover
// and friends as zero and cannot represent :is()/:where(), so internal/selector
// computes specificity and hands cascadia text it can parse.

type rule struct {
	sel     string
	match   cascadia.Matcher
	spec    [3]int // ids, classes+attrs, types+pseudos
	pseudo  selector.PseudoElement
	group   int32 // index of the first rule expanded from the same selector
	key     selector.KeyKind
	keyName string
	classes []string // every class keyName can be, when there are several
	decls   []decl
	// valueBytes is the length of the values of decls (declarationSteps).
	valueBytes int
	throws     bool // juice 9 throws when it applies this rule
}

// nonVisualElements never receive a style attribute. This is an element-level
// test, not subtree pruning: juice styles a <b> inside <noscript>.
var nonVisualElements = map[string]bool{
	"head": true, "title": true, "base": true, "link": true,
	"style": true, "meta": true, "script": true, "noscript": true,
}

// compileRule turns one selector of a list into rules, one per :is()
// alternative. It returns ignored=true for a selector juice preserves rather
// than inlines, invalid=true for one browsers reject, and no rules at all for
// one neither engine can match.
func compileRule(arm []byte, decls []decl, o *options) (rules []rule, ignored, invalid bool) {
	c := selector.Compile(arm, selector.Mode{JuiceSpecificity: o.juiceSpecificity, Juice9: o.juice9Selectors, JuiceScope: o.juiceScope, Lenient: o.applyInvalid})
	if c.Ignored || c.Invalid {
		return nil, c.Ignored, c.Invalid
	}
	sel := string(arm)
	for _, v := range c.Variants {
		parse := cascadia.ParseWithBudget
		if o.juice9Selectors {
			// juice 9 and 10 match with css-select.
			parse = cascadia.ParseForJuice
		}
		m, err := parse(v.Text, o.budget)
		if err != nil {
			// juice swallows selectors its engine cannot parse.
			continue
		}
		r := rule{sel: sel, decls: decls, valueBytes: valueBytes(decls), spec: c.Spec, pseudo: c.Pseudo, match: m, key: v.Key, keyName: v.KeyName, classes: v.Classes}
		rules = append(rules, r)
	}
	return rules, false, false
}

// valueBytes is the length of the values of decls.
func valueBytes(decls []decl) int {
	n := 0
	for _, d := range decls {
		n += len(d.value)
	}
	return n
}
