package juicer

import (
	"strings"
	"testing"
)

// A placeholder's number is read from the document, which may hold the
// placeholder's text itself: a run of digits that overflows int must not
// become a negative index.
func TestPlaceholderNumberOverflow(t *testing.T) {
	doc := "<p>{{ a }} juice_code_block_9223372036854775808_ juice_code_block_18446744073709551616_ juice_code_block_99999999999999999999999_</p>"
	out, err := Inline(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "{{ a }} juice_code_block_9223372036854775808_") {
		t.Errorf("the template tag or the text after it is lost: %s", out)
	}
}

// A Liquid tag never holds "{%": an earlier "{%", such as the "{%22" of a
// URL, is text. Taken as the start of a block, it hid the quote that closes
// the attribute, up to the end of the next Liquid tag.
func TestLiquidBlockStartsAtItsLastOpening(t *testing.T) {
	doc := `<style>p{color:red}</style><a href="https://x.test/?d={%22id%22:5}">link</a><p>Hello {% if vip %}VIP{% endif %}, the end</p>`
	out, err := Inline(doc, CompatibilityMode(Juice10), CodeBlocks(DefaultCodeBlocks()))
	if err != nil {
		t.Fatal(err)
	}
	want := `<a href="https://x.test/?d={%22id%22:5}">link</a><p style="color: red;">Hello {% if vip %}VIP{% endif %}, the end</p>`
	if !strings.Contains(out, want) {
		t.Errorf("got %s\nwant it to hold %s", out, want)
	}
}

// juice's own pairs start at their first opening, as juice does.
func TestJuicePairsStartAtTheirFirstOpening(t *testing.T) {
	src, found := encodeCodeBlocks([]byte("a {{ b {{ c }} d <% e <% f %> g"), DefaultCodeBlocks())
	if got := string(src); got != "a juice_code_block_0_ d juice_code_block_1_ g" {
		t.Errorf("encoded as %q", got)
	}
	if len(found) != 2 || string(found[0]) != "{{ b {{ c }}" || string(found[1]) != "<% e <% f %>" {
		t.Errorf("blocks %q", found)
	}
}
