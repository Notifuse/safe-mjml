package testutils

import "strings"

// EffectiveDeclarations parses an inline style and returns the declarations
// that take effect, as property → value. A property declared twice keeps its
// last value, and a longhand declared before a shorthand that resets it
// (padding-top before padding, border-left before border) is dropped, since
// the shorthand overrides it. Two styles with the same declarations in another
// order compare equal only when the order does not change what applies:
// "padding:2px;padding-top:40px" and "padding-top:40px;padding:2px" do not.
//
// As in CSS, a declaration marked !important wins over a later one that is
// not, and is not reset by a shorthand that is not. Declarations are split
// on the semicolons outside quotes and parentheses: url("a;b.png") is one
// value.
func EffectiveDeclarations(style string) map[string]string {
	props := make(map[string]string)
	important := make(map[string]bool)
	for _, decl := range splitDeclarations(style) {
		prop, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		prop = strings.TrimSpace(prop)
		if prop == "" {
			continue
		}
		value = strings.TrimSpace(value)
		isImportant := isImportantValue(value)
		if important[prop] && !isImportant {
			continue
		}
		for earlier := range props {
			if resetBy(earlier, prop) && (isImportant || !important[earlier]) {
				delete(props, earlier)
				delete(important, earlier)
			}
		}
		props[prop] = value
		important[prop] = isImportant
	}
	return props
}

// isImportantValue reports whether a declaration's value ends with
// !important (with optional whitespace, as in "red ! important").
func isImportantValue(value string) bool {
	rest, ok := strings.CutSuffix(strings.ToLower(strings.TrimSpace(value)), "important")
	if !ok {
		return false
	}
	return strings.HasSuffix(strings.TrimSpace(rest), "!")
}

// splitDeclarations splits a style attribute on its semicolons, except
// those inside quotes or parentheses.
func splitDeclarations(style string) []string {
	var decls []string
	var quote byte
	depth, start := 0, 0
	for i := 0; i < len(style); i++ {
		c := style[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')' && depth > 0:
			depth--
		case c == ';' && depth == 0:
			decls = append(decls, style[start:i])
			start = i + 1
		}
	}
	return append(decls, style[start:])
}

// resetBy reports whether declaring shorthand resets longhand.
func resetBy(longhand, shorthand string) bool {
	if longhand == shorthand {
		return false
	}
	switch shorthand {
	case "padding", "margin", "background", "font", "outline", "inset", "list-style", "flex", "text-decoration":
		return strings.HasPrefix(longhand, shorthand+"-") || (shorthand == "font" && longhand == "line-height")
	case "border":
		// border resets the width, style and colour of every side, not
		// border-radius, border-collapse, border-spacing or border-image.
		rest, ok := strings.CutPrefix(longhand, "border-")
		if !ok {
			return false
		}
		switch rest {
		case "width", "style", "color", "top", "right", "bottom", "left":
			return true
		}
		for _, side := range []string{"top-", "right-", "bottom-", "left-"} {
			if part, ok := strings.CutPrefix(rest, side); ok {
				return part == "width" || part == "style" || part == "color"
			}
		}
		return false
	case "border-top", "border-right", "border-bottom", "border-left":
		return strings.HasPrefix(longhand, shorthand+"-") && !strings.HasSuffix(longhand, "-radius")
	case "border-width", "border-style", "border-color":
		suffix := strings.TrimPrefix(shorthand, "border")
		for _, side := range []string{"top", "right", "bottom", "left"} {
			if longhand == "border-"+side+suffix {
				return true
			}
		}
		return false
	case "border-radius":
		return strings.HasPrefix(longhand, "border-") && strings.HasSuffix(longhand, "-radius")
	}
	return false
}
