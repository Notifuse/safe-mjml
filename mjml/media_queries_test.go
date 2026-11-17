package mjml

import (
	"reflect"
	"testing"
)

// TestGroupedMediaQueries checks mjml-core's groupMediaQueries (MJML 5): one
// rule per distinct declaration list, in the order each first appears, with
// its classes in the order they registered, each prefixed; a class
// registered again keeps its place and takes the last width.
func TestGroupedMediaQueries(t *testing.T) {
	c := &rootComponent{columnClasses: map[string]string{}}
	c.registerColumnClass("mj-column-per-50", "50%")
	c.registerColumnClass("mj-column-px-200", "200px")
	c.registerColumnClass("mj-column-per-50-0", "50%")
	c.registerColumnClass("mj-column-px-200", "200PX")
	c.registerColumnClass("mj-column-px-201", "200PX")

	want := []string{
		".a .mj-column-per-50, .a .mj-column-per-50-0 { width:50% !important; max-width: 50%; }",
		".a .mj-column-px-200, .a .mj-column-px-201 { width:200PX !important; max-width: 200PX; }",
	}
	if got := c.groupedMediaQueries(".a "); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
