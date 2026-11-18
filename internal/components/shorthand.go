package components

import (
	"math"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/constants"
	"github.com/Notifuse/safe-mjml/internal/html"
)

// boxSide is one side of a box, as in the four-value form of padding.
type boxSide int

const (
	sideTop boxSide = iota
	sideRight
	sideBottom
	sideLeft
)

// suffix returns the side as it ends a longhand attribute name, such as
// padding-left.
func (s boxSide) suffix() string {
	return [...]string{"-top", "-right", "-bottom", "-left"}[s]
}

// sideAttributeNames holds the longhand names of the shorthand attributes
// whose sides the box computations read, so that reading them builds no
// string.
var sideAttributeNames = map[string][4]string{
	constants.MJMLPadding:      {"padding-top", "padding-right", "padding-bottom", "padding-left"},
	constants.MJMLBorder:       {"border-top", "border-right", "border-bottom", "border-left"},
	constants.MJMLInnerBorder:  {"inner-border-top", "inner-border-right", "inner-border-bottom", "inner-border-left"},
	constants.MJMLInnerPadding: {"inner-padding-top", "inner-padding-right", "inner-padding-bottom", "inner-padding-left"},
}

// sideAttribute returns the longhand attribute of one side of attribute,
// such as padding-left.
func sideAttribute(attribute string, side boxSide) string {
	if names, ok := sideAttributeNames[attribute]; ok {
		return names[side]
	}
	return attribute + side.suffix()
}

// shorthandAttrValue returns, in whole pixels, one side of a shorthand
// attribute such as padding, as mjml-core's getShorthandAttrValue does: the
// side's own attribute (padding-left) when it is set, otherwise that side's
// value in the shorthand, which has one to four values as in CSS. Values are
// read like JavaScript's parseInt reads them ("20px" is 20, "1.5px" is 1). An
// unset or unreadable value is 0, where MJML would compute with NaN.
func (bc *BaseComponent) shorthandAttrValue(comp Component, attribute string, side boxSide) int {
	if value := bc.resolveAttribute(comp, sideAttribute(attribute, side)); value != "" {
		return parseIntPrefix(value)
	}
	return shorthandSide(bc.resolveAttribute(comp, attribute), side)
}

// shorthandBorderValue returns, in whole pixels, the width of a border
// attribute (border, inner-border) on one side, as mjml-core's
// getShorthandBorderValue does: from the side's own attribute (border-left)
// when set, otherwise from the shorthand.
func (bc *BaseComponent) shorthandBorderValue(comp Component, attribute string, side boxSide) int {
	border := bc.resolveAttribute(comp, sideAttribute(attribute, side))
	if border == "" {
		border = bc.resolveAttribute(comp, attribute)
	}
	return borderWidth(border)
}

// boxWidth returns the width a box component (mj-section, mj-wrapper) gives
// its children: its own width, as comp computes it, minus its left and right
// padding and borders, as mjml-core's getBoxWidths computes it. Where that is
// negative, SetContext gives the children 0.
func (bc *BaseComponent) boxWidth(comp Component) int {
	return sumLengths(comp.GetEffectiveWidth(),
		-bc.shorthandAttrValue(comp, constants.MJMLPadding, sideLeft),
		-bc.shorthandAttrValue(comp, constants.MJMLPadding, sideRight),
		-bc.shorthandBorderValue(comp, constants.MJMLBorder, sideLeft),
		-bc.shorthandBorderValue(comp, constants.MJMLBorder, sideRight))
}

// borderWidth returns the width of a border such as "2px solid #000", as
// mjml-core's borderParser reads it: the first run of digits at the start or
// after a space; 0 when there is none.
func borderWidth(border string) int {
	for i := 0; i < len(border); i++ {
		if isDigit(border[i]) && (i == 0 || border[i-1] == ' ') {
			return parseIntPrefix(border[i:])
		}
	}
	return 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// shorthandSide returns one side of a CSS-like shorthand value, as mjml-core's
// shorthandParser does.
func shorthandSide(value string, side boxSide) int {
	parts := strings.Fields(value)
	if len(parts) > 4 {
		parts = parts[:4]
	}
	switch len(parts) {
	case 0:
		return 0
	case 1:
		return parseIntPrefix(value) // one value, for every side
	}
	// Index of each side's value for 2, 3 and 4 values.
	var index int
	switch side {
	case sideTop:
		index = 0
	case sideRight:
		index = 1
	case sideBottom:
		index = [5]int{2: 0, 3: 2, 4: 2}[len(parts)]
	case sideLeft:
		index = [5]int{2: 1, 3: 1, 4: 3}[len(parts)]
	}
	return parseIntPrefix(parts[index])
}

// parseIntPrefix reads the integer at the start of s, after leading
// whitespace and an optional sign, as JavaScript's parseInt(s, 10) does; it
// returns 0 when there is none.
func parseIntPrefix(s string) int {
	n, _ := leadingInt(s)
	return n
}

// parseIntText returns s as JavaScript's parseInt writes it back: its
// leading integer (spaces skipped, a sign, digits), without leading zeros.
// When s does not start with one, parseInt gives NaN, which safe-mjml does not
// write: s is returned as written (Pierre's rule: no NaN). The digits
// are kept as written however many there are, rather than as a number
// that would be saturated or written with an exponent.
func parseIntText(s string) string {
	rest := trimJSSpace(s)
	sign := ""
	if rest != "" && (rest[0] == '-' || rest[0] == '+') {
		if rest[0] == '-' {
			sign = "-"
		}
		rest = rest[1:]
	}
	end := 0
	for end < len(rest) && isDigit(rest[end]) {
		end++
	}
	if end == 0 {
		return s
	}
	digits := strings.TrimLeft(rest[:end], "0")
	if digits == "" {
		return "0"
	}
	return sign + digits
}

// maxLength is the largest length, in pixels, safe-mjml computes with. Longer
// values, which only a mistyped or hostile template holds (MJML itself would
// compute with a float such as 1e20), are read as maxLength. Sums and
// differences of lengths go through sumLengths, or float64, so that they
// cannot overflow a 32-bit int.
const maxLength = 1<<31 - 1

// sumLengths adds lengths in int64 and saturates the sum at ±maxLength,
// so that it cannot overflow, whatever the size of int.
func sumLengths(lengths ...int) int {
	var sum int64
	for _, length := range lengths {
		sum += int64(length)
	}
	return int(min(max(sum, -maxLength), maxLength))
}

// leadingInt is parseIntPrefix, also reporting whether s starts with an
// integer, where JavaScript's parseInt would return NaN. The value is
// saturated at ±maxLength.
func leadingInt(s string) (int, bool) {
	s = trimJSSpace(s)
	sign := 1
	if s != "" && (s[0] == '-' || s[0] == '+') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	n, digits := 0, 0
	for ; digits < len(s) && isDigit(s[digits]); digits++ {
		d := int(s[digits] - '0')
		if n > (maxLength-d)/10 { // n*10+d would pass maxLength, or overflow a 32-bit int
			n = maxLength
			continue
		}
		n = n*10 + d
	}
	return sign * n, digits > 0
}

// saturateInf returns v, or ±maxLength for ±Infinity, which a parseInt of
// more than 308 digits gives in MJML: safe-mjml writes no Infinity.
func saturateInf(v float64) float64 {
	switch {
	case math.IsInf(v, 1):
		return maxLength
	case math.IsInf(v, -1):
		return -maxLength
	}
	return v
}

// lengthFromFloat converts a length in pixels parsed as a float to an int,
// truncated like the integer lengths MJML writes, and saturated at
// ±maxLength like leadingInt; NaN is 0.
func lengthFromFloat(v float64) int {
	switch {
	case v != v: // NaN
		return 0
	case v >= maxLength:
		return maxLength
	case v <= -maxLength:
		return -maxLength
	}
	return int(v)
}

// addPaddingLonghands writes the padding longhands (padding-top…) set on
// the component, in the given order, resolved like the shorthandAttrValue
// reading that sizes the content: the element, mj-class and mj-attributes.
func (bc *BaseComponent) addPaddingLonghands(comp Component, tag *html.HTMLTag, sides ...boxSide) {
	for _, side := range sides {
		name := sideAttribute(constants.MJMLPadding, side)
		if value := bc.resolveAttribute(comp, name); value != "" {
			tag.AddStyle(name, value)
		}
	}
}

// widthUnit returns the unit of a width as mjml-core's widthParser reads it
// (/[\d.,]*(\D*)$/): what follows its last digit, past any "." or ","
// right after it, or "px" when that is empty (a bare number, or no width).
func widthUnit(width string) string {
	unit := width[strings.LastIndexFunc(width, func(r rune) bool { return r >= '0' && r <= '9' })+1:]
	if unit = strings.TrimLeft(unit, ".,"); unit == "" {
		return "px"
	}
	return unit
}

// jsParseInt writes parseInt(s, 10) as JavaScript prints it: the integer s
// starts with, or NaN. Unlike leadingInt, it does not saturate.
func jsParseInt(s string) string {
	v, ok := jsParseIntValue(s)
	if !ok {
		return "NaN"
	}
	return jsNumber(v)
}

// jsParseIntValue is parseInt(s, 10) as a float64, as JavaScript computes
// it: the integer s starts with, after leading whitespace and an optional
// sign, rounded once to the nearest float64 (strconv.ParseFloat, as
// JavaScript does, where accumulating digit by digit would round at every
// step, and differently on CPUs that fuse the multiply and add); false
// where parseInt returns NaN.
func jsParseIntValue(s string) (float64, bool) {
	s = trimJSSpace(s)
	sign := ""
	if s != "" && (s[0] == '-' || s[0] == '+') {
		sign, s = s[:1], s[1:]
	}
	digits := 0
	for digits < len(s) && isDigit(s[digits]) {
		digits++
	}
	if digits == 0 {
		return 0, false
	}
	// A decimal integer always parses; a huge one gives ±Inf, as
	// parseInt gives ±Infinity.
	v, _ := strconv.ParseFloat(sign+s[:digits], 64)
	return v, true
}

// parseIntAttribute writes an HTML attribute MJML writes with parseInt
// (the width of an mj-table, of a carousel thumbnail): the integer value
// starts with, or, where parseInt gives NaN (an empty value), the value as
// written. safe-mjml writes no NaN or Infinity width (TestRenderInvariants), as
// it does not copy MJML's NaN for an empty column width either.
func parseIntAttribute(value string) string {
	// Infinity, for an integer too long for a float64, is kept out the same
	// way: the value as written.
	if v, ok := jsParseIntValue(value); !ok || math.IsInf(v, 0) {
		return value
	}
	return jsParseInt(value)
}
