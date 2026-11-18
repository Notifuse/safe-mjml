#!/usr/bin/env bash
# Prints the binary size each library adds (docs/library-comparison.md): a
# program that reads a file and renders it with the library, built with
# -trimpath -ldflags='-s -w', minus the same program that only reads the
# file. "inliner" is the part of safe-mjml's that is its CSS inliner. Run
# from tools/libcmp, on the platform to measure.
set -euo pipefail

work=$(mktemp -d "$PWD/.sizes-XXXXXX")
trap 'rm -rf "$work"' EXIT

program() { # name, import, call
	mkdir -p "$work/$1"
	cat > "$work/$1/main.go" <<EOF
package main

import (
	"bytes"
	"context"
	"os"
$2
)

var _ = bytes.MinRead
var _ = context.Background

func main() {
	src, _ := os.ReadFile(os.Args[1])
	s := string(src)
	_ = s
	$3
}
EOF
}

program base '' '_ = s'
program notifuse 'mjml "github.com/Notifuse/safe-mjml/mjml"' '_, _ = mjml.Render(s)'
program inliner 'juicer "github.com/Notifuse/safe-mjml/internal/juicer"' '_, _ = juicer.Inline(s)'
program preslav 'mjml "github.com/preslavrachev/gomjml/mjml"' '_, _ = mjml.Render(s)'
program mgml 'core "github.com/krozhkov/mgml/core"
	mjml "github.com/krozhkov/mgml/mjml"' '_, _ = mjml.MJML2Html(s, &core.MJMLOptions{ValidationLevel: core.ValidationLevelSoft})'
program julez 'julez "github.com/julez-dev/mjmlgo/v2"' 'var b bytes.Buffer; _ = julez.NewRenderer().Render(context.Background(), &b, src)'
program tcmits 'mjml2html "github.com/TcMits/mjml2html"' '_, _ = mjml2html.ToHTML(s, mjml2html.ValidationLevel("soft"))'
program mrml 'mrmlgo "github.com/codedninja/mrmlgo"' 'o, _ := mrmlgo.NewParseOptions(nil); r, _ := o.ParseMJML(s); _, _ = r.ToHTML()'

size() {
	go build -trimpath -ldflags='-s -w' -o "$work/$1.bin" "./$(basename "$work")/$1"
	wc -c < "$work/$1.bin"
}

base=$(size base)
echo "Binary size added ($(go env GOOS)/$(go env GOARCH), $(go version | cut -d' ' -f3)):"
for lib in notifuse inliner julez mgml preslav tcmits mrml; do
	added=$(( $(size "$lib") - base ))
	printf '%-10s %6.1f MB\n' "$lib" "$(echo "$added / 1048576" | bc -l)"
done
