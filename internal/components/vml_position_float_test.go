package components

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestVMLPercentMatchesJavaScript checks the no-repeat VML value of every
// integer percentage from 0% to 1000% against the value MJML computes in
// JavaScript, (-50 + (p / 100) * 100) / 100. The expected values come from
// Node, not from Go arithmetic: Go may fuse a multiply and an add (FMA on
// arm64) even across statements, which would bias the reference. Regenerate
// testdata/vml-percent-js.txt with:
//
//	node -e 'const o=[];for(let p=0;p<=1000;p++){o.push(p+" "+String((-50+(p/100)*100)/100))}require("fs").writeFileSync("testdata/vml-percent-js.txt",o.join("\n")+"\n")'
func TestVMLPercentMatchesJavaScript(t *testing.T) {
	file, err := os.Open("testdata/vml-percent-js.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	checked := 0
	for scanner.Scan() {
		percent, want, ok := strings.Cut(scanner.Text(), " ")
		if !ok {
			t.Fatalf("malformed line %q", scanner.Text())
		}
		if got := vmlPercentToDecimal(percent+"%", false); got != want {
			t.Errorf("%s%%: got %s, want %s (JavaScript)", percent, got, want)
		}
		checked++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if checked != 1001 {
		t.Fatalf("checked %d values, want 1001", checked)
	}
}
