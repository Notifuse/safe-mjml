package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// hostile runs one check of a template written to cause harm against a
// library. Each check runs in its own process, because some libraries crash
// the process in a way Go cannot recover from.
//
//	go run . hostile <lib> raw-quote|escaped-quote|concurrent|inline-styles
func hostile(lib, check string) {
	render := renderers[lib]
	if render == nil {
		fmt.Fprintln(os.Stderr, "unknown library", lib)
		os.Exit(2)
	}
	switch check {
	case "raw-quote", "escaped-quote":
		// A quote inside an attribute value, written raw (inside single
		// quotes) or escaped as &quot;, as a template engine would.
		alt, href := `'x" onerror="alert(1)'`, `'https://example.com/" onclick="alert(2)'`
		if check == "escaped-quote" {
			alt, href = `"x&quot; onerror=&quot;alert(1)"`, `"https://example.com/&quot; onclick=&quot;alert(2)"`
		}
		out, err := render(`<mjml><mj-body><mj-section><mj-column>` +
			`<mj-image src="https://example.com/a.png" alt=` + alt + ` />` +
			`<mj-button href=` + href + `>Go</mj-button>` +
			`</mj-column></mj-section></mj-body></mjml>`)
		injected := strings.Contains(out, `" onerror="alert(1)"`) || strings.Contains(out, `" onclick="alert(2)"`)
		fmt.Printf("%s %s: injected=%v err=%v\n", lib, check, injected, err)
	case "concurrent":
		// 16 goroutines rendering the same small template 30 times each.
		const doc = `<mjml><mj-body><mj-section><mj-column><mj-text>Hi</mj-text><mj-button href="https://example.com">Go</mj-button></mj-column></mj-section></mj-body></mjml>`
		var wg sync.WaitGroup
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 30 {
					_, _ = render(doc)
				}
			}()
		}
		wg.Wait()
		fmt.Printf("%s concurrent: ok\n", lib)
	case "inline-styles":
		// n rules for one class in an inline mj-style, and n elements with
		// that class: the family inline-rules-and-elements of
		// mjml/scaling_test.go, at 1, 4, 16 and 64 times its size.
		fmt.Printf("%s inline-styles:", lib)
		for _, k := range []int{1, 4, 16, 64} {
			n := 300 * k
			doc := `<mjml><mj-head><mj-style inline="inline">` + strings.Repeat(`.a { color: red; }`, n) +
				`</mj-style></mj-head><mj-body><mj-section><mj-column>` +
				strings.Repeat(`<mj-text css-class="a">t</mj-text>`, n) + `</mj-column></mj-section></mj-body></mjml>`
			start := time.Now()
			_, _ = render(doc)
			fmt.Printf(" %d KB %v,", len(doc)/1024, time.Since(start).Round(time.Millisecond))
		}
		fmt.Println()
	default:
		fmt.Fprintln(os.Stderr, "unknown check", check)
		os.Exit(2)
	}
}
