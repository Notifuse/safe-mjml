package mjml_test

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Notifuse/safe-mjml/mjml"
)

// markerContexts places a component, written <X ATTRS>, in a document that
// renders it; DEFAULTS are attributes it needs, which the tested one
// replaces when it has the same name.
var markerContexts = map[string]struct{ doc, defaults string }{
	"mj-body":              {`<mjml><mj-body X><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-section":           {`<mjml><mj-body><mj-section X><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`, `background-url="https://e.com/bg.png"`},
	"mj-wrapper":           {`<mjml><mj-body><mj-wrapper X><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-wrapper></mj-body></mjml>`, `background-url="https://e.com/bg.png"`},
	"mj-hero":              {`<mjml><mj-body><mj-hero X><mj-text>x</mj-text></mj-hero></mj-body></mjml>`, `background-url="https://e.com/bg.png"`},
	"mj-column":            {`<mjml><mj-body><mj-section><mj-column X><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-group":             {`<mjml><mj-body><mj-section><mj-group X><mj-column><mj-text>x</mj-text></mj-column></mj-group></mj-section></mj-body></mjml>`, ``},
	"mj-text":              {`<mjml><mj-body><mj-section><mj-column><mj-text X>x</mj-text></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-button":            {`<mjml><mj-body><mj-section><mj-column><mj-button X>x</mj-button></mj-column></mj-section></mj-body></mjml>`, `href="https://e.com"`},
	"mj-image":             {`<mjml><mj-body><mj-section><mj-column><mj-image X /></mj-column></mj-section></mj-body></mjml>`, `src="https://e.com/a.png" href="https://e.com"`},
	"mj-divider":           {`<mjml><mj-body><mj-section><mj-column><mj-divider X /></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-spacer":            {`<mjml><mj-body><mj-section><mj-column><mj-spacer X /></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-table":             {`<mjml><mj-body><mj-section><mj-column><mj-table X><tr><td>x</td></tr></mj-table></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-raw":               {`<mjml><mj-body><mj-raw X><p>x</p></mj-raw></mj-body></mjml>`, ``},
	"mj-social":            {`<mjml><mj-body><mj-section><mj-column><mj-social X><mj-social-element name="facebook" href="https://e.com">F</mj-social-element></mj-social></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-social-element":    {`<mjml><mj-body><mj-section><mj-column><mj-social><mj-social-element X>F</mj-social-element></mj-social></mj-column></mj-section></mj-body></mjml>`, `name="facebook" href="https://e.com"`},
	"mj-navbar":            {`<mjml><mj-body><mj-section><mj-column><mj-navbar X><mj-navbar-link href="/a">A</mj-navbar-link></mj-navbar></mj-column></mj-section></mj-body></mjml>`, `hamburger="hamburger" base-url="https://e.com"`},
	"mj-navbar-link":       {`<mjml><mj-body><mj-section><mj-column><mj-navbar><mj-navbar-link X>A</mj-navbar-link></mj-navbar></mj-column></mj-section></mj-body></mjml>`, `href="/a"`},
	"mj-accordion":         {`<mjml><mj-body><mj-section><mj-column><mj-accordion X><mj-accordion-element><mj-accordion-title>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element></mj-accordion></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-accordion-element": {`<mjml><mj-body><mj-section><mj-column><mj-accordion><mj-accordion-element X><mj-accordion-title>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element></mj-accordion></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-accordion-title":   {`<mjml><mj-body><mj-section><mj-column><mj-accordion><mj-accordion-element><mj-accordion-title X>T</mj-accordion-title><mj-accordion-text>X</mj-accordion-text></mj-accordion-element></mj-accordion></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-accordion-text":    {`<mjml><mj-body><mj-section><mj-column><mj-accordion><mj-accordion-element><mj-accordion-title>T</mj-accordion-title><mj-accordion-text X>X</mj-accordion-text></mj-accordion-element></mj-accordion></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-carousel":          {`<mjml><mj-body><mj-section><mj-column><mj-carousel X><mj-carousel-image src="https://e.com/1.png" href="https://e.com" /><mj-carousel-image src="https://e.com/2.png" /></mj-carousel></mj-column></mj-section></mj-body></mjml>`, ``},
	"mj-carousel-image":    {`<mjml><mj-body><mj-section><mj-column><mj-carousel><mj-carousel-image X /><mj-carousel-image src="https://e.com/2.png" /></mj-carousel></mj-column></mj-section></mj-body></mjml>`, `src="https://e.com/1.png" href="https://e.com"`},
	"mj-font":              {`<mjml><mj-head><mj-font X /></mj-head><mj-body><mj-section><mj-column><mj-text font-family="Brand">x</mj-text></mj-column></mj-section></mj-body></mjml>`, `name="Brand" href="https://e.com/brand.css"`},
	"mj-breakpoint":        {`<mjml><mj-head><mj-breakpoint X /></mj-head><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`, ``},
	"mjml":                 {`<mjml X><mj-body><mj-section><mj-column><mj-text>x</mj-text></mj-column></mj-section></mj-body></mjml>`, ``},
}

// rootAttributes are the attributes safe-mjml reads on <mjml>, which the
// allowed-attribute table does not list: they reach <html> and the body.
var rootAttributes = []string{"lang", "dir", "owa"}

// globalAttributes are allowed on every component, outside the table.
var globalAttributes = []string{"css-class", "mj-class"}

// knownBrokenAttributes lists the documents of the test, "component
// attribute" with the attribute written in single quotes around a raw
// quote, that still write it raw, with the reason. A listed document that
// no longer breaks fails the test until its line is removed.
var knownBrokenAttributes = map[string]string{}

// The marker: a value with an encoded double quote, as a template may hold.
const marker = `QQ&quot;ZZ`

// rawMarkerInTag finds the marker with a raw quote inside a tag, Outlook
// conditionals included, where the quote closes an attribute. In text or
// in a stylesheet, a raw quote breaks nothing.
var rawMarkerInTag = regexp.MustCompile(`<[^<>]*QQ"ZZ[^<>]*>`)

// TestAttributeValuesNeverBreakTheirAttribute sets the marker on every
// attribute safe-mjml reads on every tag: the ones MJML allows on every
// component, the global css-class and mj-class, and lang, dir and owa on
// <mjml>, each directly and through an mj-class definition, one document
// each (an inventory from the table alone missed <html lang> and the class
// of a navbar link). It checks
// that no tag of the output holds a raw QQ"ZZ: attribute values are kept as
// written (as MJML keeps them), so the encoded quote must still be encoded
// wherever the value is written, an attribute, a style or an Outlook
// conditional. A second pass writes the marker with a raw quote in a
// single-quoted attribute, which safe-mjml escapes where MJML writes it raw. Before, 275 of these documents wrote QQ"ZZ, which closed the
// attribute early (and let a value add markup).
func TestAttributeValuesNeverBreakTheirAttribute(t *testing.T) {
	data, err := os.ReadFile("../internal/components/allowed-css-attributes.json")
	if err != nil {
		t.Fatal(err)
	}
	var allowed map[string]map[string]string
	if err := json.Unmarshal(data, &allowed); err != nil {
		t.Fatal(err)
	}
	allowed["mj-wrapper"] = allowed["mj-section"]
	allowed["mjml"] = map[string]string{}
	for _, attribute := range rootAttributes {
		allowed["mjml"][attribute] = "string"
	}
	documents := 0
	broken := map[string]bool{}
	render := func(name, template string) {
		documents++
		html, err := mjml.Render(template)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return
		}
		if rawMarkerInTag.MatchString(html) {
			broken[name] = true
		}
	}
	for component, context := range markerContexts {
		var attributes []string
		for attribute := range allowed[component] {
			attributes = append(attributes, attribute)
		}
		if len(attributes) == 0 {
			t.Errorf("%s: no allowed attribute", component)
		}
		inHead := strings.Contains(context.doc, "<mj-head>")
		if component != "mjml" && !inHead {
			attributes = append(attributes, globalAttributes...)
		}
		sort.Strings(attributes)
		for _, value := range []string{`="` + marker + `"`, `='QQ"ZZ'`} {
			for _, attribute := range attributes {
				attrs := []string{attribute + value}
				var defaults []string
				for _, d := range strings.Fields(context.defaults) {
					if !strings.HasPrefix(d, attribute+"=") {
						attrs = append(attrs, d)
						defaults = append(defaults, d)
					}
				}
				render(component+" "+attribute+value, strings.Replace(context.doc, " X", " "+strings.Join(attrs, " "), 1))
				// The same value through an mj-class definition, which
				// reaches the component by another path.
				if component == "mjml" || inHead || attribute == "mj-class" {
					continue
				}
				head := `<mjml><mj-head><mj-attributes><mj-class name="m" ` + attribute + value + ` /></mj-attributes></mj-head>`
				doc := head + strings.TrimPrefix(strings.Replace(context.doc, " X", " "+strings.Join(append([]string{`mj-class="m"`}, defaults...), " "), 1), "<mjml>")
				render(component+" mj-class "+attribute+value, doc)
			}
		}
	}
	var names []string
	for name := range broken {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if reason, ok := knownBrokenAttributes[name]; ok {
			t.Logf("known (%s): %s holds a raw QQ\"ZZ", reason, name)
			continue
		}
		t.Errorf("%s: the output holds a raw QQ\"ZZ", name)
	}
	for name := range knownBrokenAttributes {
		if !broken[name] {
			t.Errorf("knownBrokenAttributes lists %s, which no longer breaks: remove it", name)
		}
	}
	if documents < 1500 {
		t.Errorf("only %d documents", documents)
	}
	t.Logf("%d documents, %d broken", documents, len(broken))
}
