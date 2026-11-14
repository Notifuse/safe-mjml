package styles

import (
	"testing"
)

func TestNormalizeColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "short hex", input: "#123", want: "#112233"},
		{name: "already long", input: "#112233", want: "#112233"},
		{name: "rgba", input: "rgba(0,0,0,0.5)", want: "rgba(0,0,0,0.5)"},
		{name: "empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NormalizeColor(tt.input); got != tt.want {
				t.Fatalf("NormalizeColor(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
