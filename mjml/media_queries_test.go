package mjml

import (
	"reflect"
	"testing"
)

// TestGroupedMediaQueries checks mjml-core's groupMediaQueries (MJML 5): one
// rule per distinct declaration list, in the order each first appears, with
// its classes in the order they registered, each prefixed; a class
// registered again keeps its place and takes the last rule. A gutter's
// padding rule (MJML 5.4) groups with the classes of the same padding.
func TestGroupedMediaQueries(t *testing.T) {
	c := &rootComponent{columnClasses: map[string]string{}}
	padding := "{ padding: 0px 5px 0px 0px !important; }"
	c.registerColumnClass("mj-column-per-50", mediaQueryRule("50%"))
	c.registerColumnClass("mj-column-gutter-2-1-px-10", padding)
	c.registerColumnClass("mj-column-px-200", mediaQueryRule("200px"))
	c.registerColumnClass("mj-column-per-50-0", mediaQueryRule("50%"))
	c.registerColumnClass("mj-column-px-200", mediaQueryRule("200PX"))
	c.registerColumnClass("mj-column-px-201", mediaQueryRule("200PX"))
	c.registerColumnClass("mj-column-gutter-3-1-px-10", padding)

	want := []string{
		".a .mj-column-per-50, .a .mj-column-per-50-0 { width:50% !important; max-width: 50%; }",
		".a .mj-column-gutter-2-1-px-10, .a .mj-column-gutter-3-1-px-10 { padding: 0px 5px 0px 0px !important; }",
		".a .mj-column-px-200, .a .mj-column-px-201 { width:200PX !important; max-width: 200PX; }",
	}
	if got := c.groupedMediaQueries(".a "); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
