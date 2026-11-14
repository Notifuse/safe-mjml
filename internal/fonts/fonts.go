// Package fonts decides which web fonts a rendered document imports, as
// mjml-core's buildFontsTags does.
package fonts

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Notifuse/safe-mjml/internal/html"
)

// DefaultFontStack is the default font family used by MJML components.
const DefaultFontStack = "Ubuntu, Helvetica, Arial, sans-serif"

// Declaration is a font declared by an mj-font tag.
type Declaration struct {
	Name string
	Href string
}

// font is an entry of the font registry.
type font struct {
	name, lower, href string
}

// defaultFonts is mjml-core's default font registry, in its order.
var defaultFonts = []font{
	{"Open Sans", "open sans", "https://fonts.googleapis.com/css?family=Open+Sans:300,400,500,700"},
	{"Droid Sans", "droid sans", "https://fonts.googleapis.com/css?family=Droid+Sans:300,400,500,700"},
	{"Lato", "lato", "https://fonts.googleapis.com/css?family=Lato:300,400,500,700"},
	{"Roboto", "roboto", "https://fonts.googleapis.com/css?family=Roboto:300,400,500,700"},
	{"Ubuntu", "ubuntu", "https://fonts.googleapis.com/css?family=Ubuntu:300,400,500,700"},
}

// ToImport returns the stylesheet URLs a document imports, as mjml-core's
// buildFontsTags does: the default fonts, then the mj-font declarations (a
// declaration replaces the URL of a font with exactly its name, in place, and
// adds the others at the end, in the order they are first declared), each
// imported when the body or the inline styles (mj-style inline="inline")
// name it in a font-family.
//
// mjml-core tests the body with /"[^"]*font-family:[^"]*NAME[^"]*"/gi: NAME
// after a font-family: within the text between two double quotes, and the
// inline styles with /font-family:[^;}]*NAME/gi. The same tests are made
// here in time linear in the size of the input, whatever it holds: each
// text between two quotes (or between ; and } in an inline style) is
// searched from its first font-family: only, which covers the later ones,
// and every font name is looked for in one pass (familyMatcher). Names are
// literal, and compared in lower case.
//
// The body is not copied: font-family: is found in any case where it is
// (familyFinder), and only the texts after it are compared in lower case,
// which most renders spend little time on.
func ToImport(body string, inlineStyles []string, declarations []Declaration) []string {
	registry, matcher := defaultFonts, defaultMatcher()
	if len(declarations) > 0 {
		registry = buildRegistry(declarations)
		names := make([]string, len(registry))
		for i, f := range registry {
			names[i] = f.lower
		}
		matcher = newFamilyMatcher(names)
	}
	// Each tail is read by the matcher as it is found: none is kept.
	search := matcher.search()
	eachTail(body, true, search.read)
	for _, style := range inlineStyles {
		eachTail(style, false, search.read)
	}
	found := search.found()
	var urls []string
	for i, f := range registry {
		if found[i] {
			urls = append(urls, f.href)
		}
	}
	return urls
}

// defaultMatcher is the matcher of the default fonts, built once: most
// documents declare no font.
var defaultMatcher = sync.OnceValue(func() *familyMatcher {
	names := make([]string, len(defaultFonts))
	for i, f := range defaultFonts {
		names[i] = f.lower
	}
	return newFamilyMatcher(names)
})

// buildRegistry returns the default fonts with the declarations applied, in
// time linear in their number.
func buildRegistry(declarations []Declaration) []font {
	if len(declarations) == 0 {
		return defaultFonts
	}
	registry := append([]font(nil), defaultFonts...)
	index := make(map[string]int, len(registry)+len(declarations))
	for i, f := range registry {
		index[f.name] = i
	}
	for _, d := range declarations {
		if i, ok := index[d.Name]; ok {
			registry[i].href = d.Href
			continue
		}
		index[d.Name] = len(registry)
		registry = append(registry, font{d.Name, strings.ToLower(d.Name), d.Href})
	}
	return registry
}

const fontFamily = "font-family:"

// familyFinder finds the occurrences of font-family:, in any case, in a
// text it does not copy. It looks for "y:" and "Y:" and checks the bytes
// before them, in time linear in the text over all its calls: the next
// position of each is searched for again only once the calls have gone past
// it. As the text in lower case would (strings.ToLower), it matches only
// ASCII letters: no other character lowers to one of "font-family".
type familyFinder struct {
	s            string
	lower, upper int // the next "y:" and "Y:"; -1 for none left, -2 not searched yet
}

func newFamilyFinder(s string) familyFinder { return familyFinder{s: s, lower: -2, upper: -2} }

// next returns the index of the first font-family: at or after from, or -1.
func (f *familyFinder) next(from int) int {
	for {
		lower := f.search(&f.lower, "y:", from)
		upper := f.search(&f.upper, "Y:", from)
		y := lower
		if y < 0 || upper >= 0 && upper < y {
			y = upper
		}
		if y < 0 {
			return -1
		}
		// y is the index of the y of "font-family:": the name starts
		// len("font-famil") bytes before it.
		if start := y - (len(fontFamily) - 2); start >= from && strings.EqualFold(f.s[start:y], fontFamily[:len(fontFamily)-2]) {
			return start
		}
		from = y + 1
	}
}

// search returns the next occurrence of needle at or after from, cached in
// at.
func (f *familyFinder) search(at *int, needle string, from int) int {
	if *at == -1 || *at >= from {
		return *at
	}
	if from > len(f.s) {
		*at = -1 // from only grows: none is left
	} else if i := strings.Index(f.s[from:], needle); i >= 0 {
		*at = from + i
	} else {
		*at = -1
	}
	return *at
}

// eachTail calls read with, for each run of s where a font-family: may
// apply, the text after its first font-family:, in the case of s. With
// quoted, a run is the text between two consecutive double quotes (one
// without a closing quote does not count), as mjml-core's pattern for the
// body; otherwise it is the text between ; or } (or the ends of s), as its
// pattern for inline styles.
func eachTail(s string, quoted bool, read func(tail string)) {
	finder := newFamilyFinder(s)
	from := 0
	if quoted {
		// Runs start after the first quote.
		from = strings.IndexByte(s, '"') + 1
		if from == 0 {
			return
		}
	}
	for {
		// The run of the next font-family: is the one it is in: none of
		// its characters is a quote or a stop.
		at := finder.next(from)
		if at < 0 {
			return
		}
		start := at + len(fontFamily)
		var end int
		if quoted {
			i := strings.IndexByte(s[start:], '"')
			if i < 0 {
				return // no closing quote
			}
			end = start + i
		} else if i := strings.IndexAny(s[start:], ";}"); i >= 0 {
			end = start + i
		} else {
			end = len(s)
		}
		read(s[start:end])
		// The next run starts after this one's end: its later
		// font-family: are covered by the first.
		from = end + 1
	}
}

// familyMatcher finds which of a set of names occur in texts, with an
// Aho-Corasick automaton: one pass over the texts, whatever the number and
// the length of the names.
type familyMatcher struct {
	next     map[uint64]int32 // state<<8 | byte → state
	fail     []int32
	order    []int32 // the states in breadth-first order
	terminal []int32 // the state of each name
	empty    []int   // the names that are empty, found in any text
}

func newFamilyMatcher(names []string) *familyMatcher {
	m := &familyMatcher{next: map[uint64]int32{}, fail: []int32{0}, terminal: make([]int32, len(names))}
	for i, name := range names {
		if name == "" {
			m.empty = append(m.empty, i)
		}
		state := int32(0)
		for j := 0; j < len(name); j++ {
			key := uint64(state)<<8 | uint64(name[j])
			to, ok := m.next[key]
			if !ok {
				to = int32(len(m.fail))
				m.fail = append(m.fail, 0)
				m.next[key] = to
			}
			state = to
		}
		m.terminal[i] = state
	}
	// The failure links, breadth first; each state's children are found
	// from the transitions, grouped by state once.
	children := make([][]uint64, len(m.fail))
	for key := range m.next {
		children[key>>8] = append(children[key>>8], key)
	}
	m.order = []int32{0}
	for head := 0; head < len(m.order); head++ {
		state := m.order[head]
		for _, key := range children[state] {
			child := m.next[key]
			c := byte(key)
			if state != 0 {
				m.fail[child] = m.step(m.fail[state], c)
			}
			m.order = append(m.order, child)
		}
	}
	return m
}

// step returns the state after reading c in state.
func (m *familyMatcher) step(state int32, c byte) int32 {
	for {
		if to, ok := m.next[uint64(state)<<8|uint64(c)]; ok {
			return to
		}
		if state == 0 {
			return 0
		}
		state = m.fail[state]
	}
}

// familySearch is a search of the names of a familyMatcher in texts read
// one after the other.
type familySearch struct {
	m       *familyMatcher
	reached []bool // the states reached
	anyText bool   // whether a text was read
}

func (m *familyMatcher) search() *familySearch {
	return &familySearch{m: m, reached: make([]bool, len(m.fail))}
}

// read reads a text, in lower case as strings.ToLower writes it: ASCII
// letters are folded here, and a text with other characters is lowered
// whole.
func (s *familySearch) read(text string) {
	s.anyText = true
	if !isASCII(text) {
		text = strings.ToLower(text)
	}
	state := int32(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		state = s.m.step(state, c)
		s.reached[state] = true
	}
}

// found reports, for each name, whether it occurs in one of the texts read.
func (s *familySearch) found() []bool {
	m := s.m
	// A state reached means the names that end its text, along the failure
	// links, occur too: propagate from the deepest states up.
	for i := len(m.order) - 1; i > 0; i-- {
		if state := m.order[i]; s.reached[state] {
			s.reached[m.fail[state]] = true
		}
	}
	found := make([]bool, len(m.terminal))
	for i, state := range m.terminal {
		found[i] = state != 0 && s.reached[state]
	}
	for _, i := range m.empty {
		found[i] = s.anyText
	}
	return found
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// BuildFontsTags generates HTML for font imports (similar to MJML.io's buildFontsTags)
func BuildFontsTags(fontsToImport []string) string {
	if len(fontsToImport) == 0 {
		return ""
	}

	var result strings.Builder

	// Generate conditional comment for non-Outlook clients (MJML's exact format)
	result.WriteString("<!--[if !mso]><!-->")

	// Generate <link> tags (no newlines between elements)
	for _, url := range fontsToImport {
		result.WriteString(`<link href="` + html.EscapeAttribute(url) + `" rel="stylesheet" type="text/css">`)
	}

	// Generate <style> tag with @import statements, one per line as MJML joins
	// them (the minifier keeps the newlines inside <style>)
	result.WriteString("<style type=\"text/css\">")
	for i, url := range fontsToImport {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("@import url(%s);", url))
	}
	result.WriteString("</style>")

	result.WriteString("<!--<![endif]-->")

	return result.String()
}
