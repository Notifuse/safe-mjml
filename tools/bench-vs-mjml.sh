#!/usr/bin/env bash
# Measures safe-mjml against MJML 5.4.1 on the templates of the README's
# performance table, and prints the table in Markdown:
#
#   - one render in process, warm: Go's benchmark (BenchmarkReadmeTemplates)
#     and the mean of 500 renders in Node.js after 50 to warm up;
#   - 50 compilations through each CLI, one process each, output discarded;
#   - the peak memory (maximum resident set) of one CLI compilation.
#
# Needs Go, Node.js and `npm ci` in tools/mjml-reference. The CI runs it on
# GitHub's Linux arm64 runner (.github/workflows/bench.yml), whose numbers
# the README gives: run it there, not on a busy laptop.
set -euo pipefail
export LC_ALL=C

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
templates=(austin-layout-from-mjml-io notifuse-full)
work=$(mktemp -d)
trap 'mv "$work" "$work.done"' EXIT

go build -o "$work/safe-mjml" ./cmd/safe-mjml
mjml_cli="$root/tools/mjml-reference/node_modules/.bin/mjml"

# Go: ns/op of each sub-benchmark.
(cd mjml && go test -run '^$' -bench 'BenchmarkReadmeTemplates' -benchtime 3s -count 3) |
	awk '/^BenchmarkReadmeTemplates\// { split($1, a, "/"); sub(/-[0-9]+$/, "", a[2]); print a[2], $3 }' >"$work/go.txt"
# Node: ms per render.
files=()
for t in "${templates[@]}"; do files+=("$root/mjml/testdata/$t.mjml"); done
(cd tools/mjml-reference && node bench-render.mjs 500 "${files[@]}") >"$work/node.txt"

# Seconds for 50 runs of a command.
fifty() {
	local start end
	start=$(date +%s%N)
	for _ in $(seq 50); do "$@" >/dev/null 2>&1; done
	end=$(date +%s%N)
	awk -v ns=$((end - start)) 'BEGIN { printf "%.2f", ns / 1e9 }'
}
# Peak memory of one run, in MB.
peak() {
	python3 -c 'import resource, subprocess, sys
subprocess.run(sys.argv[1:], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
peak = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss  # KB on Linux, bytes on macOS
print("%.0f" % (peak / (1 << 20) if sys.platform == "darwin" else peak / 1024))' "$@"
}

echo "| Template | | safe-mjml | MJML 5.4.1 (Node.js $(node --version | sed 's/^v//')) |"
echo "|---|---|---|---|"
for t in "${templates[@]}"; do
	file="mjml/testdata/$t.mjml"
	go_ms=$(awk -v t="$t" '$1 == t { s += $2; n++ } END { printf "%.2f", s / n / 1e6 }' "$work/go.txt")
	node_ms=$(awk -v t="$t" '$1 == t { printf "%.1f", $2 }' "$work/node.txt")
	echo "| \`$t\` | One render, in process, warm | $go_ms ms | $node_ms ms |"
	echo "| | 50 compilations through the CLI | $(fifty "$work/safe-mjml" compile "$file" -o "$work/out.html") s | $(fifty "$mjml_cli" "$file" -o "$work/out-js.html") s |"
	echo "| | Peak memory of the CLI | $(peak "$work/safe-mjml" compile "$file" -o "$work/out.html") MB | $(peak "$mjml_cli" "$file" -o "$work/out-js.html") MB |"
done
echo
echo "$(uname -sm), $(nproc 2>/dev/null || sysctl -n hw.ncpu) CPUs, $(go version | awk '{ print $3 }'), $(git rev-parse --short HEAD)."
