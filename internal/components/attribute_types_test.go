package components

import "testing"

// TestAttributeTypes checks the value checks against mjml-core's types
// (types/*.js), including their quirks: rgb() is found anywhere in a color,
// color names are case-sensitive, an integer only needs a digit, an empty
// unit allows a bare number.
func TestAttributeTypes(t *testing.T) {
	for _, tc := range []struct {
		spec, value string
		valid       bool
	}{
		{"color", "#ABCDEF", true},
		{"color", "#abc", true},
		{"color", "#abcd", false},
		{"color", "red", true},
		{"color", "Red", false},
		{"color", "transparent", true},
		{"color", "inherit", true},
		{"color", "none", false},
		{"color", "", false},
		{"color", "rgb(1, 2,3)", true},
		{"color", "x rgba(1,2,3,0.5) y", true},
		{"color", "rgba(1,2,3,0.12345)", false},
		{"enum(left,right)", "left", true},
		{"enum(left,right)", "Left", false},
		{"enum(left,right)", "", false},
		{"enum(full-width,false,)", "", true},
		{"enum(full-width,false,)", "false", true},
		{"unit(px)", "10px", true},
		{"unit(px)", "10", false},
		{"unit(px)", "0", true},
		{"unit(px)", "1.5px", true},
		{"unit(px)", "-1px", false},
		{"unit(px)", "10px 10px", false},
		{"unit(px,%){1,4}", "10px 5% 0 2px", true},
		{"unit(px,%){1,4}", "1px 1px 1px 1px 1px", false},
		{"unit(px,%){1,4}", "", false},
		{"unit(px,%,)", "1.5", true},
		{"unit(px,%,auto)", "auto", true},
		{"unit(px,%)", "auto", false},
		{"unitWithNegative(px,em)", "-2em", true},
		{"integer", "4", true},
		{"integer", "a4b", true},
		{"integer", "", false},
		{"boolean", "true", true},
		{"boolean", "True", false},
		{"string", "", true},
		{"string", "anything", true},
	} {
		message := newAttributeType(tc.spec).check(tc.value)
		if (message == "") != tc.valid {
			t.Errorf("%s %q: got message %q, want valid=%v", tc.spec, tc.value, message, tc.valid)
		}
	}
}

func TestAttributeTypeMessages(t *testing.T) {
	for _, tc := range []struct{ spec, value, want string }{
		{"enum(left,right,center,justify)", "bogus", "has invalid value: bogus for type Enum, only accepts left, right, center, justify"},
		{"enum(full-width,false,)", "x", "has invalid value: x for type Enum, only accepts full-width, false,"},
		{"unit(px,%){1,4}", "1em", "has invalid value: 1em for type Unit, only accepts (px, %) units and 1 to 4 value(s)"},
		{"unit(px,%,)", "x", "has invalid value: x for type Unit, only accepts (px, %, ) units and 1 value(s)"},
		{"color", "nonsense", "has invalid value: nonsense for type Color"},
		{"integer", "", "has invalid value:  for type NInteger"},
		{"boolean", "yes", "has invalid value: yes for type Boolean"},
	} {
		if got := newAttributeType(tc.spec).check(tc.value); got != tc.want {
			t.Errorf("%s %q: got %q, want %q", tc.spec, tc.value, got, tc.want)
		}
	}
}
