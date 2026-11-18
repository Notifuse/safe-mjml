package juicer

import (
	"errors"
	"strings"
	"testing"

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
}
