package juicer

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Outlook ignores CSS width, height and background on table elements, so
// juice mirrors those declarations into presentational attributes.
//
// Two details are load-bearing. These read the resolved property map rather
// than reparsing the style attribute, so they see values after variable
// substitution. And they only look at the head of a duplicate chain.

var widthHeightElements = map[string]bool{
	"table": true, "td": true, "th": true, "img": true,
}

var tableElements = map[string]bool{
	"table": true, "th": true, "tr": true, "td": true, "caption": true,
	"colgroup": true, "col": true, "thead": true, "tbody": true, "tfoot": true,
}

var styleToAttribute = map[string]string{
	"background-color": "bgcolor",
	"background-image": "background",
	"text-align":       "align",
	"vertical-align":   "valign",
}

func (in *pass) promoteAttributes(root *html.Node) {
	o := in.o
	var heads []int32
	for _, el := range in.resolved {
		n, m := el.node, el.props
		if o.applyWidthAttributes {
			in.setDimension(n, m, "width")
		}
		if o.applyHeightAttributes {
			in.setDimension(n, m, "height")
		}
		if o.applyAttributesTableElement && tableElements[foldedName(n)] {
			// juice walks the element's properties, so attributes land in
			// property order rather than table order.
			heads = m.order(heads[:0])
			for _, i := range heads {
				attr, ok := styleToAttribute[m.slots[i].prop]
				if !ok {
					continue
				}
				v := in.attrValue(m.slots[i].value)
				if attr == "background" {
					v = extractURL(v)
				}
				// A gradient cannot go in a presentational attribute.
				if l := strings.ToLower(v); strings.Contains(l, "linear-gradient(") || strings.Contains(l, "radial-gradient(") {
					continue
				}
				// Browsers drop an invalid color, but bgcolor's legacy
				// parsing turns almost any text into one.
				if attr == "bgcolor" && !o.promoteInvalidColors && !maybeColor(v) {
					continue
				}
				setAttr(n, attr, v)
			}
		}
	}
}

// maybeColor rejects colors with a malformed hex code or a wrong count of
// rgb() or hsl() components.
// ponytail: other values pass, unknown keywords included; a full CSS color
// parser if those turn up.
func maybeColor(v string) bool {
	v = strings.TrimSpace(v)
	if hex, ok := strings.CutPrefix(v, "#"); ok {
		_, err := strconv.ParseUint(hex, 16, 64)
		return err == nil && (len(hex) == 3 || len(hex) == 4 || len(hex) == 6 || len(hex) == 8)
	}
	name, args, _ := strings.Cut(v, "(")
	switch strings.ToLower(name) {
	case "rgb", "rgba", "hsl", "hsla":
	default:
		return true
	}
	args, ok := strings.CutSuffix(args, ")")
	if !ok {
		return false
	}
	// Nested calls such as calc() count as one component.
	var flat strings.Builder
	depth := 0
	for i := range len(args) {
		if args[i] == '(' {
			depth++
		}
		if depth == 0 {
			flat.WriteByte(args[i])
		}
		if args[i] == ')' && depth > 0 {
			depth--
		}
	}
	args = flat.String()
	if strings.Contains(args, ",") {
		n := strings.Count(args, ",") + 1
		return n == 3 || n == 4
	}
	rgb, alpha, hasAlpha := strings.Cut(args, "/")
	return len(strings.Fields(rgb)) == 3 && (!hasAlpha || len(strings.Fields(alpha)) == 1)
}

// attrValue drops a preserved !important, which has no place in an attribute.
func (in *pass) attrValue(v []byte) string {
	s := string(v)
	if strings.HasSuffix(s, "!important") {
		s = strings.TrimRight(strings.TrimSuffix(s, "!important"), " \t\n\r\f")
	}
	return s
}

func (in *pass) setDimension(n *html.Node, m *propMap, dim string) {
	if !widthHeightElements[foldedName(n)] {
		return
	}
	i := m.find(dim)
	if i < 0 {
		return
	}
	v := in.attrValue(m.slots[i].value)
	// juice tests for px or auto anywhere in the value, then strips the first
	// "px". That really does turn `height: auto` into height="auto".
	if strings.Contains(v, "px") || strings.Contains(v, "auto") {
		setAttr(n, dim, strings.Replace(v, "px", "", 1))
		return
	}
	if tableElements[foldedName(n)] && strings.Contains(v, "%") {
		setAttr(n, dim, v)
	}
}

// foldedName is the element name for juice's table and dimension lists, which
// it checks case-insensitively; names only keep their case in XML mode.
func foldedName(n *html.Node) string {
	if strings.IndexFunc(n.Data, func(r rune) bool { return 'A' <= r && r <= 'Z' }) < 0 {
		return n.Data
	}
	return strings.ToLower(n.Data)
}

// extractURL unwraps url(x), url('x') or url("x") to x, exactly as juice's
// /^url\((["'])?([^"']+)\1\)$/ does: whitespace is kept, and anything else,
// including `none` or an empty url(""), passes through unchanged.
func extractURL(v string) string {
	inner, ok := strings.CutPrefix(v, "url(")
	if !ok {
		return v
	}
	if inner, ok = strings.CutSuffix(inner, ")"); !ok {
		return v
	}
	if len(inner) >= 2 && (inner[0] == '"' || inner[0] == '\'') && inner[len(inner)-1] == inner[0] {
		inner = inner[1 : len(inner)-1]
	}
	if inner == "" || strings.ContainsAny(inner, `"'`) {
		return v
	}
	return inner
}
