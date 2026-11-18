package components

// Walk calls visit for comp and then, depth first and in document order, for
// each of the descendants that are rendered. When visit returns false, the
// descendants of that component are skipped; visit can then handle them
// itself, when it needs the parent to make sense of them.
//
// It is the one traversal of the component tree: every pass that needs to see
// the whole tree (render metadata, column classes, carousel CSS…) goes
// through it, so a component nested in a new place is seen by all of them,
// and a child its container does not render is seen by none.
func Walk(comp Component, visit func(Component) bool) {
	if comp == nil || !visit(comp) {
		return
	}
	filter, filtered := comp.(childRenderer)
	for _, child := range comp.GetChildren() {
		if filtered && !filter.rendersChild(child) {
			continue
		}
		Walk(child, visit)
	}
}

// childRenderer is a container that renders only some of its children (a
// misplaced component, or an mj-raw MJML does not allow there, is left out of
// the output). Its renderable-children helper and Walk both use rendersChild,
// so they agree on which children those are.
type childRenderer interface {
	rendersChild(child Component) bool
}
