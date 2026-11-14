package juicer

import (
	"bytes"
	"slices"

	"github.com/Notifuse/safe-mjml/internal/cascadia"
)

// Template expressions are swapped for inert placeholders before parsing and
// restored after serializing, so the HTML parser never sees them. Without
// this, template syntax in tag position is mangled by attribute scanning and
// quotes inside a value get escaped.
//
// juice ships the same mechanism but its default set is Handlebars and EJS
// only. Liquid's statement tag {% ... %} is absent, so juice corrupts
// `<td {% if x %}class="a"{% endif %}>` into `<td {% if x %}class="a" endif %}>`.
// We add it, which is a deliberate divergence from the reference.

// CodeBlock is a pair of template delimiters whose contents are left untouched.
type CodeBlock struct {
	Start, End string
	// Innermost starts a block at the last Start before its End: a
	// template tag of this kind never holds its own Start, so an earlier
	// one is text, such as the "{%22" of a URL. juice's own pairs, {{ }}
	// and <% %>, start at the first, as juice does.
	Innermost bool
}

// DefaultCodeBlocks returns a fresh copy of the delimiters protected by default.
func DefaultCodeBlocks() []CodeBlock {
	return []CodeBlock{
		{Start: "{{", End: "}}"},                  // Handlebars, Liquid output, Go templates
		{Start: "{%", End: "%}", Innermost: true}, // Liquid statements, Jinja, Nunjucks
		{Start: "<%", End: "%>"},                  // EJS, ERB
	}
}

const placeholderPrefix = "juice_code_block_"

// encodeCodeBlocks replaces every template expression with a placeholder made
// only of identifier characters, so it survives tokenizing in text, attribute
// value and tag position alike. The placeholder is lowercase because attribute
// names are lowercased during parsing.
func encodeCodeBlocks(src []byte, blocks []CodeBlock) ([]byte, [][]byte) {
	if len(blocks) == 0 {
		return src, nil
	}
	var found [][]byte
	var out []byte
	i := 0
	// A delimiter pair that is left open once is open for good: no later
	// opening can find a close either, and one whose opening no longer
	// occurs is done too. Other pairs still apply.
	open := slices.Clone(blocks)
	// next[k] is where open[k] next starts, at or after i once refreshed, so
	// each delimiter's search moves forward through src only once.
	next := make([]int, len(open))
	for k := range next {
		next[k] = -1
	}
	for i < len(src) {
		// Earliest opening delimiter wins, so nested-looking syntax is taken
		// in source order rather than delimiter order.
		bestBlock := -1
		for k := 0; k < len(open); k++ {
			if next[k] < i {
				j := bytes.Index(src[i:], []byte(open[k].Start))
				if j < 0 {
					open, next = slices.Delete(open, k, k+1), slices.Delete(next, k, k+1)
					k--
					continue
				}
				next[k] = i + j
			}
			if bestBlock < 0 || next[k] < next[bestBlock] {
				bestBlock = k
			}
		}
		if bestBlock < 0 {
			break
		}
		b := open[bestBlock]
		start := next[bestBlock]
		// The close comes after the whole opening, so "<%>" is not a block.
		close := bytes.Index(src[start+len(b.Start):], []byte(b.End))
		if close < 0 {
			open, next = slices.Delete(open, bestBlock, bestBlock+1), slices.Delete(next, bestBlock, bestBlock+1)
			continue
		}
		end := start + len(b.Start) + close + len(b.End)
		if b.Innermost {
			// A "{%" in a URL (?d={%22id%22:5}) would otherwise take the
			// quote that closes its attribute into the block, up to the end
			// of the next Liquid tag, and the HTML parser would read the
			// rest of the email into that attribute.
			if k := bytes.LastIndex(src[start+len(b.Start):end-len(b.End)], []byte(b.Start)); k >= 0 {
				start += len(b.Start) + k
			}
		}
		if out == nil {
			out = make([]byte, 0, len(src))
		}
		out = append(out, src[i:start]...)
		out = append(out, placeholder(len(found))...)
		found = append(found, src[start:end])
		i = end
	}
	if out == nil {
		return src, nil
	}
	return append(out, src[i:]...), found
}

func placeholder(n int) []byte {
	b := make([]byte, 0, len(placeholderPrefix)+4)
	b = append(b, placeholderPrefix...)
	b = appendInt(b, n)
	// The trailing underscore stops placeholder 1 matching inside placeholder 10.
	return append(b, '_')
}

func appendInt(b []byte, n int) []byte {
	if n >= 10 {
		b = appendInt(b, n/10)
	}
	return append(b, byte('0'+n%10))
}

// decodeCodeBlocks restores the original template expressions. An empty
// attribute serializes bare, so a placeholder that landed in tag position has
// no ="" suffix to strip, but we tolerate one in case the value was preserved.
//
// A placeholder is put back wherever its text stands, as juice does, so a
// rule or a CSS variable that copies it writes the block as many times: each
// copy after the first costs its bytes in budget, as a declaration's value
// does, or 104 KB of template wrote 2 GB. The copies are counted before the
// output is built, and it reports false when the budget runs out.
func decodeCodeBlocks(out []byte, found [][]byte, budget *cascadia.Budget) ([]byte, bool) {
	if len(found) == 0 {
		return out, true
	}
	written := make([]bool, len(found))
	extra := 0
	eachPlaceholder(out, len(found), func(start, end, n int) {
		if written[n] {
			extra += len(found[n])
		}
		written[n] = true
	})
	if extra > 0 && !budget.Spend(valueSteps(extra)) {
		return nil, false
	}
	var buf []byte
	i := 0
	eachPlaceholder(out, len(found), func(start, end, n int) {
		if buf == nil {
			buf = make([]byte, 0, len(out)+extra)
		}
		buf = append(buf, out[i:start]...)
		buf = append(buf, found[n]...)
		i = end
	})
	if buf == nil {
		return out, true
	}
	return append(buf, out[i:]...), true
}

// eachPlaceholder calls f with the span of each placeholder in out, in
// order, and its number, below count.
func eachPlaceholder(out []byte, count int, f func(start, end, n int)) {
	i := 0
	for {
		j := bytes.Index(out[i:], []byte(placeholderPrefix))
		if j < 0 {
			return
		}
		start := i + j
		k := start + len(placeholderPrefix)
		n := 0
		digits := 0
		for k < len(out) && out[k] >= '0' && out[k] <= '9' {
			// Saturated at count, so that a long run of digits cannot
			// overflow into a negative index.
			n = min(n*10+int(out[k]-'0'), count)
			k++
			digits++
		}
		if digits == 0 || k >= len(out) || out[k] != '_' || n >= count {
			// Not one of ours; skip past the prefix and keep looking.
			i = start + len(placeholderPrefix)
			continue
		}
		k++
		if bytes.HasPrefix(out[k:], []byte(`=""`)) {
			k += 3
		}
		f(start, k, n)
		i = k
	}
}
