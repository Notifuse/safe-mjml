package components

import (
	gohtml "github.com/Notifuse/safe-mjml/internal/html"
	"strconv"
	"strings"
)

// parseBackgroundPosition converts CSS keywords/percent/length into canonical (xKeyword, yKeyword)
func parseBackgroundPosition(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "top", "center" // fallback consistent with MJML default "top center"
	}
	toks := strings.Fields(raw)
	if len(toks) == 1 {
		v := toks[0]
		if v == "top" || v == "bottom" {
			return "center", v
		}
		return v, "center"
	}
	if len(toks) >= 2 {
		// Determine which is x vs y similar to MJML JS logic
		v1, v2 := toks[0], toks[1]
		if isVertical(v1) || (v1 == "center" && isHorizontal(v2)) {
			return v2, v1
		}
		return v1, v2
	}
	return "center", "center"
}

func isHorizontal(v string) bool {
	switch v {
	case "left", "right":
		return true
	}
	return false
}

func isVertical(v string) bool {
	switch v {
	case "top", "bottom":
		return true
	}
	return false
}

// overridePosition applies background-position-x / -y overrides
func overridePosition(x, y, posXOverride, posYOverride string) (string, string) {
	if posXOverride != "" {
		x = posXOverride
	}
	if posYOverride != "" {
		y = posYOverride
	}
	return x, y
}

func buildBackgroundShorthand(color, url, posX, posY, size, repeat string) string {
	// Similar to MJML: color + url('...') + posX posY / size + repeat
	parts := []string{}
	// Only include color if it's explicitly provided (including "transparent")
	if color != "" {
		parts = append(parts, color)
	}
	if url != "" {
		parts = append(parts, "url('"+htmlEscape(url)+"')")
	}
	if posX != "" || posY != "" {
		position := posX + " " + posY
		// Include size in the position part with slash delimiter if we have a size
		if size != "" {
			position += " / " + size
		}
		parts = append(parts, position)
	}
	if repeat != "" {
		parts = append(parts, repeat)
	}
	return strings.Join(parts, " ")
}

func computeVMLType(repeat, size string) string {
	if repeat == "no-repeat" && size != "auto" {
		return "frame"
	}
	return "tile"
}

func computeVMLSize(size string) (sizeAttr string, aspect string) {
	switch size {
	case "cover":
		return `size="1,1"`, "atleast"
	case "contain":
		return `size="1,1"`, "atmost"
	case "", "auto":
		return "", ""
	}
	parts := strings.Fields(size)
	if len(parts) == 1 {
		return `size="` + htmlEscape(parts[0]) + `"`, "atmost"
	}
	if len(parts) >= 2 {
		return `size="` + htmlEscape(parts[0]+`,`+parts[1]) + `"`, ""
	}
	return "", ""
}

// AIDEV-NOTE: VML positioning depends on background-repeat mode - see docs/vml-background-positioning.md
func computeVMLPosition(posX, posY, size string, repeat string) (originX, originY, posValX, posValY string) {
	// With background-size auto, MJML keeps its old behavior whatever the
	// position: a tiled image anchored at "0.5, 0" (Outlook cannot use the
	// image's own size in frame mode).
	if size == "auto" {
		return "0.5", "0", "0.5", "0"
	}
	// Follows mjml-section's renderWithBackground (MJML 4.18). Keywords become
	// percentages and any other non-percentage value falls back to 50% (x) or
	// 0% (y). Only background-repeat="repeat" maps the percentage directly;
	// every other value (no-repeat, repeat-x, repeat-y) shifts it by -0.5.
	x := normalizeVMLPercent(posX, vmlKeywordsX, "50%")
	y := normalizeVMLPercent(posY, vmlKeywordsY, "0%")
	tile := repeat == "repeat"
	vx := vmlPercentToDecimal(x, tile)
	vy := vmlPercentToDecimal(y, tile)
	return vx, vy, vx, vy
}

// Background-position keywords as percentages, per axis.
var (
	vmlKeywordsX = map[string]string{"left": "0%", "center": "50%", "right": "100%"}
	vmlKeywordsY = map[string]string{"top": "0%", "center": "50%", "bottom": "100%"}
)

// normalizeVMLPercent maps a background-position keyword to its percentage,
// keeps a percentage as is, and replaces anything else with fallback.
func normalizeVMLPercent(value string, keywords map[string]string, fallback string) string {
	if percent, ok := keywords[value]; ok {
		return percent
	}
	if isPlainPercentage(value) {
		return value
	}
	return fallback
}

// isPlainPercentage matches /^\d+(\.\d+)?%$/ like MJML's isPercentage.
func isPlainPercentage(value string) bool {
	number, ok := strings.CutSuffix(value, "%")
	if !ok || number == "" {
		return false
	}
	integer, fraction, hasDot := strings.Cut(number, ".")
	if integer == "" || !allDigits(integer) {
		return false
	}
	return !hasDot || (fraction != "" && allDigits(fraction))
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// vmlPercentToDecimal converts a percentage checked by isPlainPercentage to
// the VML origin/position value, with the same float operations as MJML:
// parseInt of the percentage (decimals are dropped) divided by 100, then, when
// not tiling, (-50 + decimal * 100) / 100.
func vmlPercentToDecimal(percent string, tile bool) string {
	integer, _, _ := strings.Cut(strings.TrimSuffix(percent, "%"), ".")
	n, err := strconv.Atoi(integer)
	if err != nil {
		n = 0
	}
	decimal := float64(n) / 100
	if !tile {
		// The explicit conversion rounds decimal*100 before the addition, as
		// JavaScript does; without it Go may fuse the two operations (FMA on
		// arm64) and print -0.15999999999999998 instead of -0.16.
		decimal = (-50 + float64(decimal*100)) / 100
	}
	return strconv.FormatFloat(decimal, 'f', -1, 64)
}

// htmlEscape returns s ready for a double-quoted attribute: s is kept as
// written in the template, so only a raw double quote is escaped
// (html.EscapeAttribute).
func htmlEscape(s string) string {
	return gohtml.EscapeAttribute(s)
}
