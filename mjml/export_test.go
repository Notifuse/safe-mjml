package mjml

import (
	"maps"
	"slices"
)

// ComponentTags returns the tags safe-mjml builds a component for, sorted,
// for the tests of package mjml_test.
func ComponentTags() []string { return slices.Sorted(maps.Keys(componentFactories)) }
