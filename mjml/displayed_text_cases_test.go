package mjml

import "testing"

// TestDisplayedTextIsComparedAsAClientShowsIt checks that the comparison
// with MJML fails on a difference a client shows, and passes on whitespace
// a client does not show.
func TestDisplayedTextIsComparedAsAClientShowsIt(t *testing.T) {
	page := func(body string) string {
		return `<!doctype html><html><head><title>T</title></head><body>` + body + `</body></html>`
	}
	differ := []struct{ name, expected, actual string }{
		{"space before an inline element", `<div>Hello <b>world</b></div>`, `<div>Hello<b>world</b></div>`},
		{"space after an inline element", `<div><a href="#">A</a> B</div>`, `<div><a href="#">A</a>B</div>`},
		{"word moved across an inline element", `<div>Hello <b>bold</b> world</div>`, `<div>Hello world <b>bold</b></div>`},
		{"spaces of a pre", `<pre>  a  </pre>`, `<pre>a</pre>`},
		{"spaces inside a pre", `<pre><span>A</span>   <span>B</span></pre>`, `<pre><span>A</span><span>B</span></pre>`},
		{"spaces of a textarea", `<textarea>  a  </textarea>`, `<textarea>a</textarea>`},
		{"white-space:pre", `<div style="white-space:pre">a  b</div>`, `<div style="white-space:pre">a b</div>`},
		{"a word lost", `<div>A text on</div>`, `<div>A on</div>`},
		{"the title", `<div>x</div>`, `<div>x</div>`},
	}
	for _, c := range differ {
		expected, actual := page(c.expected), page(c.actual)
		if c.name == "the title" {
			actual = `<!doctype html><html><head><title>U</title></head><body>` + c.actual + `</body></html>`
		}
		if displayedTextDifference(expected, actual) == "" {
			t.Errorf("%s: %q and %q compare as equal", c.name, c.expected, c.actual)
		}
	}
	same := []struct{ name, expected, actual string }{
		{"collapsed run", `<div>A text on</div>`, `<div>A   text
 on</div>`},
		{"edge of a block", `<div>A</div>`, `<div>  A  </div>`},
		{"around a br", `<div>A<br>B</div>`, `<div>A <br> B</div>`},
		{"merged across an inline element", `<div>A <b>B</b></div>`, `<div>A <b> B</b></div>`},
		{"before a block", `<div><a href="#">A</a> <div>B</div></div>`, `<div><a href="#">A</a><div>B</div></div>`},
	}
	for _, c := range same {
		if d := displayedTextDifference(page(c.expected), page(c.actual)); d != "" {
			t.Errorf("%s: %q and %q differ:\n%s", c.name, c.expected, c.actual, d)
		}
	}
}

// TestPreambleIsCompared checks that the comparison with MJML sees a
// missing or changed doctype, which the DOM comparison drops.
func TestPreambleIsCompared(t *testing.T) {
	body := `<html><head><title>T</title></head><body><div>x</div></body></html>`
	if preambleDifference("<!doctype html>"+body, body) == "" {
		t.Error("a missing doctype compares as equal")
	}
	if preambleDifference("<!doctype html>"+body, "<!DOCTYPE html PUBLIC \"x\">"+body) == "" {
		t.Error("another doctype compares as equal")
	}
	if d := preambleDifference("<!doctype html>"+body, "<!doctype html>\n"+body); d != "" {
		t.Errorf("whitespace after the doctype differs: %s", d)
	}
}

func TestVoidElementSlashIsNotCompared(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	for _, written := range []string{"<br/>", "<br />"} {
		if d := referenceDifferences(page("<div>one<br>two</div>"), page("<div>one"+written+"two</div>")); len(d) > 0 {
			t.Errorf("<br> and %s compare as different: %v", written, d)
		}
	}
	// In SVG a self-closing tag ends the element; without the slash the next
	// elements nest in it, which changes the tree.
	if d := referenceDifferences(page(`<svg><path d="M0"/><g></g></svg>`), page(`<svg><path d="M0"><g></g></path></svg>`)); len(d) == 0 {
		t.Error("a self-closing <path/> and a <path> that contains the next element compare as equal")
	}
}

func TestWhitespaceNextToABlockIsNotCompared(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	if d := referenceDifferences(page("<div>a<div>x</div> b</div>"), page("<div>a<div>x</div>b</div>")); len(d) > 0 {
		t.Errorf("a space after a block compares as different: %v", d)
	}
	if compareDOMTrees(page("<div>a<span>x</span> b</div>"), page("<div>a<span>x</span>b</div>")) {
		t.Error("a space after an inline element compares as equal in the DOM")
	}
}

func TestAWordMovedAcrossABlockIsADifference(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	if compareDOMTrees(page("<div>ab<div>x</div></div>"), page("<div>a<div>x</div>b</div>")) {
		t.Error("a word moved after a block child compares as equal in the DOM")
	}
}

func TestWhitespaceNextToAConditionalCommentIsNotCompared(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	revealed := "<!--[if mso]>--><div>mso</div><!--<![endif]-->"
	spaced := "<!--[if mso]>--> <div>mso</div> <!--<![endif]-->"
	if d := referenceDifferences(page(revealed), page(spaced)); len(d) > 0 {
		t.Errorf("whitespace next to the delimiters of a conditional compares as different: %v", d)
	}
	// Between two inline elements inside a conditional, a space is displayed.
	if d := checkMSOConditionalDifferences(page("<!--[if mso]><span>a</span> <span>b</span><![endif]-->"), page("<!--[if mso]><span>a</span><span>b</span><![endif]-->")); d == "" {
		t.Error("a space between two spans of a conditional compares as equal")
	}
}

// In a conditional block, which is compared as written once its tags'
// attributes are sorted, an attribute written bare is the empty value it
// means in HTML: MJML writes empty attributes bare once juice has run. Any
// other value, and a missing attribute, still differ.
func TestBareAttributeIsEmptyInAConditionalBlock(t *testing.T) {
	page := func(img string) string {
		return "<!doctype html><html><head><title>T</title></head><body><!--[if !mso]><!--><div>" + img + "</div><!--<![endif]--></body></html>"
	}
	if d := checkMSOConditionalDifferences(page(`<img alt src="a.png" width="1">`), page(`<img src="a.png" alt="" width="1">`)); d != "" {
		t.Errorf("alt and alt=\"\" compare as different: %s", d)
	}
	for _, other := range []string{`<img src="a.png" alt="x" width="1">`, `<img src="a.png" width="1">`, `<img alt src="a.png" width="2">`} {
		if d := checkMSOConditionalDifferences(page(`<img alt src="a.png" width="1">`), page(other)); d == "" {
			t.Errorf("<img alt src=\"a.png\" width=\"1\"> and %s compare as equal", other)
		}
	}
}

// A comment is not displayed: the text on either side of it is compared
// joined, as a client shows it. html-minifier, which minifies the
// references, moves the whitespace next to a comment across it ("A
// <!--x-->B" becomes "A<!--x--> B"), which is the same displayed text. A
// word moved across the comment so that the text reads differently, or a
// word lost, is still a difference.
func TestWhitespaceAcrossACommentIsNotCompared(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	for _, pair := range [][2]string{
		{"<div>A <!--x-->B</div>", "<div>A<!--x--> B</div>"},
		{"<div>A <!--[CDATA[ c ]]-->B <!-- d --> D</div>", "<div>A<!--[CDATA[ c ]]--> B<!-- d --> D</div>"},
		{"<p>Go <!--x--> now</p>", "<p>Go<!--x--> now</p>"},
		{"<div>A <!--x--><!--y-->B</div>", "<div>A<!--x--><!--y--> B</div>"},
	} {
		if d := displayedTextDifference(page(pair[0]), page(pair[1])); d != "" {
			t.Errorf("%s and %s compare as different: %s", pair[0], pair[1], d)
		}
		if d := referenceDifferences(page(pair[0]), page(pair[1])); len(d) > 0 {
			t.Errorf("%s and %s: reference differences %v", pair[0], pair[1], d)
		}
	}
	for _, pair := range [][2]string{
		{"<div>A<!--x-->B C</div>", "<div>A B<!--x-->C</div>"}, // "AB C" and "A BC"
		{"<div>A <!--x-->B</div>", "<div>A <!--x--></div>"},    // B lost
		{"<div>A <!--x-->B</div>", "<div>A <!--x B--></div>"},  // B in the comment
		{"<div>A <!--x--><b>B</b></div>", "<div>A<!--x--><b>B</b></div>"},
	} {
		if d := displayedTextDifference(page(pair[0]), page(pair[1])); d == "" {
			t.Errorf("%s and %s compare as equal", pair[0], pair[1])
		}
	}
}

// TestSpaceNextToAnImageIsCompared checks that a space next to an inline
// element that displays something (an image, a form control) is compared,
// as a client shows it between two images, while a space at the edge of a
// block, or next to an empty inline element, is still not. A review found
// that removing the space between two images passed the comparison.
func TestSpaceNextToAnImageIsCompared(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	for _, c := range [][2]string{
		{`<div><img src="a"> <img src="b"></div>`, `<div><img src="a"><img src="b"></div>`},
		{`<div>A <img src="a"></div>`, `<div>A<img src="a"></div>`},
		{`<div><img src="a"> B</div>`, `<div><img src="a">B</div>`},
		{`<div><input> <button>Go</button></div>`, `<div><input><button>Go</button></div>`},
	} {
		if displayedTextDifference(page(c[0]), page(c[1])) == "" {
			t.Errorf("%s and %s compare as equal", c[0], c[1])
		}
	}
	for _, c := range [][2]string{
		{`<div> <img src="a"></div>`, `<div><img src="a"></div>`},
		{`<div><img src="a"> </div>`, `<div><img src="a"></div>`},
		{`<div><span></span> <span></span>A</div>`, `<div><span></span><span></span>A</div>`},
	} {
		if d := displayedTextDifference(page(c[0]), page(c[1])); d != "" {
			t.Errorf("%s and %s compare as different: %s", c[0], c[1], d)
		}
	}
}

// TestCloseTagWithoutItsEndIsADifference checks that a close tag that meets
// a "<" before its ">" (">Hi</mj-text</div>"), which an HTML parser reads
// with the next tag as one close tag, differs from a document without it.
// A review found that ">Hi</mj-text</div>" compared equal to ">Hi</div>".
// A "<" that opens no tag (a space after it) is still text.
func TestCloseTagWithoutItsEndIsADifference(t *testing.T) {
	page := func(body string) string {
		return "<!doctype html><html><head><title>T</title></head><body>" + body + "</body></html>"
	}
	// The </td> after it closes the div anyway, so the DOM is the same.
	cell := func(content string) string {
		return page(`<table><tbody><tr><td><div style="color:red;">` + content + `</div></td></tr></tbody></table>`)
	}
	for _, leftover := range []string{"</mj-text", "</b"} {
		if d := referenceDifferences(cell("Hi"), cell("Hi"+leftover)); len(d) == 0 {
			t.Errorf("%s left before </div> compares as equal", leftover)
		}
	}
	same := page("<div>a < b and c </ d</div>")
	if d := referenceDifferences(same, same); len(d) > 0 {
		t.Errorf("a document compares as different from itself: %v", d)
	}
}
