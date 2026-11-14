package main

import (
	"fmt"
	"log"

	"github.com/Notifuse/safe-mjml/mjml"
)

func main() {
	mjmlContent := `<mjml>
        <mj-head>
          <mj-title>My Newsletter</mj-title>
        </mj-head>
        <mj-body>
          <mj-section>
            <mj-column>
              <mj-text>Hello World!</mj-text>
              <mj-button href="https://example.com">Click Me</mj-button>
            </mj-column>
          </mj-section>
        </mj-body>
      </mjml>`

	// Method 1: Direct rendering (recommended)
	html, err := mjml.Render(mjmlContent)
	if err != nil {
		log.Fatal("Render error:", err)
	}
	fmt.Println(html)

	// Method 2: with the validation warnings
	result, err := mjml.Compile(mjmlContent)
	if err != nil {
		log.Fatal("Compile error:", err)
	}
	for _, w := range result.Warnings {
		log.Printf("line %d: (%s) %s", w.Line, w.TagName, w.Message)
	}
	fmt.Println(result.HTML)

	// Method 3: parse once, render several times
	doc, err := mjml.Parse(mjmlContent)
	if err != nil {
		log.Fatal("Parse error:", err)
	}
	html, err = doc.Render()
	if err != nil {
		log.Fatal("Render error:", err)
	}
	fmt.Println(html)
}
