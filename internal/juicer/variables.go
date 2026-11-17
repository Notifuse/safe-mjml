package juicer

import (
	"bytes"

	"github.com/Notifuse/safe-mjml/internal/juicer/selector"
	"golang.org/x/net/html"
)

// CSS custom property resolution.
//
// Elements are visited in document order, so an ancestor's variables are
// always substituted before a descendant reads them. juice instead walks its
// styleProps in accumulation order (rule order crossed with document order)
// and can therefore read a value its own later pass would have changed; that
// order dependence is a bug we do not reproduce.
//
// One juice behaviour is worth keeping and falls out for free: only elements
// some rule matched have a property map, so a custom property declared on an
// untouched element is invisible.

func (in *pass) resolveVariables() {
	for _, el := range in.resolved {
		for i := range el.props.slots {
			p := &el.props.slots[i]
			if bytes.HasPrefix([]byte(p.prop), []byte("--")) {
				continue
			}
			if at, _ := findVar(p.value, 0, in.o.spacedVar); at < 0 {
				continue
			}
			budget := maxVarSubstitutions
			p.value = in.substitute(el.node, p.value, 0, &budget)
		}
	}
}

const maxVarDepth = 16

// maxVarSubstitutions bounds the work for one value. Depth alone does not: a
// property referring to itself twice doubles at every level. juice never
// finishes such a value.
const maxVarSubstitutions = 256

// substitute replaces every var() reference in v. An unresolvable reference
// with no fallback is left in place, as juice leaves it.
func (in *pass) substitute(n *html.Node, v []byte, depth int, budget *int) []byte {
	if depth > maxVarDepth || *budget <= 0 {
		return v
	}
	out := v
	// from is where the next scan starts, so a reference we leave in place
	// cannot be found again on the following pass.
	from := 0
	for from < len(out) {
		at, open := findVar(out, from, in.o.spacedVar)
		if at < 0 || *budget <= 0 {
			return out
		}
		end := selector.ParenEnd(out, open)
		// ParenEnd falls back to the end of input when the parenthesis is
		// never closed, which is not a usable span.
		if end < open+2 || out[end-1] != ')' {
			return out
		}
		name, fallback := splitVarArgs(out[open+1 : end-1])

		*budget--
		var with []byte
		switch val, ok := in.lookupVar(n, string(bytes.TrimSpace(name))); {
		case ok:
			with = val
		case len(bytes.TrimSpace(fallback)) > 0:
			// juice tests the fallback for truth: var(--x,) stays as written.
			with = bytes.TrimSpace(fallback)
		default:
			from = end
			continue
		}
		// "$`", "$'" and "$&" copy the value around the call: N of them
		// before a tail of T bytes make N times T bytes, and "$&" in a
		// fallback holds the call again, which doubles at every level. The
		// copy is paid before it is made.
		size := jsReplacementLen(with, out, at, end)
		if size > len(with) && !in.o.budget.Spend(valueSteps(size)) {
			return out
		}
		repl := in.substitute(n, jsReplacement(with, out, at, end), depth+1, budget)

		// The copy, and the output it makes: a value of 256 references to
		// a long custom property is 256 times its length.
		if !in.o.budget.Spend(valueSteps(len(out) - (end - at) + len(repl))) {
			return out
		}
		merged := make([]byte, 0, len(out)-(end-at)+len(repl))
		merged = append(merged, out[:at]...)
		merged = append(merged, repl...)
		merged = append(merged, out[end:]...)
		out = merged
		from = at + len(repl)
	}
	return out
}

// lookupVar finds a custom property on the element or its nearest ancestor
// that a rule matched.
func (in *pass) lookupVar(n *html.Node, name string) ([]byte, bool) {
	for e := n; e != nil; e = e.Parent {
		// Each ancestor is a step: a reference on an element 1,900 deep
		// walked them all, 256 times a value.
		if !in.o.budget.Spend(1) {
			return nil, false
		}
		m, ok := in.byNode[e]
		if !ok {
			continue
		}
		// Finding the name compares it with the element's custom
		// properties: a name of 120 KB, looked up from 1,900 elements deep,
		// read 228 GB for 228 million steps.
		if !in.o.budget.Spend(len(name) / bytesPerStep) {
			return nil, false
		}
		i := m.find(name)
		if m.dup && !in.o.headVars {
			i = m.winner(name)
		}
		if i >= 0 {
			return m.slots[i].value, true
		}
	}
	return nil, false
}

// findVar locates the next var() call whose "(" is at or after from,
// returning where its name starts and the index of its "(", or -1. Like
// juice, the name is the run of letters and hyphens before the parenthesis,
// matched case-insensitively, so `somevar(` is not a var() call. The run
// reaches before from: a value substituted just before, such as 1px in
// var(--x)var(--y), makes it `pxvar(`, which juice leaves as written. juice
// also allows whitespace before the parenthesis, reading `VAR (` as a call,
// where CSS does not; spaced allows it.
func findVar(v []byte, from int, spaced bool) (at, open int) {
	for i := from; i < len(v); i++ {
		if v[i] != '(' {
			continue
		}
		e := i
		for spaced && e > 0 && isCSSSpace(v[e-1]) {
			e--
		}
		s := e
		for s > 0 && (v[s-1] == '-' || v[s-1]|0x20 >= 'a' && v[s-1]|0x20 <= 'z') {
			s--
		}
		if bytes.EqualFold(v[s:e], []byte("var")) {
			return s, i
		}
	}
	return -1, -1
}

// splitVarArgs divides a var() argument list into the name and its optional
// fallback, at the first top-level comma.
func splitVarArgs(arg []byte) (name, fallback []byte) {
	depth := 0
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				return arg[:i], arg[i+1:]
			}
		}
	}
	return arg, nil
}

// jsReplacementLen is the length of jsReplacement(repl, v, at, end),
// computed without building it.
func jsReplacementLen(repl, v []byte, at, end int) int {
	if bytes.IndexByte(repl, '$') < 0 {
		return len(repl)
	}
	n := 0
	for i := 0; i < len(repl); i++ {
		if repl[i] != '$' || i+1 == len(repl) {
			n++
			continue
		}
		switch repl[i+1] {
		case '$':
			n++
		case '&':
			n += end - at
		case '`':
			n += at
		case '\'':
			n += len(v) - end
		default:
			n++
			continue
		}
		i++
	}
	return n
}

// jsReplacement is repl as JavaScript's String.prototype.replace reads it,
// which juice calls with the value of a variable: "$$" is "$", "$&" the call
// replaced (v[at:end]), "$`" what comes before it and "$'" what follows.
// Other dollars are themselves.
func jsReplacement(repl, v []byte, at, end int) []byte {
	if bytes.IndexByte(repl, '$') < 0 {
		return repl
	}
	out := make([]byte, 0, jsReplacementLen(repl, v, at, end))
	for i := 0; i < len(repl); i++ {
		if repl[i] != '$' || i+1 == len(repl) {
			out = append(out, repl[i])
			continue
		}
		switch repl[i+1] {
		case '$':
			out = append(out, '$')
		case '&':
			out = append(out, v[at:end]...)
		case '`':
			out = append(out, v[:at]...)
		case '\'':
			out = append(out, v[end:]...)
		default:
			out = append(out, '$')
			continue
		}
		i++
	}
	return out
}
