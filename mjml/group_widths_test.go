package mjml

import (
	"fmt"
	"github.com/Notifuse/safe-mjml/internal/parser"
	"strings"
	"sync"
	"testing"
)

const groupWidthsTwoColumns = `<mj-column><mj-text>G1</mj-text></mj-column><mj-column><mj-text>G2</mj-text></mj-column>`

func groupWidthsTemplate(head, sectionChildren string) string {
	return `<mjml>` + head + `<mj-body width="600px"><mj-section>` + sectionChildren + `</mj-section></mj-body></mjml>`
}

// TestGroupWidthRules pins the exact media-query rule of each group class in
// both responsive style blocks. The fixture comparison sorts style characters,
// so it cannot tell `.a{40%} .b{60%}` from `.a{60%} .b{40%}`; this test can.
func TestGroupWidthRules(t *testing.T) {
	tests := []struct {
		name         string
		head         string
		children     string
		wantRules    map[string]string // class -> rule width
		wantGroupDiv string            // class attribute of the group div, when checked
		wantNoRule   string            // a class that must have no rule, when checked
	}{
		{
			name:         "width-less group next to a column takes half",
			children:     `<mj-group>` + groupWidthsTwoColumns + `</mj-group><mj-column><mj-text>C</mj-text></mj-column>`,
			wantRules:    map[string]string{"mj-column-per-50": "50%"},
			wantGroupDiv: `class="mj-column-per-50 mj-outlook-group-fix" style="font-size:0;line-height:0;`,
			wantNoRule:   "mj-column-per-100",
		},
		{
			name:      "percentage group next to a column of another width",
			children:  `<mj-group width="60%">` + groupWidthsTwoColumns + `</mj-group><mj-column width="40%"><mj-text>C</mj-text></mj-column>`,
			wantRules: map[string]string{"mj-column-per-60": "60%", "mj-column-per-40": "40%", "mj-column-per-50": "50%"},
		},
		{
			name:      "percentage group alone",
			children:  `<mj-group width="60%">` + groupWidthsTwoColumns + `</mj-group>`,
			wantRules: map[string]string{"mj-column-per-60": "60%", "mj-column-per-50": "50%"},
		},
		{
			name:      "decimal percentage group",
			children:  `<mj-group width="33.5%">` + groupWidthsTwoColumns + `</mj-group><mj-column width="66.5%"><mj-text>C</mj-text></mj-column>`,
			wantRules: map[string]string{"mj-column-per-33": "33.5%", "mj-column-per-66-5": "66.5%"},
		},
		{
			name:      "group sharing a section with two columns",
			children:  `<mj-group>` + groupWidthsTwoColumns + `</mj-group><mj-column><mj-text>C</mj-text></mj-column><mj-column><mj-text>D</mj-text></mj-column>`,
			wantRules: map[string]string{"mj-column-per-33": "33.333333333333336%", "mj-column-per-33-333333333333336": "33.333333333333336%"},
		},
		{
			name:      "pixel group",
			children:  `<mj-group width="400px">` + groupWidthsTwoColumns + `</mj-group><mj-column width="200px"><mj-text>C</mj-text></mj-column>`,
			wantRules: map[string]string{"mj-column-px-400": "400px", "mj-column-px-200": "200px"},
		},
		{
			name:      "pixel group width from mj-attributes",
			head:      `<mj-head><mj-attributes><mj-group width="300px" /></mj-attributes></mj-head>`,
			children:  `<mj-group>` + groupWidthsTwoColumns + `</mj-group><mj-column><mj-text>C</mj-text></mj-column>`,
			wantRules: map[string]string{"mj-column-px-300": "300px"},
		},
		{
			name:      "group and column sharing a class name: the column, rendered last, wins",
			children:  `<mj-group width="33.5%">` + groupWidthsTwoColumns + `</mj-group><mj-column width="33%"><mj-text>C</mj-text></mj-column>`,
			wantRules: map[string]string{"mj-column-per-33": "33%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//GIVEN: a section holding a group and its siblings.
			template := groupWidthsTemplate(tt.head, tt.children)

			//WHEN: it is rendered.
			html, err := Render(template)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			//THEN: each class has exactly the expected rule in both responsive blocks.
			for class, width := range tt.wantRules {
				rule := fmt.Sprintf(" { width:%s !important; max-width: %s; }", width, width)
				if want := "." + class + rule; !strings.Contains(html, "        "+want+"\n") {
					t.Errorf("missing media-query rule %q", want)
				}
				if want := ".moz-text-html ." + class + rule; !strings.Contains(html, want) {
					t.Errorf("missing moz rule %q", want)
				}
			}
			if tt.wantGroupDiv != "" && !strings.Contains(html, tt.wantGroupDiv) {
				t.Errorf("expected the group div %q", tt.wantGroupDiv)
			}
			if tt.wantNoRule != "" && strings.Contains(html, "."+tt.wantNoRule+" {") {
				t.Errorf("expected no rule for %q", tt.wantNoRule)
			}
		})
	}
}

// TestGroupSharesOutlookRowWithSiblings checks that a group and the column next
// to it sit in the same Outlook row: one shared table, one cell each.
func TestGroupSharesOutlookRowWithSiblings(t *testing.T) {
	//GIVEN: a width-less group followed by a column in a 600px section.
	template := groupWidthsTemplate("", `<mj-group>`+groupWidthsTwoColumns+`</mj-group><mj-column><mj-text>C</mj-text></mj-column>`)

	//WHEN: it is rendered.
	html, err := Render(template)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	//THEN: the group cell opens the shared row and the column cell follows it in the same row.
	groupCell := `<!--[if mso | IE]><table role="presentation" border="0" cellpadding="0" cellspacing="0"><tr><td class="" style="width:300px;" ><![endif]-->`
	columnCell := `<!--[if mso | IE]></td><td class="" style="vertical-align:top;width:300px;" ><![endif]-->`
	if !strings.Contains(html, groupCell) {
		t.Errorf("expected the group cell to open the shared Outlook row: %s", groupCell)
	}
	if !strings.Contains(html, columnCell) {
		t.Errorf("expected the column cell in the same Outlook row: %s", columnCell)
	}
	if strings.Count(html, `<table role="presentation" border="0" cellpadding="0" cellspacing="0"><tr>`) != 1 {
		t.Errorf("expected a single shared Outlook table for the section children")
	}
}

// TestGroupWithSiblingRendersIdenticallyTwiceAndConcurrently renders a group
// with a sibling column from one component twice, then concurrently through the
// AST cache, and expects identical output every time.
func TestGroupWithSiblingRendersIdenticallyTwiceAndConcurrently(t *testing.T) {
	template := groupWidthsTemplate("", `<mj-raw><p>R</p></mj-raw><mj-group width="60%">`+groupWidthsTwoColumns+`</mj-group><mj-column><mj-text>C</mj-text></mj-column>`)

	//GIVEN: one component built from the template.
	ast, err := parser.ParseMJML(template)
	if err != nil {
		t.Fatalf("ParseMJML failed: %v", err)
	}
	component, err := newComponentTree(ast)
	if err != nil {
		t.Fatalf("NewFromAST failed: %v", err)
	}

	//WHEN: it is rendered twice.
	var first, second strings.Builder
	if err := component.Render(&first); err != nil {
		t.Fatalf("first Render failed: %v", err)
	}
	if err := component.Render(&second); err != nil {
		t.Fatalf("second Render failed: %v", err)
	}

	//THEN: both renders are identical.
	if first.String() != second.String() {
		t.Fatalf("expected identical output across repeated renders\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}

	//WHEN: the template is rendered concurrently through the AST cache.
	cache := NewCache(0)
	defer cache.Close()
	reference, err := Render(template)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	const workers = 16
	var wg sync.WaitGroup
	outputs := make([]string, workers)
	errs := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			outputs[idx], errs[idx] = Render(template, WithCache(cache))
		}(i)
	}
	wg.Wait()

	//THEN: every render matches the uncached one.
	for i := range workers {
		if errs[i] != nil {
			t.Fatalf("worker %d: Render failed: %v", i, errs[i])
		}
		if outputs[i] != reference {
			t.Fatalf("worker %d: output differs from the uncached render", i)
		}
	}
}
