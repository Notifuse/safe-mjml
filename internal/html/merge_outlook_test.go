package html

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMergeOutlookConditionals(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"no conditional", "<p>x</p>", "<p>x</p>"},
		{"adjacent", "<!--[if mso | IE]><table><![endif]--><!--[if mso | IE]><tr><![endif]-->", "<!--[if mso | IE]><table><tr><![endif]-->"},
		{"whitespace between", "a<![endif]-->\n \t<!--[if mso | IE]>b", "ab"},
		{"no-break space", "a<![endif]--> <!--[if mso | IE]>b", "ab"},
		{"NEL is not JavaScript whitespace", "a<![endif]-->\u0085<!--[if mso | IE]>b", "a<![endif]-->\u0085<!--[if mso | IE]>b"},
		{"content between", "a<![endif]--><p>x</p><!--[if mso | IE]>b", "a<![endif]--><p>x</p><!--[if mso | IE]>b"},
		{"other conditional", "a<![endif]--><!--[if !mso]><!-->b", "a<![endif]--><!--[if !mso]><!-->b"},
		{"several pairs", "1<![endif]--><!--[if mso | IE]>2<![endif]--> <!--[if mso | IE]>3", "123"},
	}
	for _, tc := range cases {
		var sb strings.Builder
		sb.WriteString(tc.in)
		MergeOutlookConditionalsInBuilder(&sb)
		if got := sb.String(); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// jsMerge is mjml-core's regexp, /(<!\[endif]-->\s*?<!--\[if mso \| IE]>)/gm,
// with JavaScript's \s spelled out, used as the reference.
var jsMerge = regexp.MustCompile(`<!\[endif\]-->[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]*?<!--\[if mso \| IE\]>`)

// mergeInChunks streams s through an OutlookMergeWriter in chunks of size n.
func mergeInChunks(s string, n int) string {
	var out strings.Builder
	m := NewOutlookMergeWriter(&out)
	for len(s) > n {
		_, _ = m.WriteString(s[:n])
		s = s[n:]
	}
	_, _ = m.WriteString(s)
	_ = m.Flush()
	return out.String()
}

func TestOutlookMergeWriterMatchesMJMLAcrossWrites(t *testing.T) {
	tricky := []string{
		"a<![endif]--><!--[if mso | IE]>b",
		"a<![endif]-->  \n<!--[if mso | IE]>b<![endif]--> <!--[if mso | IE]>c",
		"a<![endif]--> 　<!--[if mso | IE]>b",
		"a<![endif]-->\u0085<!--[if mso | IE]>b",
		"a<![endif]--><!--[if !mso]><!-->b<![endif]-->",
		"<![endif]--><![endif]--><!--[if mso | IE]><!--[if mso | IE]>",
		"a<![endif]--> <!--[if mso | I",
		"a<![endif",
		"<![endif]-->",
	}
	for _, s := range tricky {
		want := jsMerge.ReplaceAllString(s, "")
		for split := 0; split <= len(s); split++ {
			var out strings.Builder
			m := NewOutlookMergeWriter(&out)
			_, _ = m.WriteString(s[:split])
			_, _ = m.WriteString(s[split:])
			_ = m.Flush()
			if got := out.String(); got != want {
				t.Errorf("%q split at %d: got %q, want %q", s, split, got, want)
			}
		}
	}

	snapshots, err := filepath.Glob(filepath.Join("..", "..", "mjml", "testdata", "snapshots", "*.html"))
	if err != nil || len(snapshots) == 0 {
		t.Fatalf("no snapshots found: %v", err)
	}
	for _, file := range snapshots {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		want := jsMerge.ReplaceAllString(string(content), "")
		for _, n := range []int{1, 2, 3, 7, len(content) + 1} {
			if got := mergeInChunks(string(content), n); got != want {
				t.Errorf("%s in chunks of %d differs from the MJML merge", filepath.Base(file), n)
			}
		}
	}
}
