package mjml

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/Notifuse/safe-mjml/internal/astcache"
	"github.com/Notifuse/safe-mjml/internal/components"
	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/debug"
	"github.com/Notifuse/safe-mjml/internal/fonts"
	"github.com/Notifuse/safe-mjml/internal/globals"
	"github.com/Notifuse/safe-mjml/internal/html"
	"github.com/Notifuse/safe-mjml/internal/options"
	"github.com/Notifuse/safe-mjml/internal/parser"
)

// calculateOptimalBufferSize determines the optimal buffer size based on template complexity
func calculateOptimalBufferSize(mjmlContent string) int {
	mjmlSize := len(mjmlContent)
	componentCount := strings.Count(mjmlContent, "<mj-")

	// Prevent division by zero for empty MJML content
	if mjmlSize == 0 {
		// Return a reasonable default buffer size for empty input
		return 1024
	}

	// Calculate component density (components per 1000 characters)
	complexity := float64(componentCount) / float64(mjmlSize) * 1000

	if complexity > 10 {
		// Very dense template - needs more buffer per component
		return mjmlSize*5 + componentCount*180
	} else if complexity > 5 {
		// Medium density - balanced approach
		return mjmlSize*4 + componentCount*140
	} else {
		// Light template - more conservative
		return mjmlSize*3 + componentCount*100
	}
}

// render builds the component tree of ast and renders it, with the
// validation warnings found while building it, in document order, also when
// it fails. A document
// without mj-body is an error at its mjml element, as MJML throws
// "Malformed MJML" for it. bufferSize is the size of the output buffer, estimated from the
// template.
func render(ast *parser.MJMLNode, bufferSize int, cfg *config) (string, []options.Warning, error) {
	startTime := time.Now()
	debugEnabled := debug.Enabled()

	renderOpts, warnings := newRenderOpts(cfg)
	component, err := buildComponentTree(ast, renderOpts)
	warnings.flush()
	if err != nil {
		return "", warnings.all, err
	}
	// buildComponentTree refuses any root but mjml, so this is the document.
	root := component.(*rootComponent)
	if root.Body == nil {
		return "", warnings.all, components.NewNodeError(ast, errNoBody)
	}

	// Pre-allocate with complexity-aware sizing.
	if debugEnabled {
		debug.DebugLogWithData("mjml", "render-html-start", "Starting HTML rendering", map[string]any{
			"buffer_size": bufferSize,
		})
	}
	output, err := root.renderString(bufferSize)
	if err != nil {
		if debugEnabled {
			debug.DebugLogError("mjml", "render-html-error", "Failed to render HTML", err)
		}
		return "", warnings.all, err
	}
	if debugEnabled {
		debug.DebugLogWithData("mjml", "render-complete", "MJML rendering completed", map[string]any{
			"output_length": len(output),
			"total_time_ms": time.Since(startTime).Milliseconds(),
		})
	}
	return output, warnings.all, nil
}

// The two templates MJML refuses as "Malformed MJML".
const errNoBody = "malformed MJML: the mjml element has no mj-body"

func errRootNotMJML(tag string) string {
	return "malformed MJML: the root element is " + tag + ", not mjml"
}

// checkDocument returns the error of a parsed template that MJML refuses as
// "Malformed MJML", as the render would give it: a root other than mjml, or
// an mjml without mj-body. Parse checks it, so that a Document always
// renders.
func checkDocument(ast *parser.MJMLNode) error {
	if ast.GetTagName() != "mjml" {
		return components.NewNodeError(ast, errRootNotMJML(ast.GetTagName()))
	}
	if ast.FindFirstChild("mj-body") == nil {
		return components.NewNodeError(ast, errNoBody)
	}
	return nil
}

// parse parses an MJML template, through the cache when there is one.
// parseMJML is the parser; a test replaces it to make Parse panic.
var parseMJML = parser.ParseMJML

func parse(template string, cache *astcache.Cache) (*parser.MJMLNode, error) {
	if debug.Enabled() {
		debug.DebugLogWithData("mjml", "parse-start", "Starting MJML parsing", map[string]any{
			"content_length": len(template),
			"cache":          cache != nil,
		})
	}
	var (
		ast *parser.MJMLNode
		err error
	)
	if cache != nil {
		ast, err = cache.Parse(template, parseMJML)
	} else {
		ast, err = parseMJML(template)
	}
	if err != nil && debug.Enabled() {
		debug.DebugLogError("mjml", "parse-error", "Failed to parse MJML", err)
	}
	return ast, err
}

// warningCollector gathers the validation warnings of one render. Warnings
// found while the component tree is built are held, then sorted by the
// position of their element and delivered once the tree is complete, so callers get them in document order
// whatever the build order (the head is built before the body).
type warningCollector struct {
	caller   func(options.Warning)
	building bool
	pending  []options.Warning
	all      []options.Warning
}

func (wc *warningCollector) handle(w options.Warning) {
	if wc.building {
		wc.pending = append(wc.pending, w)
		return
	}
	wc.deliver(w)
}

func (wc *warningCollector) deliver(w options.Warning) {
	wc.all = append(wc.all, w)
	if wc.caller != nil {
		wc.caller(w)
	}
}

// flush delivers the warnings held during the build, sorted by line.
func (wc *warningCollector) flush() {
	options.SortWarnings(wc.pending)
	pending := wc.pending
	wc.pending = nil
	wc.building = false
	for _, w := range pending {
		wc.deliver(w)
	}
}

// newRenderOpts returns the render options of cfg and installs the
// collector of the render's validation warnings, which also passes them to
// the handler an internal option set, if any. Call flush on it once the
// component tree is built.
func newRenderOpts(cfg *config) (*options.RenderOpts, *warningCollector) {
	renderOpts := cfg.renderOpts()
	collector := &warningCollector{caller: renderOpts.WarningHandler, building: true, all: []options.Warning{}}
	renderOpts.WarningHandler = collector.handle
	return renderOpts, collector
}

// buildComponentTree loads the mj-attributes of the document's mj-head into
// the render options (per render, no global state) and creates the component
// tree. Every public entry point goes through it so they all resolve
// attributes the same way.
func buildComponentTree(ast *parser.MJMLNode, renderOpts *options.RenderOpts) (components.Component, error) {
	globalAttrs := globals.NewGlobalAttributes()
	if headNode := ast.FindFirstChild("mj-head"); headNode != nil {
		globalAttrs.ProcessAttributesFromHead(headNode)
	}
	renderOpts.GlobalAttributes = globalAttrs

	debugEnabled := debug.Enabled()
	if debugEnabled {
		debug.DebugLog("mjml", "component-tree-start", "Creating component tree from AST")
	}
	// As MJML ("Malformed MJML"), a template whose root is not mjml (a
	// fragment such as an mj-section, mj-head, an unknown tag) is an error at
	// its root; the mjml root builds its own head and body.
	if ast.GetTagName() != "mjml" {
		return nil, components.NewNodeError(ast, errRootNotMJML(ast.GetTagName()))
	}
	component, err := createComponent(ast, renderOpts)
	if errors.Is(err, errUnknownComponent) {
		err = components.WrapNodeError(ast, err)
	}
	if err != nil {
		if debugEnabled {
			debug.DebugLogError("mjml", "component-tree-error", "Failed to create component tree", err)
		}
		return nil, err
	}
	if debugEnabled {
		debug.DebugLog("mjml", "component-tree-complete", "Component tree created successfully")
	}
	return component, nil
}

// rootComponent is the component of the mjml root element: the document.
type rootComponent struct {
	*components.BaseComponent
	Head *components.MJHeadComponent
	Body *components.MJBodyComponent
	// beforeDoctype is the content of the root's mj-raw at
	// position="file-start", one per line, which MJML writes before the
	// doctype.
	beforeDoctype    string
	mobileCSSAdded   bool              // Track if mobile CSS has been added
	columnClasses    map[string]string // the width of each column class's media-query rule
	columnClassOrder []string          // Preserve insertion order of column classes
	carouselCSS      strings.Builder   // Collect carousel CSS from components
	hasCarousel      bool              // a carousel registered its head style, even an empty one
}

// GetChildren returns the document's mj-head and mj-body, those it has, so
// that components.Walk sees the whole document.
func (c *rootComponent) GetChildren() []components.Component {
	children := make([]components.Component, 0, 2)
	if c.Head != nil {
		children = append(children, c.Head)
	}
	if c.Body != nil {
		children = append(children, c.Body)
	}
	return children
}

// AddChild returns an error: the document's children are its mj-head and
// mj-body, which the tree builder sets in Head and Body. A child added any
// other way would never be rendered.
func (c *rootComponent) AddChild(child components.Component) error {
	return fmt.Errorf("mjml: cannot add %s to the mjml root, whose children are its mj-head and mj-body", child.GetTagName())
}

// RequestMobileCSS allows components to request mobile CSS to be added
func (c *rootComponent) RequestMobileCSS() {
	c.mobileCSSAdded = true
}

// RegisterCarouselCSS allows carousel components to register their CSS
func (c *rootComponent) RegisterCarouselCSS(css string) {
	c.carouselCSS.WriteString(css)
}

// collectCarouselCSS recursively collects carousel CSS from all components.
// Resets any CSS collected by a previous render so repeated renders of the
// same component instance don't accumulate duplicate carousel CSS.
func (c *rootComponent) collectCarouselCSS() {
	c.carouselCSS.Reset()
	c.hasCarousel = false
	if c.Body != nil {
		c.collectCarouselCSSFromComponent(c.Body)
	}
}

// collectCarouselCSSFromComponent collects the CSS of every carousel in the
// tree rooted at comp, in document order.
func (c *rootComponent) collectCarouselCSSFromComponent(comp components.Component) {
	// In the order mjml-core registers their componentHeadStyle.
	components.WalkInCreationOrder(comp, func(comp components.Component) {
		if carouselComp, ok := comp.(*components.MJCarouselComponent); ok {
			c.hasCarousel = true
			c.carouselCSS.WriteString(carouselComp.GenerateCSS())
		}
	})
}

// collectColumnClasses recursively collects all column classes used in the document
func (c *rootComponent) collectColumnClasses() {
	c.columnClasses = make(map[string]string)
	c.columnClassOrder = c.columnClassOrder[:0]
	if c.Body != nil {
		c.collectColumnClassesFromComponent(c.Body)
	}
}

func (c *rootComponent) registerColumnClass(className, width string) {
	if _, exists := c.columnClasses[className]; !exists {
		c.columnClassOrder = append(c.columnClassOrder, className)
	}
	c.columnClasses[className] = width
}

// collectColumnClassesFromComponent registers the responsive class of every
// mj-column and mj-group in the tree rooted at comp, in document order.
func (c *rootComponent) collectColumnClassesFromComponent(comp components.Component) {
	components.Walk(comp, func(comp components.Component) bool {
		if column, ok := comp.(interface {
			GetColumnClass() (string, string)
		}); ok {
			c.registerColumnClass(column.GetColumnClass())
		}
		return true
	})
}

// defaultBreakpoint is the viewport width at which MJML stops stacking columns
// when the document has no mj-breakpoint.
const defaultBreakpoint = "480px"

// resolveBreakpoint returns the width of the last mj-breakpoint in mj-head, or
// defaultBreakpoint. As in MJML, every mj-breakpoint overwrites the value, so a
// last mj-breakpoint without width brings back the default.
func (c *rootComponent) resolveBreakpoint() string {
	breakpoint := ""
	if c.Head != nil {
		for _, child := range c.Head.GetChildren() {
			if bp, ok := child.(*components.MJBreakpointComponent); ok {
				breakpoint = bp.Node.GetAttribute(constants.MJMLWidth)
			}
		}
	}
	if breakpoint == "" {
		return defaultBreakpoint
	}
	return breakpoint
}

// lowerBreakpoint returns the largest width below the breakpoint, used by the
// max-width media queries, following MJML's makeLowerBreakpoint: the first
// number found minus one pixel ("480px" gives "479px"), or the breakpoint
// unchanged when it holds no number.
func lowerBreakpoint(breakpoint string) string {
	start := strings.IndexFunc(breakpoint, func(r rune) bool { return r >= '0' && r <= '9' })
	if start < 0 {
		return breakpoint
	}
	end := start
	for end < len(breakpoint) && breakpoint[end] >= '0' && breakpoint[end] <= '9' {
		end++
	}
	pixels, err := strconv.Atoi(breakpoint[start:end])
	if err != nil {
		return breakpoint
	}
	return strconv.Itoa(pixels-1) + "px"
}

// generateResponsiveCSS generates responsive CSS for collected column classes
func (c *rootComponent) generateResponsiveCSS(breakpoint string) string {
	var css strings.Builder

	// Standard responsive media query
	css.WriteString("<style type=\"text/css\">@media only screen and (min-width:")
	css.WriteString(breakpoint)
	css.WriteString(") {\n")
	// In the order the columns registered their class, as MJML
	for _, className := range c.columnClassOrder {
		size := c.columnClasses[className]
		// Include both percentage and pixel-based classes
		css.WriteString("        .")
		css.WriteString(className)
		css.WriteString(" { width:")
		css.WriteString(size)
		css.WriteString(" !important; max-width: ")
		css.WriteString(size)
		css.WriteString("; }\n")
	}
	css.WriteString("      }</style>")

	// Mozilla-specific responsive media query
	css.WriteString(`<style media="screen and (min-width:`)
	css.WriteString(html.EscapeAttribute(breakpoint))
	css.WriteString(`)">`)
	first := true
	for _, className := range c.columnClassOrder {
		if !first {
			css.WriteByte(' ')
		}
		first = false

		size := c.columnClasses[className]
		// Include both percentage and pixel-based classes
		css.WriteString(`.moz-text-html .`)
		css.WriteString(className)
		css.WriteString(` { width:`)
		css.WriteString(size)
		css.WriteString(` !important; max-width: `)
		css.WriteString(size)
		css.WriteString(`; }`)
	}
	css.WriteString(`</style>`)

	// <mjml owa="desktop">: the same rules for Outlook on the web, prefixed
	// with [owa], one per line, as mjml-core's forceOWADesktop writes them.
	// The root's attribute as written: mj-attributes do not reach <mjml>.
	if c.Node.GetAttribute("owa") == "desktop" {
		css.WriteString("<style type=\"text/css\">\n")
		for i, className := range c.columnClassOrder {
			if i > 0 {
				css.WriteByte('\n')
			}
			size := c.columnClasses[className]
			css.WriteString("[owa] ." + className + " { width:" + size + " !important; max-width: " + size + "; }")
		}
		css.WriteString("\n</style>")
	}

	return css.String()
}

// extractHeadMetadata collects document-level metadata from mj-head children: the title,
// which the tree builder read into the render options (headTitle), and the custom font
// declarations.
//
// fontDeclarations lists every mj-font with a name and an href, in document order;
// fonts.ImportURLs resolves names declared more than once and names shared with
// GoogleFontsMapping.
func (c *rootComponent) extractHeadMetadata() (title string, fontDeclarations []fonts.Declaration) {
	if c.RenderOpts != nil {
		title = c.RenderOpts.Title
	}
	if c.Head == nil {
		return title, fontDeclarations
	}

	for _, child := range c.Head.GetChildren() {
		switch comp := child.(type) {
		case *components.MJFontComponent:
			// MJML reads the attributes of head components as written:
			// neither mj-attributes nor mj-all reach them.
			getAttr := func(name string) string {
				if attr := comp.Node.GetAttribute(name); attr != "" {
					return attr
				}
				return comp.GetDefaultAttribute(name)
			}

			fontName := getAttr("name")
			fontHref := getAttr("href")
			if fontName != "" && fontHref != "" {
				fontDeclarations = append(fontDeclarations, fonts.Declaration{Name: fontName, Href: fontHref})
			}
		}
	}

	return title, fontDeclarations
}

// generateCustomStyles generates the final mj-style content tag (mjml-core's skeleton)
func (c *rootComponent) generateCustomStyles() string {
	var content strings.Builder

	// Collect all mj-style content (mjml-core's globalData.style). As
	// buildStyleFromTags, the tag is written when there is one mj-style
	// that is not inline, even an empty one, or one whose content MJML
	// loses (<mj-style>…</MJ-STYLE>).
	styles := 0
	if c.Head != nil {
		for _, child := range c.Head.GetChildren() {
			if styleComp, ok := child.(*components.MJStyleComponent); ok {
				if isInlineStyle(styleComp) {
					continue
				}
				styles++
				content.WriteString(strings.TrimSpace(styleComp.Node.Text))
			}
		}
	}
	if styles == 0 {
		return ""
	}
	return fmt.Sprintf(`<style type="text/css">%s</style>`, content.String())
}

// accordionHeadStyle is mjml-accordion's headStyle (MJML 5 selects the
// checkbox by its type inside the media query, and by its tag outside).
func accordionHeadStyle() string {
	return `noinput.mj-accordion-checkbox { display: block! important; }
@media yahoo, only screen and (min-width:0) {
  .mj-accordion-element { display:block; }
  .mj-accordion-checkbox[type="checkbox"], .mj-accordion-less { display: none !important; }
  .mj-accordion-checkbox[type="checkbox"]+* .mj-accordion-title { cursor: pointer; touch-action: manipulation; -webkit-user-select: none; -moz-user-select: none; user-select: none; }
  .mj-accordion-checkbox[type="checkbox"]+* .mj-accordion-content { overflow: hidden; display: none; }
  .mj-accordion-checkbox[type="checkbox"]+* .mj-accordion-more { display: block !important; }
  .mj-accordion-checkbox:checked+* .mj-accordion-content { display: block; }
  .mj-accordion-checkbox:checked+* .mj-accordion-more { display: none !important; }
  .mj-accordion-checkbox:checked+* .mj-accordion-less { display: block !important; }
}
.moz-text-html input.mj-accordion-checkbox+* .mj-accordion-title { cursor: auto; touch-action: auto; -webkit-user-select: auto; -moz-user-select: auto; user-select: auto; }
.moz-text-html input.mj-accordion-checkbox+* .mj-accordion-content { overflow: hidden; display: block; }
.moz-text-html input.mj-accordion-checkbox+* .mj-accordion-ico { display: none; }
@goodbye { @gmail }
`
}

// navbarHeadStyle is mjml-navbar's headStyle.
func navbarHeadStyle(breakpoint string) string {
	return `
        noinput.mj-menu-checkbox { display:block!important; max-height:none!important; visibility:visible!important; }
        @media only screen and (max-width:` + lowerBreakpoint(breakpoint) + `) {
          .mj-menu-checkbox[type="checkbox"] ~ .mj-inline-links { display:none!important; }
          .mj-menu-checkbox[type="checkbox"]:checked ~ .mj-inline-links,
          .mj-menu-checkbox[type="checkbox"] ~ .mj-menu-trigger { display:block!important; max-width:none!important; max-height:none!important; font-size:inherit!important; }
          .mj-menu-checkbox[type="checkbox"] ~ .mj-inline-links > a { display:block!important; }
          .mj-menu-checkbox[type="checkbox"]:checked ~ .mj-menu-trigger .mj-menu-icon-close { display:block!important; }
          .mj-menu-checkbox[type="checkbox"]:checked ~ .mj-menu-trigger .mj-menu-icon-open { display:none!important; }
        }
        `
}

// imageHeadStyle is mjml-image's headStyle.
func imageHeadStyle(breakpoint string) string {
	return `@media only screen and (max-width:` + lowerBreakpoint(breakpoint) + `) {
                table.mj-full-width-mobile { width: 100% !important; }
                td.mj-full-width-mobile { width: auto !important; }
            }
            `
}

// componentHeadStyles returns the head styles of the body's components in
// one <style>, as mjml-core's buildStyleFromComponents writes them: first
// the styles of each instance (componentsHeadStyle: the carousels'), then
// the style of each component that has one (headStyle: mj-image, mj-navbar,
// mj-accordion), once per component, in the order mjml-core registers them,
// as it creates the components (components.WalkInCreationOrder). It returns
// "" when there is none.
func (c *rootComponent) componentHeadStyles(breakpoint string) string {
	if c.Body == nil {
		return ""
	}
	var styles []string
	if c.hasCarousel {
		// A carousel registers its style even when it writes none (no
		// image): MJML then writes an empty <style>.
		styles = append(styles, c.carouselCSS.String())
	}
	registered := map[string]bool{}
	components.WalkInCreationOrder(c.Body, func(comp components.Component) {
		tag := comp.GetTagName()
		if registered[tag] {
			return
		}
		var style string
		switch tag {
		case "mj-image":
			style = imageHeadStyle(breakpoint)
		case "mj-navbar":
			style = navbarHeadStyle(breakpoint)
		case "mj-accordion":
			style = accordionHeadStyle()
		default:
			return
		}
		registered[tag] = true
		styles = append(styles, style)
	})
	if len(styles) == 0 {
		return ""
	}
	return `<style type="text/css">` + strings.Join(styles, "\n") + `</style>`
}

func (c *rootComponent) GetTagName() string {
	return "mjml"
}

// Render implements optimized Writer-based rendering for rootComponent
func (c *rootComponent) Render(w io.StringWriter) error {
	document, err := c.renderString(0)
	if err != nil {
		return err
	}
	_, err = w.WriteString(document)
	return err
}

// renderString renders the document. The writes go through an Outlook merge
// writer, so adjacent Outlook conditionals are merged as they are written, as
// mjml-core merges them over the whole document, and into a buffer of
// sizeHint bytes where the font imports, known once the body is written, are
// inserted in place: the document is not copied.
func (c *rootComponent) renderString(sizeHint int) (string, error) {
	out := &documentBuffer{b: make([]byte, 0, sizeHint+fontImportsReserve)}
	// mjml-core runs juice on the document when it has an inline mj-style,
	// even an empty one, and merges the Outlook conditionals once, after
	// juice: juice reads them unmerged. A merge before juice made markup of
	// what a stray "<![endif]-->" in an mj-raw left in a comment, the Outlook
	// table of the section after it.
	styles := c.inlineStyles()
	w := html.NewOutlookMergeWriter(out)
	if len(styles) > 0 {
		w = html.NewOutlookWriteThrough(out)
	}
	fontsAt, fontTags, err := c.renderDocument(w, out)
	if out.err != nil {
		// The writes that failed stopped the render where they were
		// checked; the error is the buffer's, whatever was returned.
		return "", out.err
	}
	if err != nil {
		return "", err
	}
	if fontTags != "" {
		out.insert(fontsAt, fontTags)
	} else if from, to, ok := html.OutlookPairAt(out.String(), fontsAt); ok && len(styles) == 0 {
		// The merge writer was flushed where the font imports go; with
		// none, the Outlook conditionals on either side are adjacent, and
		// mjml-core merges them, as it merges over the whole document.
		out.remove(from, to)
	}
	if len(styles) > 0 {
		// juice writes the document again. The merge after it also joins
		// the conditionals juice left adjacent, by dropping what separated
		// them, such as a stray close tag.
		juiced, err := juiceDocument(out.String(), styles)
		if err != nil {
			return "", err
		}
		if len(juiced) > maxOutputSize {
			return "", errOutputTooLarge
		}
		return html.MergeOutlookConditionals(juiced), nil
	}
	if out.Len() > maxOutputSize {
		return "", errOutputTooLarge
	}
	return out.String(), nil
}

// maxOutputSize bounds the HTML of a render. A template can make its HTML
// far larger than itself: a value of mj-all or of an mj-class is written on
// every element it applies to, so 150 KB of template wrote 240 MB with 1 GB
// of memory, and MJML does the same. An email over a few megabytes is not
// sent anyway (Gmail clips one at 102 KB).
const maxOutputSize = 16 << 20

var errOutputTooLarge = fmt.Errorf("the HTML would be larger than %d MB, safe-mjml's limit", maxOutputSize>>20)

// fontImportsReserve is room left in the document buffer for the font
// imports, inserted last: those of a few Google fonts.
const fontImportsReserve = 1024

// documentBuffer holds the document as it is rendered. Unlike a
// strings.Builder, it can insert text in the middle in place, and its String
// view does not copy; nothing may be written after String. A write that
// would take it past maxOutputSize fails, and so does every write after it.
type documentBuffer struct {
	b   []byte
	err error
}

func (d *documentBuffer) WriteString(s string) (int, error) {
	if d.err == nil && len(d.b)+len(s) > maxOutputSize {
		d.err = errOutputTooLarge
	}
	if d.err != nil {
		return 0, d.err
	}
	d.b = append(d.b, s...)
	return len(s), nil
}

// Len returns the number of bytes written.
func (d *documentBuffer) Len() int { return len(d.b) }

// String returns the content, without copying it.
func (d *documentBuffer) String() string {
	return unsafe.String(unsafe.SliceData(d.b), len(d.b))
}

// remove removes the bytes from offset from to offset to.
func (d *documentBuffer) remove(from, to int) {
	d.b = append(d.b[:from], d.b[to:]...)
}

// insert inserts s at offset at, moving what follows.
func (d *documentBuffer) insert(at int, s string) {
	if s == "" {
		return
	}
	d.b = append(d.b, s...) // room at the end, reallocating if needed
	copy(d.b[at+len(s):], d.b[at:len(d.b)-len(s)])
	copy(d.b[at:], s)
}

// renderDocument writes the whole HTML document to w, which merges the
// Outlook conditionals into out, except the font imports: it returns them,
// with the offset in out where they go, as they depend on the body.
func (c *rootComponent) renderDocument(w *html.OutlookMergeWriter, out *documentBuffer) (fontsAt int, fontTags string, err error) {
	// Resolved once per render and passed to the head styles that use it.
	breakpoint := c.resolveBreakpoint()
	debugEnabled := debug.Enabled()
	if debugEnabled {
		debug.DebugLog("mjml-root", "render-start", "Starting root MJML component rendering")
	}

	// Collect column classes; widths and sibling counts are in the context
	// each component got when the tree was built.
	if debugEnabled {
		debug.DebugLog("mjml-root", "collect-column-classes", "Collecting column classes for responsive CSS")
	}
	c.collectColumnClasses()
	if debugEnabled {
		debug.DebugLogWithData("mjml-root", "column-classes-collected", "Column classes collected", map[string]any{
			"class_count": len(c.columnClasses),
		})
	}

	// Collect carousel CSS from all carousel components
	if debugEnabled {
		debug.DebugLog("mjml-root", "collect-carousel-css", "Collecting carousel CSS")
	}
	c.collectCarouselCSS()

	// Extract head metadata (title, custom fonts) before rendering body so accessibility
	// attributes can access the document title during body rendering.
	title, fontDeclarations := c.extractHeadMetadata()

	// DOCTYPE and HTML opening - include attributes from MJML root element
	// The root's attributes as written: neither mj-attributes nor mj-all
	// reach <mjml>.
	langValue := c.Node.GetAttribute("lang")
	if langValue == "" {
		langValue = constants.LangUndetermined
	}
	dirValue := c.Node.GetAttribute("dir")
	if dirValue == "" {
		dirValue = constants.DirAuto
	}

	if c.beforeDoctype != "" {
		if _, err := w.WriteString(c.beforeDoctype + "\n"); err != nil {
			return 0, "", err
		}
	}
	if _, err := w.WriteString(`<!doctype html><html lang="` + html.EscapeAttribute(langValue) + `" dir="` + html.EscapeAttribute(dirValue) + `" xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">`); err != nil {
		return 0, "", err
	}
	if _, err := w.WriteString(`<head>`); err != nil {
		return 0, "", err
	}

	if _, err := w.WriteString(`<title>` + title + `</title>`); err != nil {
		return 0, "", err
	}
	if _, err := w.WriteString(`<!--[if !mso]><!--><meta http-equiv="X-UA-Compatible" content="IE=edge"><!--<![endif]-->`); err != nil {
		return 0, "", err
	}
	if _, err := w.WriteString(`<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">`); err != nil {
		return 0, "", err
	}
	if _, err := w.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`); err != nil {
		return 0, "", err
	}

	// Base CSS
	baseCSSText := `<style type="text/css">#outlook a { padding:0; }
      body { margin:0;padding:0;-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%; }
      table, td { border-collapse:collapse;mso-table-lspace:0pt;mso-table-rspace:0pt; }
      img { border:0;height:auto;line-height:100%; outline:none;text-decoration:none;-ms-interpolation-mode:bicubic; }
      p { display:block;margin:13px 0; }</style>`
	if _, err := w.WriteString(baseCSSText); err != nil {
		return 0, "", err
	}

	// MSO conditionals
	msoText := `<!--[if mso]>
    <noscript>
    <xml>
    <o:OfficeDocumentSettings>
      <o:AllowPNG/>
      <o:PixelsPerInch>96</o:PixelsPerInch>
    </o:OfficeDocumentSettings>
    </xml>
    </noscript>
    <![endif]--><!--[if lte mso 11]>
    <style type="text/css">
      .mj-outlook-group-fix { width:100% !important; }
    </style>
    <![endif]-->`
	if _, err := w.WriteString(msoText); err != nil {
		return 0, "", err
	}

	// The font imports go here, once the body shows which fonts it uses.
	if err := w.Flush(); err != nil {
		return 0, "", err
	}
	fontsAt = out.Len()

	// Dynamic responsive CSS based on collected column classes - only if we have columns
	if len(c.columnClasses) > 0 {
		responsiveCSS := c.generateResponsiveCSS(breakpoint)
		if _, err := w.WriteString(responsiveCSS); err != nil {
			return 0, "", err
		}
	}

	// The components' head styles, in one <style> as mjml-core writes them.
	if _, err := w.WriteString(c.componentHeadStyles(breakpoint)); err != nil {
		return 0, "", err
	}

	// Custom styles from mj-style components (mjml-core's skeleton)
	customStyles := c.generateCustomStyles()
	if _, err := w.WriteString(customStyles); err != nil {
		return 0, "", err
	}

	// Render mj-raw components inside head
	if c.Head != nil {
		for _, child := range c.Head.GetChildren() {
			if rawComp, ok := child.(*components.MJRawComponent); ok {
				if err := rawComp.Render(w); err != nil {
					return 0, "", err
				}
			}
		}
	}

	if _, err := w.WriteString(`</head>`); err != nil {
		return 0, "", err
	}

	if err := w.Flush(); err != nil {
		return 0, "", err
	}
	bodyStart := out.Len()

	// mj-body writes <body>, the preview and its div (mjml-body's render
	// since MJML 5): the skeleton writes only what surrounds them.
	// Stream the body directly to w; this is the only time it is rendered.
	if c.Body != nil {
		if err := c.Body.Render(w); err != nil {
			if debugEnabled {
				debug.DebugLogError("mjml-root", "render-body-error", "Failed to render body", err)
			}
			return 0, "", err
		}
	}
	if _, err := w.WriteString(`</html>`); err != nil {
		return 0, "", err
	}
	if err := w.Flush(); err != nil {
		return 0, "", err
	}

	// As mjml-core's buildFontsTags: import each font the body or the inline
	// styles use. The tags are cloned: they must not share the buffer they
	// are inserted into.
	tags := fonts.BuildFontsTags(fonts.ToImport(out.String()[bodyStart:], c.inlineStyles(), fontDeclarations))
	return fontsAt, strings.Clone(tags), nil
}

// inlineStyles returns the content of the mj-style tags with inline="inline".
func (c *rootComponent) inlineStyles() []string {
	var styles []string
	if c.Head == nil {
		return nil
	}
	for _, child := range c.Head.GetChildren() {
		if style, ok := child.(*components.MJStyleComponent); ok {
			if isInlineStyle(style) {
				styles = append(styles, style.Node.Text)
			}
		}
	}
	return styles
}
