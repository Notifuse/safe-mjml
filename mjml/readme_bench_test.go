package mjml

import (
	"os"
	"testing"
)

// BenchmarkReadmeTemplates renders the templates of the README's performance
// table, warm and in process (tools/bench-vs-mjml.sh): the Austin template of
// mjml.io, which has inline mj-styles, and a Notifuse email, which has none.
func BenchmarkReadmeTemplates(b *testing.B) {
	for _, name := range []string{"austin-layout-from-mjml-io", "notifuse-full"} {
		source, err := os.ReadFile("testdata/" + name + ".mjml")
		if err != nil {
			b.Fatal(err)
		}
		template := string(source)
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Render(template); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
