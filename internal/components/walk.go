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

// childRenderer is a container that renders only some of its children. Every
// container renders a misplaced child as MJML does, with a warning, except
// mj-carousel, which renders only its images: MJML throws for any other
// child there (testdata/placement/rejected.json). Its renderable-children
// helper and Walk both use rendersChild, so they agree on which children
// those are.
type childRenderer interface {
	rendersChild(child Component) bool
}

// WalkInCreationOrder calls visit for each descendant of comp, in the order
// mjml-core creates their components, which is the order the head styles of
// components are registered in (initComponent): a container creates all its
// children, in order, then renders each one, which creates its own. So the
// children of comp come first, then the descendants of the first child, and
// so on. As Walk, it leaves out the children a container does not render.
func WalkInCreationOrder(comp Component, visit func(Component)) {
	if comp == nil {
		return
	}
	filter, filtered := comp.(childRenderer)
	children := comp.GetChildren()
	rendered := children[:0:0]
	for _, child := range children {
		if !filtered || filter.rendersChild(child) {
			rendered = append(rendered, child)
		}
	}
	for _, child := range rendered {
		visit(child)
	}
	for _, child := range rendered {
		WalkInCreationOrder(child, visit)
	}
}
