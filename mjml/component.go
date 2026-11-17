package mjml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/debug"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// createComponent creates the component of an MJML element node.
func createComponent(node *parser.MJMLNode, opts *options.RenderOpts) (components.Component, error) {
	tagName := node.GetTagName()

	// Log component creation
	if debug.Enabled() {
		debug.DebugLogWithData("component", "create", "Creating component", map[string]any{
			"tag_name":     tagName,
			"has_children": len(node.Children) > 0,
			"attr_count":   len(node.Attrs),
		})
	}

	if tagName == "mjml" {
		return createrootComponent(node, opts)
	}
	if factory, ok := componentFactories[tagName]; ok {
		return factory(node, opts), nil
	}
	err := fmt.Errorf("%w: %s", errUnknownComponent, tagName)
	if debug.Enabled() {
		debug.DebugLogError("component", "create-error", "Unknown component type", err)
	}
	return nil, err
}

// componentFactories creates the component of each tag safe-mjml implements,
// mjml apart.
var componentFactories = map[string]func(*parser.MJMLNode, *options.RenderOpts) components.Component{
	"mj-head": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJHeadComponent(n, o)
	},
	"mj-body": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJBodyComponent(n, o)
	},
	"mj-section": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJSectionComponent(n, o)
	},
	"mj-column": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJColumnComponent(n, o)
	},
	"mj-text": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJTextComponent(n, o)
	},
	"mj-button": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJButtonComponent(n, o)
	},
	"mj-image": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJImageComponent(n, o)
	},
	"mj-title": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJTitleComponent(n, o)
	},
	"mj-font": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJFontComponent(n, o)
	},
	"mj-breakpoint": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJBreakpointComponent(n, o)
	},
	"mj-wrapper": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJWrapperComponent(n, o)
	},
	"mj-divider": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJDividerComponent(n, o)
	},
	"mj-social": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJSocialComponent(n, o)
	},
	"mj-social-element": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJSocialElementComponent(n, o)
	},
	"mj-group": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJGroupComponent(n, o)
	},
	"mj-preview": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJPreviewComponent(n, o)
	},
	"mj-style": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJStyleComponent(n, o)
	},
	"mj-attributes": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJAttributesComponent(n, o)
	},
	"mj-all": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJAllComponent(n, o)
	},
	"mj-accordion": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJAccordionComponent(n, o)
	},
	"mj-accordion-text": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJAccordionTextComponent(n, o)
	},
	"mj-accordion-title": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJAccordionTitleComponent(n, o)
	},
	"mj-accordion-element": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJAccordionElementComponent(n, o)
	},
	"mj-carousel": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJCarouselComponent(n, o)
	},
	"mj-carousel-image": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJCarouselImageComponent(n, o)
	},
	"mj-hero": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJHeroComponent(n, o)
	},
	"mj-navbar": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJNavbarComponent(n, o)
	},
	"mj-navbar-link": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJNavbarLinkComponent(n, o)
	},
	"mj-spacer": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJSpacerComponent(n, o)
	},
	"mj-table": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJTableComponent(n, o)
	},
	"mj-raw": func(n *parser.MJMLNode, o *options.RenderOpts) components.Component {
		return components.NewMJRawComponent(n, o)
	},
}

// errUnknownComponent is the error createComponent returns, wrapped with the
// tag name, for a tag that is not a component safe-mjml implements.
var errUnknownComponent = errors.New("unknown component")

func createrootComponent(node *parser.MJMLNode, opts *options.RenderOpts) (*rootComponent, error) {
	// Extract lang attribute from root MJML element and store in opts
	// Default to LangUndetermined if not specified - this is the proper fallback
	// per emailmarkup.org accessibility guidelines: "It's not nearly as good as
	// setting a language but it's much better than setting nothing"
	if langAttr := node.GetAttribute("lang"); langAttr != "" {
		opts.Lang = langAttr
	} else {
		opts.Lang = constants.LangUndetermined
	}
	// As mjml-core: dir || 'auto', which the body's div writes.
	opts.Dir = node.GetAttribute("dir")
	if opts.Dir == "" {
		opts.Dir = constants.DirAuto
	}

	comp := &rootComponent{
		BaseComponent: components.NewBaseComponent(node, opts),
	}

	// The root renders its mj-head and mj-body, and before the doctype its
	// mj-raw at position="file-start", their contents joined by line breaks
	// (mjml-core's beforeDoctype); report anything else.
	var beforeDoctype []string
	for _, childNode := range node.Children {
		switch tag := childNode.GetTagName(); tag {
		case "mj-head", "mj-body":
		case "mj-raw":
			if childNode.GetAttribute("position") == "file-start" {
				beforeDoctype = append(beforeDoctype, strings.TrimSpace(childNode.Text))
			}
		default:
			if isMJMLTag(tag) && !unsupportedTags[tag] {
				warnIfMisplaced(opts, "mjml", childNode)
			} else {
				warnUnknownTag(opts, childNode)
			}
		}
	}

	// Find head and body components
	if headNode := node.FindFirstChild("mj-head"); headNode != nil {
		head := components.NewMJHeadComponent(headNode, opts)
		if err := buildChildren(head, headNode, opts); err != nil {
			return nil, err
		}

		comp.Head = head
	}
	// The title, read once here, as the lang: the body writes it as its
	// aria-label and the head as its <title>.
	opts.Title = headTitle(comp.Head)

	// The body is built after the head, whose mj-style inline rules its
	// components read.
	if bodyNode := node.FindFirstChild("mj-body"); bodyNode != nil {
		body := components.NewMJBodyComponent(bodyNode, opts)
		body.Preview = headPreview(comp.Head)
		if err := buildChildren(body, bodyNode, opts); err != nil {
			return nil, err
		}
		comp.Body = body
	}
	comp.beforeDoctype = strings.Join(beforeDoctype, "\n")

	return comp, nil
}

// componentContainers are the tags whose children are components, those with
// allowed children in MJML 4.18 (mjml-preset-core/lib/dependencies.js). The
// children of the other tags are content rather than components: the raw
// HTML of an ending tag such as mj-text, or the definitions of mj-attributes.
//
// Which children MJML allows in each of them is not checked: as MJML does
// with its default soft validation, a known component in a place MJML does
// not allow is built and rendered all the same.
var componentContainers = map[string]bool{
	"mj-accordion":         true,
	"mj-accordion-element": true,
	"mj-body":              true,
	"mj-carousel":          true,
	"mj-column":            true,
	"mj-group":             true,
	"mj-head":              true,
	"mj-hero":              true,
	"mj-navbar":            true,
	"mj-section":           true,
	"mj-social":            true,
	"mj-wrapper":           true,
}

// buildChildren creates the child components of parent from node, if its tag
// has component children (see componentContainers), and then theirs, down the
// whole tree. A child whose tag is not a component safe-mjml knows is left out,
// as MJML leaves out unknown elements, and so is a nested mjml element, which
// would reset the options of the document; any other error stops the build.
//
// Each child gets its context (components.ChildContextOf) before its own
// children are built, so widths flow down the tree once, from mj-body to the
// leaves. The children are created one by one, in document order, so that
// their warnings come in that order: their sibling counts are taken from the
// child nodes, as MJML takes them.
func buildChildren(parent components.Component, node *parser.MJMLNode, opts *options.RenderOpts) error {
	if !componentContainers[node.GetTagName()] {
		return nil
	}
	// As mjml-core's renderChildren, every child element counts as a
	// sibling, unknown tags included, and mj-raw as a raw one.
	// MJML 5's parser drops mj-include (ignoreIncludes): it is no sibling,
	// and the index of each child (props.index) counts the others, unknown
	// tags included.
	children := childNodes(node, opts)
	siblings, rawSiblings := 0, 0
	for _, childNode := range children {
		switch childNode.GetTagName() {
		case "mj-include":
			continue
		case "mj-raw":
			rawSiblings++
		}
		siblings++
	}
	ctx := components.ChildContextOf(parent, siblings, rawSiblings)
	index := -1
	for _, childNode := range children {
		if childNode.GetTagName() != "mj-include" {
			index++
		}
		if childNode.GetTagName() == "mjml" {
			continue
		}
		child, err := createComponent(childNode, opts)
		if errors.Is(err, errUnknownComponent) {
			warnUnknownTag(opts, childNode)
			continue
		}
		if err != nil {
			return err
		}
		warnIfMisplaced(opts, node.GetTagName(), childNode)
		if childNode.GetTagName() == "mj-attributes" {
			validateAttributeDefinitions(opts, childNode)
		}
		child.SetContext(ctx)
		if err := parent.AddChild(child); err != nil {
			return err
		}
		components.SetIndex(child, index)
		if err := buildChildren(child, childNode, opts); err != nil {
			return err
		}
	}
	return nil
}

// renderMerged renders c into sb with adjacent Outlook conditionals merged, as
// MJML does. A whole document (rootComponent) merges them itself; a fragment
// root (an mj-section, an mj-body...) is merged here, so every entry point
// gives the same HTML for it.
func renderMerged(c components.Component, sb *strings.Builder) error {
	if err := c.Render(sb); err != nil {
		return err
	}
	if _, document := c.(*rootComponent); !document {
		html.MergeOutlookConditionalsInBuilder(sb)
	}
	return nil
}

// childNodes returns the child elements of a container node, in document
// order, with an mj-raw node for each HTML comment between them, as
// mjml-parser-xml turns the comments of the template into mj-raw children
// (keepComments, true by default); with opts.SkipComments, the elements only.
// A comment where an mj-raw cannot go (mj-carousel, where MJML 4.18 fails on
// it) is dropped, without the warning a misplaced mj-raw would get.
// The parser keeps those comments in the text of the node's mixed content;
// the mj-raw nodes are built for this render, and the AST, which the cache
// shares, is left as it is.
func childNodes(node *parser.MJMLNode, opts *options.RenderOpts) []*parser.MJMLNode {
	if opts.SkipComments || !hasComment(node) || !rawAllowedIn(node.GetTagName()) {
		return node.Children
	}
	children := make([]*parser.MJMLNode, 0, len(node.Children)+1)
	for _, part := range node.MixedContent {
		if part.Node != nil {
			children = append(children, part.Node)
			continue
		}
		text := part.Text
		for {
			start := strings.Index(text, "<!--")
			if start == -1 {
				break
			}
			end := strings.Index(text[start+4:], "-->")
			if end == -1 {
				break
			}
			comment := text[start : start+4+end+3]
			children = append(children, &parser.MJMLNode{
				XMLName:    xml.Name{Local: "mj-raw"},
				Text:       comment,
				LineNumber: node.LineNumber,
			})
			text = text[start+4+end+3:]
		}
	}
	return children
}

// hasComment reports whether the text of node's mixed content holds an HTML
// comment.
func hasComment(node *parser.MJMLNode) bool {
	for _, part := range node.MixedContent {
		if part.Node == nil && strings.Contains(part.Text, "<!--") {
			return true
		}
	}
	return false
}

// rawAllowedIn reports whether an mj-raw can be a child of tag.
func rawAllowedIn(tag string) bool {
	allowed, listed := allowedChildren(tag)
	return !listed || slices.Contains(allowed, "mj-raw")
}

// headTitle returns the text of the head's mj-title, the last one when there
// are several, as mjml-title's handler sets it; "" without a head or title.
// headPreview returns the last mj-preview of head, the one mj-body writes:
// each one replaces the previous one in mjml-head-preview (add('preview')).
func headPreview(head *components.MJHeadComponent) *components.MJPreviewComponent {
	var preview *components.MJPreviewComponent
	if head == nil {
		return preview
	}
	for _, child := range head.GetChildren() {
		if p, ok := child.(*components.MJPreviewComponent); ok {
			preview = p
		}
	}
	return preview
}

func headTitle(head *components.MJHeadComponent) string {
	title := ""
	if head == nil {
		return title
	}
	for _, child := range head.GetChildren() {
		if t, ok := child.(*components.MJTitleComponent); ok {
			title = strings.TrimSpace(t.Node.Text)
		}
	}
	return title
}
