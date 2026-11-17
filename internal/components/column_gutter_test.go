package components

import (
	"encoding/xml"
	"testing"

	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// TestColumnGutter checks the gutter of mj-section (MJML 5.4) on a column of
// a 601px section: its class and media-query width less its share of the
// gutters, its gutter class and desktop padding rule, the padding of its div
// (mobile) and of its Outlook cell. The values are those MJML 5.4.1 writes
// (mjml/testdata/mj-section-gutter-*).
func TestColumnGutter(t *testing.T) {
	tests := []struct {
		name        string
		width       string // the column's width attribute, "" for none
		gutter      string
		direction   string
		siblings    int
		index       int
		wantClass   string
		wantWidth   string
		wantGutter  string
		wantPadding string // the desktop rule's padding
		wantMobile  string
		wantOutlook string
	}{
		{
			name: "first of two, pixel gutter", gutter: "13px", siblings: 2, index: 0,
			wantClass: "mj-column-per-48-918469", wantWidth: "48.918469%",
			wantGutter:  "mj-column-gutter-2-1-per-2-163062",
			wantPadding: "0% 1.081531% 0% 0%", wantMobile: "0% 0% 1.081531% 0%",
			wantOutlook: "0px 7px 0px 0px",
		},
		{
			name: "last of two, pixel gutter", gutter: "13px", siblings: 2, index: 1,
			wantClass: "mj-column-per-48-918469", wantWidth: "48.918469%",
			wantGutter:  "mj-column-gutter-2-2-per-2-163062",
			wantPadding: "0% 0% 0% 1.081531%", wantMobile: "1.081531% 0% 0% 0%",
			wantOutlook: "0px 0px 0px 6px",
		},
		{
			name: "pixel width, the odd pixel to the first column", width: "300px", gutter: "13px", siblings: 2, index: 0,
			wantClass: "mj-column-px-294", wantWidth: "294px",
			wantGutter:  "mj-column-gutter-2-1-px-13",
			wantPadding: "0px 7px 0px 0px", wantMobile: "0% 0% 1.081531% 0%",
			wantOutlook: "0px 7px 0px 0px",
		},
		{
			name: "pixel width, second column", width: "300px", gutter: "13px", siblings: 2, index: 1,
			wantClass: "mj-column-px-293", wantWidth: "293px",
			wantGutter:  "mj-column-gutter-2-2-px-13",
			wantPadding: "0px 0px 0px 6px", wantMobile: "1.081531% 0% 0% 0%",
			wantOutlook: "0px 0px 0px 6px",
		},
		{
			name: "percent gutter on a percent column", width: "50%", gutter: "4%", siblings: 2, index: 0,
			wantClass: "mj-column-per-48", wantWidth: "48%",
			wantGutter:  "mj-column-gutter-2-1-per-4",
			wantPadding: "0% 2% 0% 0%", wantMobile: "0% 0% 2% 0%",
			wantOutlook: "0px 12px 0px 0px",
		},
		{
			name: "rtl, first of three", gutter: "13px", direction: "rtl", siblings: 3, index: 0,
			wantClass: "mj-column-per-31-891292", wantWidth: "31.891292%",
			wantGutter:  "mj-column-gutter-3-1-per-2-163062-rtl",
			wantPadding: "0% 0% 0% 1.081531%", wantMobile: "0% 0% 1.081531% 0%",
			wantOutlook: "0px 0px 0px 7px",
		},
		{
			name: "alone: no padding", gutter: "11px", siblings: 1, index: 0,
			wantClass: "mj-column-per-100", wantWidth: "100%",
			wantGutter:  "mj-column-gutter-1-1-per-1-830283",
			wantPadding: "0% 0% 0% 0%", wantMobile: "0% 0% 0% 0%",
			wantOutlook: "0px 0px 0px 0px",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}}
			if tt.width != "" {
				node.Attrs = []xml.Attr{{Name: xml.Name{Local: "width"}, Value: tt.width}}
			}
			c := NewMJColumnComponent(node, &options.RenderOpts{})
			c.SetContext(ChildContext{ContainerWidth: 601, Siblings: tt.siblings, InSection: true,
				gutter: tt.gutter, direction: tt.direction})
			c.setIndex(tt.index)

			if !c.hasColumnGutter() {
				t.Fatal("hasColumnGutter() = false")
			}
			class, width := c.GetColumnClass()
			if class != tt.wantClass || width != tt.wantWidth {
				t.Errorf("GetColumnClass() = %q, %q, want %q, %q", class, width, tt.wantClass, tt.wantWidth)
			}
			gutterClass, rule, ok := c.GutterMediaQuery()
			wantRule := "{ padding: " + tt.wantPadding + " !important; }"
			if !ok || gutterClass != tt.wantGutter || rule != wantRule {
				t.Errorf("GutterMediaQuery() = %q, %q, %v, want %q, %q", gutterClass, rule, ok, tt.wantGutter, wantRule)
			}
			if got := c.mobileGutterCSS(); got != tt.wantMobile {
				t.Errorf("mobileGutterCSS() = %q, want %q", got, tt.wantMobile)
			}
			if got := c.outlookGutterCSS(); got != tt.wantOutlook {
				t.Errorf("outlookGutterCSS() = %q, want %q", got, tt.wantOutlook)
			}
		})
	}
}

// TestColumnGutterNone checks that a gutter MJML would read as none, or as
// NaN, which safe-mjml never writes, leaves the column as without gutter.
func TestColumnGutterNone(t *testing.T) {
	for _, gutter := range []string{"", "px", "abc"} {
		c := NewMJColumnComponent(&parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}}, &options.RenderOpts{})
		c.SetContext(ChildContext{ContainerWidth: 601, Siblings: 2, gutter: gutter})
		if c.hasColumnGutter() {
			t.Errorf("gutter %q: hasColumnGutter() = true", gutter)
		}
		if class, _ := c.GetColumnClass(); class != "mj-column-per-50" {
			t.Errorf("gutter %q: GetColumnClass() = %q, want mj-column-per-50", gutter, class)
		}
		if _, _, ok := c.GutterMediaQuery(); ok {
			t.Errorf("gutter %q: GutterMediaQuery() reports a rule", gutter)
		}
	}
}

// TestColumnGutterInGroup checks a column of a group: no media-query rule
// for its gutter, the desktop padding on its div, and a mobile width less
// the gutters, as a percentage of the group.
func TestColumnGutterInGroup(t *testing.T) {
	node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}}
	c := NewMJColumnComponent(node, &options.RenderOpts{})
	c.SetContext(ChildContext{ContainerWidth: 300, Siblings: 2, InsideGroup: true, gutter: "20px"})
	if _, _, ok := c.GutterMediaQuery(); ok {
		t.Error("GutterMediaQuery() reports a rule in a group")
	}
	if got, want := c.mobileGutterCSS(), "0% 3.333333% 0% 0%"; got != want {
		t.Errorf("mobileGutterCSS() = %q, want %q", got, want)
	}
	if got, want := c.mobileWidth(), "46.666667%"; got != want {
		t.Errorf("mobileWidth() = %q, want %q", got, want)
	}
}
