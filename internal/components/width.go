package components

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/Notifuse/safe-mjml/internal/styles"
)

// mjmlWidth is a width as mjml-core's widthParser reads it, with
// parseFloatToInt false, as mjml-column and mjml-group call it: the unit is
// what follows the number, as written ("px" when nothing does), and the
// number is parseFloat's when the unit is exactly "%", parseInt's
// otherwise. Any unit but "%" is read as pixels: "300PX", "50 %" and "40%;"
// are 300, 50 and 40 pixels.
type mjmlWidth struct {
	size styles.Size // a percentage when the unit is "%", else pixels
	unit string      // as written: "px", "%", "PX", " %", "%;"…
}

// parseMJMLWidth reads width as widthParser does. It reports false when
// the number is NaN in MJML (no leading number): safe-mjml then takes the
// default width, as it writes no NaN (Pierre's rule).
func parseMJMLWidth(width string) (mjmlWidth, bool) {
	unit := widthUnit(width)
	if unit == "%" {
		v, ok := jsParseFloat(width)
		if !ok {
			return mjmlWidth{}, false
		}
		return mjmlWidth{size: styles.NewPercentSize(v), unit: unit}, true
	}
	v, ok := widthParseInt(width)
	if !ok {
		return mjmlWidth{}, false
	}
	return mjmlWidth{size: styles.NewPixelSize(v), unit: unit}, true
}

// widthParseInt is parseInt(s) without a radix, as widthParser calls it,
// as a float64 as in JavaScript: JavaScript's spaces skipped (trimJSSpace),
// a sign, then decimal digits, or "0x" and hexadecimal digits (a column of
// "0x10" is 16 pixels wide). It is not saturated at maxLength, so that a
// 999999999999px column keeps its number in its class; only a number too
// long for a float64, which parseInt gives as Infinity, is read as
// maxLength, as safe-mjml writes no Infinity. false where parseInt gives NaN.
func widthParseInt(s string) (float64, bool) {
	t := trimJSSpace(s)
	sign := 1.0
	if t != "" && (t[0] == '-' || t[0] == '+') {
		if t[0] == '-' {
			sign = -1
		}
		t = t[1:]
	}
	var v float64
	if len(t) >= 2 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		digits := 2
		for ; digits < len(t) && hexDigit(t[digits]) >= 0; digits++ {
			v = v*16 + float64(hexDigit(t[digits]))
		}
		if digits == 2 {
			return 0, false // parseInt("0x") is NaN
		}
		v *= sign
	} else {
		var ok bool
		if v, ok = jsParseIntValue(s); !ok {
			return 0, false
		}
	}
	if math.IsInf(v, 0) {
		return math.Copysign(maxLength, v), true
	}
	return v, true
}

// hexDigit returns the value of the hexadecimal digit c, or -1.
func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// trimJSSpace returns s without the spaces JavaScript's parseInt and
// parseFloat skip before a number: ASCII's, the no-break space (as pasted
// from a web page), the other Unicode spaces, the line and paragraph
// separators and the byte order mark.
func trimJSSpace(s string) string {
	return strings.TrimLeftFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r', '\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
			return true
		}
		return r >= '\u2000' && r <= '\u200a'
	})
}

// cssText returns the width as mjml-core writes it in the media queries,
// `${parsedWidth}${unit}`: the number and the unit as written (300PX,
// 40%;).
func (w mjmlWidth) cssText() string {
	return jsNumber(w.size.Value()) + w.unit
}

// jsParseFloat reads the leading number of s as JavaScript's parseFloat:
// spaces skipped, a sign, digits with one dot, an exponent.
func jsParseFloat(s string) (float64, bool) {
	s = trimJSSpace(s)
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	digits := 0
	for end < len(s) && isDigit(s[end]) {
		end++
		digits++
	}
	if end < len(s) && s[end] == '.' {
		end++
		for end < len(s) && isDigit(s[end]) {
			end++
			digits++
		}
	}
	if digits == 0 {
		return 0, false
	}
	if end < len(s) && (s[end] == 'e' || s[end] == 'E') {
		exp := end + 1
		if exp < len(s) && (s[exp] == '+' || s[exp] == '-') {
			exp++
		}
		if exp < len(s) && isDigit(s[exp]) {
			for exp < len(s) && isDigit(s[exp]) {
				exp++
			}
			end = exp
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(s[:end], "."), 64)
	if err != nil {
		// Out of range: ParseFloat returns ±Inf, which safe-mjml saturates
		// like every length.
		return min(max(v, -maxLength), maxLength), true
	}
	return v, true
}

// jsMathRound is JavaScript's Math.round: the integer nearest x, a tie
// toward +Infinity (-0.5 gives -0). math.Round rounds a tie away from zero,
// and math.Floor(x+0.5) loses the bit that 0.49999999999999994 or 2^52+1
// need.
func jsMathRound(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	if r == 0 && x < 0 {
		return math.Copysign(0, -1)
	}
	return r
}

// jsToFixed6 is Number(x.toFixed(6)), mjml-column's normalizeUnitValue
// (MJML 5). toFixed takes the integer n nearest x·10^6 on the exact value of
// x, the larger magnitude on a tie (Number.prototype.toFixed, ECMA-262), where
// strconv.FormatFloat rounds an exact tie to even (1/128 gives 0.007812, not
// 0.007813). From 1e21, toFixed writes String(x), which reads back as x.
func jsToFixed6(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) >= 1e21 {
		return x
	}
	// x·10^6 is exact in 53 + 20 bits.
	scaled := new(big.Float).SetPrec(128).SetFloat64(math.Abs(x))
	scaled.Mul(scaled, new(big.Float).SetPrec(128).SetInt64(1_000_000))
	n, _ := scaled.Int(nil)
	frac := new(big.Float).SetPrec(128).Sub(scaled, new(big.Float).SetPrec(128).SetInt(n))
	if frac.Cmp(big.NewFloat(0.5)) >= 0 {
		n.Add(n, big.NewInt(1))
	}
	digits := n.String()
	if len(digits) <= 6 {
		digits = strings.Repeat("0", 7-len(digits)) + digits
	}
	text := digits[:len(digits)-6] + "." + digits[len(digits)-6:]
	if x < 0 {
		text = "-" + text
	}
	v, _ := strconv.ParseFloat(text, 64)
	return v
}
