package components

import (
	"encoding/xml"
	"strings"
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
	hugeGutter := "1" + strings.Repeat("0", 308) // 1e308, as parseInt reads it
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
			// gutter × 2 overflows, but MJML's Math.max(0, ...) makes the
			// width 0: the gutter stays (mj-section-gutter-overflow).
			name: "positive gutter whose product overflows, pixel column", width: "100.9px", gutter: hugeGutter + "px", siblings: 3, index: 0,
			wantClass: "mj-column-px-0", wantWidth: "0px",
			wantGutter:  "mj-column-gutter-3-1-px-1e+308",
			wantPadding: "0px 5e+307px 0px 0px", wantMobile: "0% 0% 8.319467554076539e+306% 0%",
			wantOutlook: "0px 5e+307px 0px 0px",
		},
		{
			// Overflows in pixels, not in percent, the column's unit.
			name: "negative gutter finite in percent", gutter: "-" + hugeGutter + "px", siblings: 3, index: 0,
			wantClass: "mj-column-per-1-1092623405435385e+307", wantWidth: "1.1092623405435385e+307%",
			wantGutter:  "mj-column-gutter-3-1-per--1-6638935108153078e+307",
			wantPadding: "0% -8.319467554076539e+306% 0% 0%", wantMobile: "0% 0% -8.319467554076539e+306% 0%",
			wantOutlook: "0px -5e+307px 0px 0px",
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
// NaN or Infinity, which safe-mjml never writes, leaves the column as
// without gutter: a percentage so large that its pixels overflow a float64
// gives MJML an Infinity width (601 * 1e308 / 100).
func TestColumnGutterNone(t *testing.T) {
	huge := "1" + strings.Repeat("0", 308) // 1e308, as parseInt reads it
	tests := []struct {
		gutter, width  string // width "" for none
		containerWidth float64
		siblings       int
		wantClass      string
	}{
		{"", "", 601, 2, "mj-column-per-50"},
		{"px", "", 601, 2, "mj-column-per-50"},
		{"abc", "", 601, 2, "mj-column-per-50"},
		{"1e308%", "", 601, 2, "mj-column-per-50"},
		{"-1e306%", "", 601, 2, "mj-column-per-50"},
		// The reduction is finite, but the column's width less it is not.
		{"-" + huge + "px", "17" + huge[2:] + "px", 601, 2, "mj-column-px-1-7e+308"},
		{"-5e307%", "1.7e308%", 1, 2, "mj-column-per-1-7e+308"},
	}
	for _, tt := range tests {
		gutter := tt.gutter
		node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}}
		if tt.width != "" {
			node.Attrs = []xml.Attr{{Name: xml.Name{Local: "width"}, Value: tt.width}}
		}
		c := NewMJColumnComponent(node, &options.RenderOpts{})
		c.SetContext(ChildContext{ContainerWidth: tt.containerWidth, Siblings: tt.siblings, gutter: gutter})
		if c.hasColumnGutter() {
			t.Errorf("gutter %q, width %q: hasColumnGutter() = true", gutter, tt.width)
		}
		if class, _ := c.GetColumnClass(); class != tt.wantClass {
			t.Errorf("gutter %q, width %q: GetColumnClass() = %q, want %s", gutter, tt.width, class, tt.wantClass)
		}
		if _, _, ok := c.GutterMediaQuery(); ok {
			t.Errorf("gutter %q, width %q: GutterMediaQuery() reports a rule", gutter, tt.width)
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

	// A pixel width whose percentage of the group overflows: no gutter,
	// rather than an Infinity width.
	huge := "17" + strings.Repeat("0", 307) + "px"
	node = &parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}, Attrs: []xml.Attr{{Name: xml.Name{Local: "width"}, Value: huge}}}
	c = NewMJColumnComponent(node, &options.RenderOpts{})
	c.SetContext(ChildContext{ContainerWidth: 1, Siblings: 2, InsideGroup: true, gutter: "2px"})
	if c.hasColumnGutter() {
		t.Errorf("width %s in a 1px group: hasColumnGutter() = true", huge)
	}
	// The same in any unit but %, which the group's mobile width also
	// divides by the group's width.
	for _, unit := range []string{"PX", "pt", "em"} {
		node.Attrs[0].Value = strings.TrimSuffix(huge, "px") + unit
		c = NewMJColumnComponent(node, &options.RenderOpts{})
		c.SetContext(ChildContext{ContainerWidth: 1, Siblings: 2, InsideGroup: true, gutter: "2px"})
		if c.hasColumnGutter() {
			t.Errorf("width %s in a 1px group: hasColumnGutter() = true", node.Attrs[0].Value)
		}
		if got := c.mobileWidth(); strings.Contains(got, "Inf") {
			t.Errorf("width %s in a 1px group: mobileWidth() = %q", node.Attrs[0].Value, got)
		}
	}
	node.Attrs[0].Value = huge
	// Without a gutter, its mobile percentage is saturated rather than
	// written as Infinity.
	c.SetContext(ChildContext{ContainerWidth: 1, Siblings: 2, InsideGroup: true})
	if got, want := c.mobileWidth(), "2147483647%"; got != want {
		t.Errorf("width %s in a 1px group: mobileWidth() = %q, want %q", huge, got, want)
	}
	node.Attrs[0].Value = "-" + huge
	c = NewMJColumnComponent(node, &options.RenderOpts{})
	c.SetContext(ChildContext{ContainerWidth: 1, Siblings: 2, InsideGroup: true})
	if got, want := c.mobileWidth(), "-2147483647%"; got != want {
		t.Errorf("width -%s in a 1px group: mobileWidth() = %q, want %q", huge, got, want)
	}
}

// TestColumnGutterClampsNegativeWidth checks a width that the gutter takes
// below zero, to -Infinity: MJML's Math.max(0, ...) makes it 0, so the
// gutter is kept, as MJML does.
func TestColumnGutterClampsNegativeWidth(t *testing.T) {
	negative := "-17" + strings.Repeat("0", 307) + "px"
	tests := []struct {
		name, gutter, width string
		containerWidth      float64
		inGroup             bool
		wantClass, wantMob  string
	}{
		{"section", "1" + strings.Repeat("0", 308) + "px", negative, 600, false, "mj-column-px-0", "100%"},
		{"group", "2px", negative, 1, true, "mj-column-px-0", "0%"},
		{"group-pt-gutter", "17" + strings.Repeat("0", 306) + "pt", "50px", 1, true, "mj-column-px-0", "0%"},
	}
	for _, tt := range tests {
		node := &parser.MJMLNode{XMLName: xml.Name{Local: "mj-column"}, Attrs: []xml.Attr{{Name: xml.Name{Local: "width"}, Value: tt.width}}}
		c := NewMJColumnComponent(node, &options.RenderOpts{})
		c.SetContext(ChildContext{ContainerWidth: tt.containerWidth, Siblings: 2, InsideGroup: tt.inGroup, gutter: tt.gutter})
		if !c.hasColumnGutter() {
			t.Errorf("%s: hasColumnGutter() = false", tt.name)
		}
		if class, _ := c.GetColumnClass(); class != tt.wantClass {
			t.Errorf("%s: GetColumnClass() = %q, want %q", tt.name, class, tt.wantClass)
		}
		if got := c.mobileWidth(); got != tt.wantMob {
			t.Errorf("%s: mobileWidth() = %q, want %q", tt.name, got, tt.wantMob)
		}
	}
}
