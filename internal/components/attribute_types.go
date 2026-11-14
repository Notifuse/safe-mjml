package components

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// typeKind is the type of an attribute in allowedAttributes, as mjml-core's
// initializeType picks it from the spec.
type typeKind uint8

const (
	kindString typeKind = iota // any value
	kindBoolean
	kindEnum
	kindColor
	kindUnit
	kindInteger
)

// attributeType checks an attribute value against its type in
// allowedAttributes (allowed-css-attributes.json), with the results of
// mjml-core's types for mjml-validator's validTypes rule. The common values
// are answered without a regular expression; mjml-core's expressions are
// compiled, on first use, only for the rare forms they alone decide.
type attributeType struct {
	kind    typeKind
	enum    map[string]struct{} // enum: the values it lists
	unit    unitSpec            // unit: its units and number of values
	message string              // the error message, $value replaced by the value
}

// unitSpec is a unit(units){min,max} or unitWithNegative(…) spec.
type unitSpec struct {
	units      []string // without auto; "" allows a bare number
	auto       bool
	negative   bool
	min, max   int
	expression func() *regexp.Regexp // mjml-core's, compiled on first use
}

// colorNames are the CSS color names mjml-core accepts
// (types/helpers/colors.js), case-sensitive.
var colorNames = func() map[string]struct{} {
	names := strings.Split("aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo inherit ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen steelblue tan teal thistle tomato transparent turquoise violet wheat white whitesmoke yellow yellowgreen", " ")
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	return set
}()

// jsSpace is JavaScript's \s, which is wider than RE2's.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// rgbExpressions are mjml-core's rgba() and rgb() color expressions,
// unanchored: they match anywhere in the value. Compiled on first use.
var rgbExpressions = sync.OnceValue(func() [2]*regexp.Regexp {
	return [2]*regexp.Regexp{
		regexp.MustCompile(`(?i)rgba\(\d{1,3},` + jsSpace + `?\d{1,3},` + jsSpace + `?\d{1,3},` + jsSpace + `?\d(\.\d{1,3})?\)`),
		regexp.MustCompile(`(?i)rgb\(\d{1,3},` + jsSpace + `?\d{1,3},` + jsSpace + `?\d{1,3}\)`),
	}
})

// newAttributeType builds the checker of spec as mjml-core's initializeType:
// the first type whose matcher matches the spec, in the order of
// types/index.js (boolean, enum, color, unit, string, integer). Its message
// is mjml-core's, without the trailing space.
func newAttributeType(spec string) *attributeType {
	lower := strings.ToLower(spec)
	switch {
	case strings.HasPrefix(lower, "boolean"):
		return &attributeType{kind: kindBoolean, message: "has invalid value: $value for type Boolean"}
	case strings.HasPrefix(lower, "enum"):
		values := strings.Split(parenthesized(spec), ",")
		t := &attributeType{kind: kindEnum, enum: make(map[string]struct{}, len(values)),
			message: "has invalid value: $value for type Enum, only accepts " + strings.Join(values, ", ")}
		for _, v := range values {
			t.enum[v] = struct{}{}
		}
		return t
	case strings.HasPrefix(lower, "color"):
		return &attributeType{kind: kindColor, message: "has invalid value: $value for type Color"}
	case isUnitSpec(lower):
		return newUnitType(spec)
	case strings.HasPrefix(lower, "string"):
		return &attributeType{kind: kindString}
	case strings.HasPrefix(lower, "integer"):
		return &attributeType{kind: kindInteger, message: "has invalid value: $value for type NInteger"}
	}
	return &attributeType{kind: kindString} // a spec mjml-core has no type for
}

// isUnitSpec is mjml-core's /^(unit|unitWithNegative)\(.*\)/i.
func isUnitSpec(lower string) bool {
	return (strings.HasPrefix(lower, "unit(") || strings.HasPrefix(lower, "unitwithnegative(")) &&
		strings.Contains(lower, ")")
}

// newUnitType builds a unit(units){min,max} checker as mjml-core's
// types/unit.js: numbers made of digits, commas and dots, each followed by
// one of the units (an empty one allows none), or 0, or auto when listed,
// between min and max of them separated by an optional space.
func newUnitType(spec string) *attributeType {
	u := unitSpec{negative: strings.HasPrefix(spec, "unitWithNegative")}
	units := strings.Split(parenthesized(spec), ",")
	args := []string{"1"}
	if open := strings.IndexByte(spec, '{'); open >= 0 {
		if end := strings.IndexByte(spec[open:], '}'); end > 1 {
			args = strings.Split(spec[open+1:open+end], ",")
		}
	}
	var quoted []string
	for _, unit := range units {
		if unit == "auto" {
			u.auto = true
			continue
		}
		u.units = append(u.units, unit)
		quoted = append(quoted, regexp.QuoteMeta(unit))
	}
	u.min, _ = strconv.Atoi(args[0])
	u.max = u.min
	if len(args) > 1 {
		u.max, _ = strconv.Atoi(args[1])
	}
	negative, auto := "", ""
	if u.negative {
		negative = "-|"
	}
	if u.auto {
		auto = "|auto"
	}
	pattern := `^(((` + negative + `\d|,|\.){1,}(` + strings.Join(quoted, "|") + `)|0` + auto + `)( )?){` + strings.Join(args, ",") + `}$`
	u.expression = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(pattern) })
	return &attributeType{
		kind:    kindUnit,
		unit:    u,
		message: "has invalid value: $value for type Unit, only accepts (" + strings.Join(units, ", ") + ") units and " + strings.Join(args, " to ") + " value(s)",
	}
}

// parenthesized returns what is between the first ( and the next ) of spec.
func parenthesized(spec string) string {
	open := strings.IndexByte(spec, '(')
	if open < 0 {
		return ""
	}
	end := strings.IndexByte(spec[open:], ')')
	if end < 0 {
		return ""
	}
	return spec[open+1 : open+end]
}

// valid reports whether value has the type, as mjml-core decides it.
func (t *attributeType) valid(value string) bool {
	switch t.kind {
	case kindBoolean:
		return value == "true" || value == "false"
	case kindEnum:
		_, ok := t.enum[value]
		return ok
	case kindColor:
		return validColor(value)
	case kindUnit:
		return t.unit.simple(value) || t.unit.expression().MatchString(value)
	case kindInteger:
		return strings.ContainsAny(value, "0123456789") // /\d+/, anywhere
	}
	return true
}

// validColor is mjml-core's color type: #rgb or #rrggbb in any case, a
// color name as written, or rgb(…) or rgba(…) anywhere in the value.
func validColor(value string) bool {
	if isHexColorValue(value) {
		return true
	}
	if _, ok := colorNames[value]; ok {
		return true
	}
	if !strings.Contains(strings.ToLower(value), "rgb") {
		return false
	}
	for _, e := range rgbExpressions() {
		if e.MatchString(value) {
			return true
		}
	}
	return false
}

// isHexColorValue is /^#([0-9a-f]{3}){1,2}$/i.
func isHexColorValue(value string) bool {
	if len(value) != 4 && len(value) != 7 || value[0] != '#' {
		return false
	}
	for i := 1; i < len(value); i++ {
		c := value[i] | 0x20
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// simple reports whether value is made of whole tokens of the spec separated
// by single spaces: a sufficient condition for mjml-core's expression, which
// decides the other values (glued tokens, a trailing space…).
func (u *unitSpec) simple(value string) bool {
	if value == "" {
		return false
	}
	n := 0
	for token := range strings.SplitSeq(value, " ") {
		n++
		if n > u.max || !u.token(token) {
			return false
		}
	}
	return n >= u.min
}

// token reports whether s is 0, auto when listed, or a number of digits,
// commas, dots (and minus signs for unitWithNegative) followed by one of the
// units.
func (u *unitSpec) token(s string) bool {
	if s == "0" || u.auto && s == "auto" {
		return true
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == ',' || s[i] == '.' || u.negative && s[i] == '-') {
		i++
	}
	if i == 0 {
		return false
	}
	for _, unit := range u.units {
		if s[i:] == unit {
			return true
		}
	}
	return false
}

// check returns "" when value is valid, else the message of the warning,
// "has invalid value: …" as mjml-core writes it, without the trailing space
// of an enum that lists an empty value (only accepts full-width, false, ).
func (t *attributeType) check(value string) string {
	if t == nil || t.valid(value) {
		return ""
	}
	return strings.TrimRight(strings.ReplaceAll(t.message, "$value", value), " ")
}
