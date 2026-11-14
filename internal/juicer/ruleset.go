package juicer

import (
	"cmp"
	"iter"
	"slices"

	"github.com/Notifuse/safe-mjml/internal/cascadia"
	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
	"golang.org/x/net/html"
)

// A rightmost-key rule index, the optimization browsers use and juice does
// not. juice runs one full document traversal per rule, so matching is
// O(rules x nodes) and dominates its runtime; here rules are bucketed by the
// id, class or tag of their rightmost compound and a node only tests the
// buckets its own id, classes and tag can reach.
//
// Buckets hold rule indices rather than matchers, and candidates are merged in
// ascending index order, because rules must be applied in stylesheet order:
// that order decides which selector wins a property and therefore where the
// property lands in the output.

type ruleSet struct {
	rules   []rule
	byID    map[string][]uint32
	byClass map[string][]uint32
	byTag   map[string][]uint32
	// any holds rules whose rightmost compound is universal or keyed only by
	// an attribute, which no node-derived key can index.
	any []uint32
	// budget takes a step for each rule tested on an element (MaxMatchSteps).
	budget *cascadia.Budget
	// jsSpaces: class attributes split as css-select splits them
	// (jsClassTokens), in the juice 9 and 10 profiles.
	jsSpaces bool
}

// scratch is per-document reusable state, kept off the Inliner so it stays
// safe to share.
type scratch struct {
	lists [][]uint32
	pos   []int32
}

// rarestClassKey turns on the choice of the rarest class in newRuleSet; a
// test turns it off to check that it changes no output.
var rarestClassKey = true

// newRuleSet indexes rules for the document under root. A rule keyed by a
// class whose compound has several (.card.c7) goes in the bucket of the one
// fewest elements of the document have: an element it matches has them all,
// and with the first one, every element of a common class tested every rule
// that names it (rules x elements).
func newRuleSet(rules []rule, root *html.Node, jsSpaces bool) *ruleSet {
	rs := &ruleSet{
		rules:    rules,
		byID:     make(map[string][]uint32),
		byClass:  make(map[string][]uint32),
		byTag:    make(map[string][]uint32),
		jsSpaces: jsSpaces,
	}
	var counts map[string]int
	for i := range rules {
		r := &rules[i]
		if r.match == nil {
			continue
		}
		switch r.key {
		case selector.KeyID:
			rs.byID[r.keyName] = append(rs.byID[r.keyName], uint32(i))
		case selector.KeyClass:
			name := r.keyName
			if rarestClassKey && len(r.classes) > 1 {
				if counts == nil {
					counts = rs.classCounts(root)
				}
				for _, c := range r.classes {
					if counts[c] < counts[name] {
						name = c
					}
				}
			}
			rs.byClass[name] = append(rs.byClass[name], uint32(i))
		case selector.KeyTag:
			rs.byTag[r.keyName] = append(rs.byTag[r.keyName], uint32(i))
		default:
			rs.any = append(rs.any, uint32(i))
		}
	}
	return rs
}

// forEach calls fn for every rule that matches n, in stylesheet order.
func (rs *ruleSet) forEach(n *html.Node, sc *scratch, fn func(i int, r *rule)) {
	sc.lists = sc.lists[:0]
	add := func(l []uint32) {
		if len(l) == 0 {
			return
		}
		// class="a a" would otherwise queue the same bucket twice. The
		// scan, as the merge's below, is a step per bucket queued: an
		// element of 40,000 classes made both quadratic in them.
		if !rs.budget.Spend(len(sc.lists)) {
			return
		}
		for _, seen := range sc.lists {
			if &seen[0] == &l[0] {
				return
			}
		}
		sc.lists = append(sc.lists, l)
	}

	for _, a := range n.Attr {
		switch a.Key {
		case "id":
			add(rs.byID[a.Val])
		case "class":
			eachClass(a.Val, rs.jsSpaces, func(tok string) { add(rs.byClass[tok]) })
		}
	}
	add(rs.byTag[n.Data])
	add(rs.any)

	if len(sc.lists) == 0 {
		return
	}
	sc.pos = sc.pos[:0]
	for range sc.lists {
		sc.pos = append(sc.pos, 0)
	}
	// Each bucket is ascending by construction and a rule lives in exactly one
	// bucket, so a k-way merge yields every candidate once, in order.
	for {
		best := -1
		var bestV uint32
		for i, l := range sc.lists {
			p := sc.pos[i]
			if int(p) < len(l) && (best < 0 || l[p] < bestV) {
				best, bestV = i, l[p]
			}
		}
		if best < 0 {
			return
		}
		sc.pos[best]++
		if !rs.budget.Spend(len(sc.lists)) {
			return
		}
		r := &rs.rules[bestV]
		if r.match.Match(n) {
			fn(int(bestV), r)
		}
	}
}

// classTokens yields a class attribute's tokens without allocating.
func classTokens(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		i := 0
		for i < len(s) {
			for i < len(s) && isCSSSpace(s[i]) {
				i++
			}
			j := i
			for j < len(s) && !isCSSSpace(s[j]) {
				j++
			}
			if j > i && !yield(s[i:j]) {
				return
			}
			i = j
		}
	}
}

// eachClass calls fn with each class of a class attribute, split as the
// profile's selectors read it: on ASCII whitespace as browsers do, or, with
// jsSpaces, on JavaScript's \s as css-select does, where the no-break space
// and the other Unicode spaces separate classes too. It allocates nothing.
func eachClass(s string, jsSpaces bool, fn func(string)) {
	if !jsSpaces {
		for tok := range classTokens(s) {
			fn(tok)
		}
		return
	}
	start := -1
	for i, r := range s {
		switch {
		case cascadia.IsJSSpace(r):
			if start >= 0 {
				fn(s[start:i])
				start = -1
			}
		case start < 0:
			start = i
		}
	}
	if start >= 0 {
		fn(s[start:])
	}
}

// foldRepeated turns mergeRepeatedSelectors on; a test turns it off to check
// that it changes no output.
var foldRepeated = true

// mergeRepeatedSelectors returns rules with every selector that several
// rules repeat folded into the last of those rules, or rules itself when no
// selector repeats. A selector matches the same elements with the same
// specificity wherever it appears, so the declarations those rules leave on
// any element are known before the document is visited: for each property,
// the one propMap.add keeps among theirs, with the declarations of the same
// rule chained to it. The last rule carries those, and the others are left
// out of the index. Without this, an element matched by n copies of a
// selector adds n declarations to its map, and n such elements cost n×n.
//
// Only a selector that is a group of its own is folded (an :is() expansion
// shares its declarations with its siblings and applies once per group),
// and not one with a pseudo-element, juice 9's throwing declarations, or
// when every declaration is kept (duplicates) or counters are replayed rule
// by rule. The rules given are not modified: an Inliner reuses them.
func mergeRepeatedSelectors(rules []rule) []rule {
	// The candidates, sorted by selector text: one allocation, and nothing
	// more unless a selector repeats. An :is() expansion is a run of rules
	// sharing the group of the first; only a group of one is folded.
	idx := make([]int32, 0, len(rules))
	for i := range rules {
		r := &rules[i]
		single := r.group == int32(i) && (i+1 == len(rules) || rules[i+1].group != int32(i))
		if r.match != nil && r.pseudo == selector.None && !r.throws && single {
			idx = append(idx, int32(i))
		}
	}
	slices.SortStableFunc(idx, func(a, b int32) int { return cmp.Compare(rules[a].sel, rules[b].sel) })
	var out []rule
	var chained map[string]bool
	for start := 0; start < len(idx); {
		end := start + 1
		for end < len(idx) && rules[idx[end]].sel == rules[idx[start]].sel {
			end++
		}
		if end-start > 1 {
			if chained == nil {
				chained = chainedProperties(rules)
			}
			if decls, ok := foldRules(rules, idx[start:end], chained); ok {
				if out == nil {
					out = slices.Clone(rules)
				}
				out[idx[end-1]].decls = decls
				out[idx[end-1]].valueBytes = valueBytes(decls)
				for _, i := range idx[start : end-1] {
					out[i].match = nil
				}
			}
		}
		start = end
	}
	if out == nil {
		return rules
	}
	return out
}

// chainedProperties returns the properties some rule declares more than
// once. propMap.add chains such declarations when the first one leads the
// property when the second comes, which depends on the rules applied before
// that rule: a property no rule repeats is decided by its strongest
// declaration alone, whatever the order of the rules.
func chainedProperties(rules []rule) map[string]bool {
	chained := map[string]bool{}
	seen := map[string]bool{}
	for i := range rules {
		clear(seen)
		for _, d := range rules[i].decls {
			if seen[d.prop] {
				chained[d.prop] = true
			}
			seen[d.prop] = true
		}
	}
	return chained
}

// foldRules returns the declarations the rules of idx, which repeat one
// selector, leave on an element they all match: for each property, the
// strongest one as propMap.add ranks them, in stylesheet order. It returns
// false when one of their properties is chained somewhere (see
// chainedProperties), where the order of the rules matters.
func foldRules(rules []rule, idx []int32, chained map[string]bool) ([]decl, bool) {
	type best struct {
		key uint64
		d   decl
	}
	winner := map[string]*best{}
	var props []string
	for _, i := range idx {
		r := &rules[i]
		for _, d := range r.decls {
			if chained[d.prop] {
				return nil, false
			}
			// As addDecls, which skips these.
			if d.prop == "" || len(d.value) == 0 && !d.keepEmpty {
				continue
			}
			prio := 0
			if d.important {
				prio = 2
			}
			key := packKey(prio, r.spec[0], r.spec[1], r.spec[2])
			cur, ok := winner[d.prop]
			switch {
			case !ok:
				d.from = i + 1
				winner[d.prop] = &best{key, d}
				props = append(props, d.prop)
			case key < cur.key || key == cur.key && d.ord < cur.d.ord:
				// The incumbent strictly outranks it, as in propMap.add.
			default:
				d.from = i + 1
				*cur = best{key, d}
			}
		}
	}
	decls := make([]decl, 0, len(props))
	for _, p := range props {
		decls = append(decls, winner[p].d)
	}
	// In stylesheet order, as the rules gave them.
	slices.SortStableFunc(decls, func(a, b decl) int { return cmp.Compare(a.ord, b.ord) })
	return decls, true
}

// classCounts returns how many elements under root have each class.
func (rs *ruleSet) classCounts(root *html.Node) map[string]int {
	counts := map[string]int{}
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		for _, a := range n.Attr {
			if a.Key != "class" {
				continue
			}
			// A class written twice counts twice: the counts only choose
			// a bucket.
			eachClass(a.Val, rs.jsSpaces, func(tok string) { counts[tok]++ })
		}
	})
	return counts
}
