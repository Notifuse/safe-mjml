package mjml

import "testing"

// TestTextWhitespaceIsComparedAsDisplayed checks the comparison of text with
// MJML: whitespace collapsed in text nodes, as html-minifier does in the
// minified references and as a client displays it, but kept as written where
// it is displayed (<pre>, <textarea>, white-space:pre*), and every other
// difference in the text still found.
func TestTextWhitespaceIsComparedAsDisplayed(t *testing.T) {
	doc := func(title, body string) string {
		return "<!doctype html><html><head><title>" + title + "</title></head><body>" + body + "</body></html>"
	}
	same := []struct{ name, expected, actual string }{
		{"collapsed text", doc("T", "<div>A text on two lines</div>"), doc("T", "<div>A   text\n   on  two\tlines</div>")},
		{"collapsed title", doc("My newsletter of November", ""), doc("My   newsletter\n  of  November", "")},
		{"collapsed around an element", doc("T", "<div>A <b>bold</b> word</div>"), doc("T", "<div>A   <b>bold</b>\n word</div>")},
		{"pre as written", doc("T", "<pre>a   b</pre>"), doc("T", "<pre>a   b</pre>")},
		{"no whitespace at a line break", doc("T", "<div>A line<br>Another</div>"), doc("T", "<div>A line\n   <br>\n   Another</div>")},
	}
	for _, c := range same {
		if d := referenceDifferences(c.expected, c.actual); len(d) > 0 {
			t.Errorf("%s: differences reported:\n%v", c.name, d)
		}
	}
	// The mutations the comparison must still catch.
	different := []struct{ name, expected, actual string }{
		{"a missing word", doc("T", "<div>A text on two lines</div>"), doc("T", "<div>A text on lines</div>")},
		{"a missing word in the title", doc("My newsletter", ""), doc("My", "")},
		{"another word after a line break", doc("T", "<div>A line<br>Another</div>"), doc("T", "<div>A line<br>Other</div>")},
		{"spaces in a pre", doc("T", "<pre>a   b</pre>"), doc("T", "<pre>a b</pre>")},
		{"spaces in a textarea", doc("T", "<div><textarea>a   b</textarea></div>"), doc("T", "<div><textarea>a b</textarea></div>")},
		{"spaces under white-space:pre", doc("T", `<div style="white-space:pre">a   b</div>`), doc("T", `<div style="white-space:pre">a b</div>`)},
		{"spaces under an ancestor's white-space:pre-wrap", doc("T", `<div style="color:red; white-space: pre-wrap"><span>a   b</span></div>`), doc("T", `<div style="color:red; white-space: pre-wrap"><span>a b</span></div>`)},
	}
	for _, c := range different {
		if d := referenceDifferences(c.expected, c.actual); len(d) == 0 {
			t.Errorf("%s: no difference reported", c.name)
		}
	}
}
