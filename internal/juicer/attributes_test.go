package juicer

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

// TestSetAttributes checks SetAttributes against what mjml-core's cheerio
// pass (xmlMode, decodeEntities off) writes in MJML 5.4.1.
func TestSetAttributes(t *testing.T) {
	attr := func(kv ...string) []html.Attribute {
		var out []html.Attribute
		for i := 0; i < len(kv); i += 2 {
			out = append(out, html.Attribute{Key: kv[i], Val: kv[i+1]})
		}
		return out
	}
	tests := []struct {
		name, doc string
		rules     []AttributeRule
		want      string
		invalid   []string
	}{
		{
			name:  "a new attribute goes last, an existing one is replaced in place",
			doc:   `<td class="a" style="x">t</td><td>u</td>`,
			rules: []AttributeRule{{Selector: ".a", Attributes: attr("data-id", "42", "class", "b")}},
			want:  `<td class="b" style="x" data-id="42">t</td><td>u</td>`,
		},
		{
			name:  "rules apply in order, the later value wins",
			doc:   `<p class="a">t</p>`,
			rules: []AttributeRule{{Selector: "p", Attributes: attr("data-id", "1")}, {Selector: ".a", Attributes: attr("data-id", "2")}},
			want:  `<p class="a" data-id="2">t</p>`,
		},
		{
			name:  "XML mode: an empty element closes itself, br holds what follows",
			doc:   `<div><span></span>a<br>b</div><img src="x">`,
			rules: []AttributeRule{{Selector: "img", Attributes: attr("title", "")}},
			want:  `<div><span/>a<br>b</br></div><img src="x" title=""/>`,
		},
		{
			name:  "entities are kept, a quote in a value is escaped",
			doc:   `<p>a &amp; b &nbsp;</p>`,
			rules: []AttributeRule{{Selector: "p", Attributes: attr("data-x", `a &amp; "b"`)}},
			want:  `<p data-x="a &amp; &quot;b&quot;">a &amp; b &nbsp;</p>`,
		},
		{
			name:  "a selector list, and a pseudo-class juice preserves, which matches nothing",
			doc:   `<a>x</a><b>y</b>`,
			rules: []AttributeRule{{Selector: "a, b", Attributes: attr("data-l", "1")}, {Selector: "a:hover", Attributes: attr("data-h", "1")}},
			want:  `<a data-l="1">x</a><b data-l="1">y</b>`,
		},
		{
			name:  "comments, Outlook conditionals included, are left as they are",
			doc:   `<!--[if mso | IE]><td class="a"><![endif]--><td class="a">t</td>`,
			rules: []AttributeRule{{Selector: "td", Attributes: attr("data-id", "1")}},
			want:  `<!--[if mso | IE]><td class="a"><![endif]--><td class="a" data-id="1">t</td>`,
		},
		{
			name:  "template tags are kept, inside a tag too, as Inline keeps them",
			doc:   `<p {% if x %}class="a"{% endif %}>{{ name }}</p>`,
			rules: []AttributeRule{{Selector: "p", Attributes: attr("data-id", "{{ id }}")}},
			want:  `<p {% if x %}class="a" {% endif %} data-id="{{ id }}">{{ name }}</p>`,
		},
		{
			name:    "a selector cheerio throws on sets nothing and is reported",
			doc:     `<p>t</p>`,
			rules:   []AttributeRule{{Selector: "p[", Attributes: attr("data-id", "1")}, {Selector: "p", Attributes: attr("data-ok", "1")}},
			want:    `<p data-ok="1">t</p>`,
			invalid: []string{"p["},
		},
	}
	for _, tt := range tests {
		got, invalid, err := SetAttributes(tt.doc, tt.rules, CodeBlocks(DefaultCodeBlocks()))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s:\n got %s\nwant %s", tt.name, got, tt.want)
		}
		if strings.Join(invalid, "|") != strings.Join(tt.invalid, "|") {
			t.Errorf("%s: invalid = %q, want %q", tt.name, invalid, tt.invalid)
		}
	}
}

// TestSetAttributesMatchBudget checks that MaxMatchSteps bounds the matching
// of the selectors, as it bounds inline mj-style's.
func TestSetAttributesMatchBudget(t *testing.T) {
	doc := strings.Repeat(`<div><p class="a">t</p></div>`, 2000)
	rules := make([]AttributeRule, 200)
	for i := range rules {
		rules[i] = AttributeRule{Selector: "div div p.a", Attributes: []html.Attribute{{Key: "data-x", Val: "1"}}}
	}
	if _, _, err := SetAttributes(doc, rules, MaxMatchSteps(10000)); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("err = %v, want ErrLimitExceeded", err)
	}
	// A budget that runs out in a selector's compile is the limit error, not
	// an invalid selector.
	a := attr("data-m", "1")
	long := "zz" + strings.Repeat(".x", 3000)
	out, invalid, err := SetAttributes("<r><b>x</b><i>y</i></r>", []AttributeRule{{Selector: "b", Attributes: a}, {Selector: long, Attributes: a}, {Selector: "i", Attributes: a}}, MaxMatchSteps(2000))
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("got %q, invalid %q, err %v; want ErrLimitExceeded", out, invalid, err)
	}
}

// attr builds attributes from name, value pairs.
func attr(kv ...string) []html.Attribute {
	var out []html.Attribute
	for i := 0; i < len(kv); i += 2 {
		out = append(out, html.Attribute{Key: kv[i], Val: kv[i+1]})
	}
	return out
}

// TestSetAttributesAsCheerio checks SetAttributes against cheerio 1.2.0,
// MJML 5.4.1's, loading each document with xmlMode and decodeEntities off and
// setting each attribute of each rule with $(selector).each(attr), as
// mjml-core does: the expected outputs are cheerio's, and a selector is
// invalid where cheerio throws on it. It covers the selection again for each
// attribute, case-sensitive names and values, cheerio-select's positional
// pseudo-classes (with its "+" that selects as "~" after one), and what
// css-what reads or rejects.
func TestSetAttributesAsCheerio(t *testing.T) {
	tests := []struct {
		name, doc string
		rules     []AttributeRule
		want      string
		invalid   []string
	}{
		{
			name:  "each attribute selects again: after target changes, rel selects nothing",
			doc:   "<a target=\"_blank\">x</a>",
			rules: []AttributeRule{{Selector: "a[target=_blank]", Attributes: attr("target", "_self", "rel", "noopener")}},
			want:  "<a target=\"_self\">x</a>",
		},
		{
			name:  "each attribute selects again: data-a set on the cell makes the div match for data-b",
			doc:   "<td><div>x</div></td>",
			rules: []AttributeRule{{Selector: "td, [data-a] div", Attributes: attr("data-a", "1", "data-b", "2")}},
			want:  "<td data-a=\"1\" data-b=\"2\"><div data-b=\"2\">x</div></td>",
		},
		{
			name:  "XML mode: names and values keep their case",
			doc:   "<td align=\"left\">a</td><SPAN>b</SPAN><svg viewBox=\"0 0 1 1\"/><input type=\"CHECKBOX\"/>",
			rules: []AttributeRule{{Selector: "TD", Attributes: attr("data-1", "1")}, {Selector: "SPAN", Attributes: attr("data-2", "1")}, {Selector: "[viewBox]", Attributes: attr("data-3", "1")}, {Selector: "[align=LEFT]", Attributes: attr("data-4", "1")}, {Selector: "[type=checkbox]", Attributes: attr("data-5", "1")}},
			want:  "<td align=\"left\">a</td><SPAN data-2=\"1\">b</SPAN><svg viewBox=\"0 0 1 1\" data-3=\"1\"/><input type=\"CHECKBOX\"/>",
		},
		{
			name:  "a descendant combinator before a positional pseudo-class: td :first is the first element under a td",
			doc:   "<r><td><div>a</div><b>x</b></td><td><div>c</div></td></r>",
			rules: []AttributeRule{{Selector: "td :first", Attributes: attr("data-m", "1")}, {Selector: "td :last", Attributes: attr("data-l", "1")}, {Selector: "r :eq(1)", Attributes: attr("data-e", "1")}},
			want:  "<r><td><div data-m=\"1\" data-e=\"1\">a</div><b>x</b></td><td><div data-l=\"1\">c</div></td></r>",
		},
		{
			name:  "a quote in the text of :contains() opens no string: the comma after it splits the list",
			doc:   "<r><p>it's</p><b>don't</b><i>x</i><p>\")</p></r>",
			rules: []AttributeRule{{Selector: "p:contains(it's), i", Attributes: attr("data-m", "1")}, {Selector: "b:contains(don't), i:contains(x)", Attributes: attr("data-n", "2")}, {Selector: "p:contains(\"), b", Attributes: attr("data-q", "3")}},
			want:  "<r><p data-m=\"1\" data-q=\"3\">it's</p><b data-n=\"2\" data-q=\"3\">don't</b><i data-m=\"1\" data-n=\"2\">x</i><p data-q=\"3\">\")</p></r>",
		},
		{
			name:  ":not(:even) on an empty selection selects nothing, as JavaScript's slice(0, -1)",
			doc:   "<r><b>x</b></r>",
			rules: []AttributeRule{{Selector: "zzz:not(:even)", Attributes: attr("data-m", "1")}, {Selector: "b:not(:even)", Attributes: attr("data-n", "1")}},
			want:  "<r><b>x</b></r>",
		},
		{
			name:  "after a positional pseudo-class, a combinator starts from the selected elements not nested in another (removeSubsets)",
			doc:   "<r><td id=\"1\"><div id=\"a\"><td id=\"2\"><div id=\"b\"/></td></div><div id=\"c\"/></td><td id=\"3\"><div id=\"d\"/></td></r>",
			rules: []AttributeRule{{Selector: "td:lt(3) > div", Attributes: attr("data-m", "1")}},
			want:  "<r><td id=\"1\"><div id=\"a\" data-m=\"1\"><td id=\"2\"><div id=\"b\"/></td></div><div id=\"c\" data-m=\"1\"/></td><td id=\"3\"><div id=\"d\" data-m=\"1\"/></td></r>",
		},
		{
			name:  "li:first",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:first", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\" data-m=\"1\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:last",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:last", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li data-m=\"1\">d</li></ol>",
		},
		{
			name:  "li:eq(-1)",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:eq(-1)", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li data-m=\"1\">d</li></ol>",
		},
		{
			name:  "li:gt(0)",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:gt(0)", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li data-m=\"1\">b</li><li data-m=\"1\">c</li></ul><ol><li data-m=\"1\">d</li></ol>",
		},
		{
			name:  "li:lt(2)",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:lt(2)", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\" data-m=\"1\">a</li><li data-m=\"1\">b</li><li>c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:even",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:even", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\" data-m=\"1\">a</li><li>b</li><li data-m=\"1\">c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:odd",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:odd", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li data-m=\"1\">b</li><li>c</li></ul><ol><li data-m=\"1\">d</li></ol>",
		},
		{
			name:  "li:not(:first)",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:not(:first)", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li data-m=\"1\">b</li><li data-m=\"1\">c</li></ul><ol><li data-m=\"1\">d</li></ol>",
		},
		{
			name:  "ul:first li:last",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "ul:first li:last", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li data-m=\"1\">c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:eq(0) + li",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:eq(0) + li", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li data-m=\"1\">b</li><li data-m=\"1\">c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:first.y + li",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:first.y + li", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  ":first",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: ":first", Attributes: attr("data-m", "1")}},
			want:  "<ul data-m=\"1\"><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:eq(x)",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:eq(x)", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
		},
		{
			name:  "li:first, li:last",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol>",
			rules: []AttributeRule{{Selector: "li:first, li:last", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\" data-m=\"1\">a</li><li>b</li><li>c</li></ul><ol><li data-m=\"1\">d</li></ol>",
		},
		{
			name:    "li,",
			doc:     "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules:   []AttributeRule{{Selector: "li,", Attributes: attr("data-m", "1")}},
			want:    "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			invalid: []string{"li,"},
		},
		{
			name:    "li, , a",
			doc:     "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules:   []AttributeRule{{Selector: "li, , a", Attributes: attr("data-m", "1")}},
			want:    "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			invalid: []string{"li, , a"},
		},
		{
			name:    "li::before",
			doc:     "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules:   []AttributeRule{{Selector: "li::before", Attributes: attr("data-m", "1")}},
			want:    "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			invalid: []string{"li::before"},
		},
		{
			name:    "li:before",
			doc:     "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules:   []AttributeRule{{Selector: "li:before", Attributes: attr("data-m", "1")}},
			want:    "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			invalid: []string{"li:before"},
		},
		{
			name:    ":focus",
			doc:     "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules:   []AttributeRule{{Selector: ":focus", Attributes: attr("data-m", "1")}},
			want:    "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			invalid: []string{":focus"},
		},
		{
			name:  "an empty path selects nothing",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
		},
		{
			name:  "a blank path selects nothing",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "  ", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
		},
		{
			name:  "[href=https://e.com]",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "[href=https://e.com]", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\" data-m=\"1\">l</a>",
		},
		{
			name:  "[class!=y]",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "[class!=y]", Attributes: attr("data-m", "1")}},
			want:  "<ul data-m=\"1\"><li class=\"y\">a</li><li data-m=\"1\">b</li><li data-m=\"1\">c</li></ul><ol data-m=\"1\"><li data-m=\"1\">d</li></ol><a href=\"https://e.com\" data-m=\"1\">l</a>",
		},
		{
			name:  "ul >",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "ul >", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\" data-m=\"1\">a</li><li data-m=\"1\">b</li><li data-m=\"1\">c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
		},
		{
			name:  "> ul",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "> ul", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
		},
		{
			name:  ":scope > ul",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: ":scope > ul", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
		},
		{
			name:  ":link",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: ":link", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\" data-m=\"1\">l</a>",
		},
		{
			name:  "LI",
			doc:   "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
			rules: []AttributeRule{{Selector: "LI", Attributes: attr("data-m", "1")}},
			want:  "<ul><li class=\"y\">a</li><li>b</li><li>c</li></ul><ol><li>d</li></ol><a href=\"https://e.com\">l</a>",
		},
	}
	for _, tt := range tests {
		got, invalid, err := SetAttributes(tt.doc, tt.rules)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s:\n got %s\nwant %s", tt.name, got, tt.want)
		}
		if strings.Join(invalid, "|") != strings.Join(tt.invalid, "|") {
			t.Errorf("%s: invalid = %q, want %q", tt.name, invalid, tt.invalid)
		}
	}
}

// TestSetAttributesInvalidWithoutAttributes checks that a rule without
// attributes is not read: mjml-core never selects with it, so a selector
// cheerio would throw on is not reported.
func TestSetAttributesInvalidWithoutAttributes(t *testing.T) {
	got, invalid, err := SetAttributes(`<p>t</p>`, []AttributeRule{{Selector: "p["}})
	if err != nil || got != `<p>t</p>` || len(invalid) != 0 {
		t.Fatalf("got %q, invalid %q, err %v; want the document unchanged and nothing reported", got, invalid, err)
	}
}

// TestSetAttributesMinifiedVoids checks MinifiedVoids: an element without
// children is its start tag alone, as html-minifier writes cheerio's <x/>,
// except in an svg, which keeps the slash (and an svg that closes itself
// keeps it for the rest of the document); the end tag of a void element
// that holds what follows is left out, and a br's is a <br>.
func TestSetAttributesMinifiedVoids(t *testing.T) {
	// The outputs of cheerio in XML mode, then html-minifier with MJML's
	// options, for the same documents and rule.
	for _, c := range []struct{ doc, want string }{
		{`<div><span></span>a<br>b</div><img src="x"/><p/>`, `<div><span>a<br>b<br></div><img src="x" alt=""><p>`},
		{`<svg><rect width="3"/><g></g><IMG/></svg><i></i>`, `<svg><rect width="3"/><g/><IMG/></svg><i>`},
		{`<svg/><i></i><img><b>x</b>`, `<svg/><i/><img alt=""><b>x</b>`},
		{`<IMG src="y"><b>z</b><BR><i>w</i>`, `<IMG src="y"><b>z</b><BR><i>w</i><BR>`},
		{`<div><svg><g><img><b>x</b></g></svg><i></i></div>`, `<div><svg><g><img alt=""><b>x</b></g></svg><i></div>`},
	} {
		got, _, err := SetAttributes(c.doc, []AttributeRule{{Selector: "img", Attributes: attr("alt", "")}}, MinifiedVoids())
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.doc, got, c.want)
		}
	}
}

// TestSetAttributesOutputLimit checks that MaxOutputSize stops the pass as
// soon as the attributes it sets would take the document past it, before
// the document is written: one value of 100,000 bytes on 20,000 elements
// would write 2 GB.
func TestSetAttributesOutputLimit(t *testing.T) {
	doc := strings.Repeat(`<b/>`, 20000)
	rules := []AttributeRule{{Selector: "b", Attributes: attr("data-v", strings.Repeat("v", 100000))}}
	_, _, err := SetAttributes(doc, rules, MaxOutputSize(16<<20))
	if !errors.Is(err, ErrOutputTooLarge) || !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("err = %v, want ErrOutputTooLarge", err)
	}
	// Under the limit, the document is written.
	if _, _, err := SetAttributes(doc[:400], rules[:1], MaxOutputSize(16<<20)); err != nil {
		t.Fatal(err)
	}
}

// TestSetAttributesWorkBudget checks that MaxMatchSteps bounds the work of
// the pass beyond the selectors' matching: the bytes of the values set,
// each node a selection walks, comments included, and the attributes an
// element is looked through for each one set.
func TestSetAttributesWorkBudget(t *testing.T) {
	many := func(n int) []html.Attribute {
		out := make([]html.Attribute, n)
		for i := range out {
			out[i] = html.Attribute{Key: "data-a" + strconv.Itoa(i), Val: "1"}
		}
		return out
	}
	selectors := func(n int) []AttributeRule {
		out := make([]AttributeRule, n)
		for i := range out {
			out[i] = AttributeRule{Selector: "x" + strconv.Itoa(i), Attributes: attr("data-x", "1")}
		}
		return out
	}
	tests := []struct {
		name  string
		doc   string
		rules []AttributeRule
	}{
		{"a long value on many elements", strings.Repeat(`<b/>`, 2000), []AttributeRule{{Selector: "b", Attributes: attr("data-v", strings.Repeat("v", 100000))}}},
		{"many attributes on many elements", strings.Repeat(`<b/>`, 3000), []AttributeRule{{Selector: "b", Attributes: many(3000)}}},
		{"many selectors over many comments", strings.Repeat(`<!---->`, 200000), selectors(20000)},
		{"a long :not() list over an empty selection", "", []AttributeRule{{Selector: "zz:not(" + strings.Repeat("a,", 40000) + ":first)", Attributes: many(4000)}}},
		{"a chain of :not(:first) over many elements", strings.Repeat(`<b/>`, 50000), []AttributeRule{{Selector: "b" + strings.Repeat(":not(:first)", 1000) + ":eq(99999999)", Attributes: many(40)}}},
	}
	for _, tt := range tests {
		start := time.Now()
		_, _, err := SetAttributes(tt.doc, tt.rules, MaxMatchSteps(250_000_000))
		if !errors.Is(err, ErrLimitExceeded) {
			t.Errorf("%s: err = %v, want ErrLimitExceeded", tt.name, err)
		}
		// Coverage counters and the race detector make each step about
		// eight times slower.
		limit := 10 * time.Second
		if testing.CoverMode() != "" || raceEnabled {
			limit *= 10
		}
		if d := time.Since(start); d > limit {
			t.Errorf("%s: took %v to fail", tt.name, d)
		}
	}
}
