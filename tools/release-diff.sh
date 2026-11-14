#!/usr/bin/env bash
# Compares the HTML of the previous release and of the working tree on every
# test document (mjml/testdata and the attribute matrix), before a release.
# Every document listed must be explained by an intended change: one that
# brings safe-mjml closer to MJML, or a documented behaviour change.
#
# Usage: tools/release-diff.sh [previous-tag]
#   previous-tag defaults to the latest v* tag before HEAD, or HEAD's own tag
#   when tracked files have uncommitted changes.
#   SWEEP=1 also renders the layer sweep (tools/mjml-reference/layer-sweep.mjs,
#   about 6,800 generated documents with their MJML references; needs npm
#   install in tools/mjml-reference): run it before every release.
#   VERBOSE=1 lists the documents that render differently, DIFF=1 also prints
#   the first lines of each difference, KEEP=1 keeps the previous release's
#   output and prints the command that reruns the check on it.
#   BENCH=0 skips the time comparison below; BENCH_TOLERANCE (default 0.15)
#   is the slowdown it accepts.
#
# It fails (TestNoRegressionAgainstRelease) when, against the MJML reference
# of a document, the working tree lost a fragment that the previous release
# wrote, or is further from the reference than the previous release was, or
# no longer matches it while the previous release did; and when the previous
# release failed on a document or rendered nothing. The full report is
# printed on failure.
#
# It then compares the render time of both releases on the benchmarks they
# share (10, 100 and 1,000 sections, and the parse alone), run in turn six
# times each, and fails when the median of the working tree is more than
# BENCH_TOLERANCE slower than the previous release's on one of them, unless
# tools/accepted-slowdowns.txt lists that benchmark with the reason (Pierre's
# decision). A review found every render 75% slower after a change that
# no output test could see.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"
# The latest tag: with uncommitted changes to tracked files the one of HEAD,
# else the one before HEAD, since once HEAD is tagged its own tag is not the
# previous release. Untracked files (notes, symlinks) do not count.
if git diff --quiet HEAD 2>/dev/null; then
	prev=${1:-$(git describe --tags --abbrev=0 --match 'v*' HEAD^ 2>/dev/null || true)}
else
	prev=${1:-$(git describe --tags --abbrev=0 --match 'v*' HEAD 2>/dev/null || true)}
fi
if [ -z "$prev" ]; then
	# The first release of the repository has nothing to be compared with.
	echo "no previous release: nothing to compare"
	exit 0
fi

work=$(mktemp -d)
cleanup() {
	git worktree remove --force "$work/prev" >/dev/null 2>&1 || true
	if [ "${KEEP:-}" = 1 ]; then
		echo "kept: rerun with SAFE_MJML_PREVIOUS_OUTPUT=$work/prev-out go test ./mjml -count=1 -run TestNoRegressionAgainstRelease -v"
	else
		rm -rf "$work"
	fi
}
trap cleanup EXIT

git worktree add -q --detach "$work/prev" "$prev"
(cd "$work/prev" && go build -o "$work/safe-mjml-prev" ./cmd/safe-mjml)
go build -o "$work/safe-mjml-head" ./cmd/safe-mjml

# Random carousel and navbar IDs would make every such document differ.
render() { "$1" compile "$2" -s 2>/dev/null | sed -E 's/[0-9a-f]{16}/RANDOM-ID/g' || true; }

documents=(mjml/testdata/*.mjml mjml/testdata/matrix/*.mjml)
if [ "${SWEEP:-}" = 1 ]; then
	(cd tools/mjml-reference && node layer-sweep.mjs "$work/sweep" 2>/dev/null)
	documents+=("$work"/sweep/*.mjml)
	export SAFE_MJML_EXTRA_DOCUMENTS="$work/sweep"
fi

changed=0
total=0
for f in "${documents[@]}"; do
	[ -e "$f" ] || continue
	total=$((total + 1))
	# Where TestNoRegressionAgainstRelease looks for it: the repository path,
	# or "mjml" followed by the absolute path of a sweep document.
	case "$f" in
	/*) out="$work/prev-out/mjml${f%.mjml}.html" ;;
	*) out="$work/prev-out/${f%.mjml}.html" ;;
	esac
	mkdir -p "$(dirname "$out")"
	if ! "$work/safe-mjml-prev" compile "$f" -s >"$out" 2>"$out.failed"; then
		echo "the previous release failed on $f: $(head -c 200 "$out.failed")"
	else
		rm -f "$out.failed"
	fi
	render "$work/safe-mjml-prev" "$f" >"$work/a.html"
	render "$work/safe-mjml-head" "$f" >"$work/b.html"
	if ! cmp -s "$work/a.html" "$work/b.html"; then
		changed=$((changed + 1))
		[ "${VERBOSE:-}" = 1 ] && echo "changed: $f"
		if [ "${DIFF:-}" = 1 ]; then
			# One tag per line, so the diff points at the element that changed.
			diff <(sed 's/></>\n</g' "$work/a.html") <(sed 's/></>\n</g' "$work/b.html") | head -20 || true
		fi
	fi
done
echo "$changed of $total documents render differently between $prev and the working tree"

if SAFE_MJML_PREVIOUS_OUTPUT="$work/prev-out" go test ./mjml -count=1 -run TestNoRegressionAgainstRelease -v >"$work/gate.log" 2>&1; then
	grep -E 'documents compared' "$work/gate.log" || true
	echo "no regression against $prev"
else
	cat "$work/gate.log"
	echo "REGRESSIONS against $prev: see above"
	exit 1
fi

[ "${BENCH:-1}" = 0 ] && exit 0
benchmarks='^Benchmark(MJMLRender_(10|100|1000)_Sections|MJMLParsing_Only)$'
(cd "$work/prev" && go test -c -o "$work/prev.test" ./mjml)
go test -c -o "$work/head.test" ./mjml
# In turn, so that a change of machine load weighs on both alike; from the
# package directory, which the benchmarks read their templates from.
for _ in 1 2 3 4 5 6; do
	for side in prev head; do
		(cd mjml && "$work/$side.test" -test.run '^$' -test.bench "$benchmarks" -test.benchtime 1s) |
			awk -v side="$side" '/^Benchmark/ { sub(/-[0-9]+$/, "", $1); print side, $1, $3 }' >>"$work/bench.txt"
	done
done
accepted=tools/accepted-slowdowns.txt
[ -e "$accepted" ] || accepted=/dev/null
if ! awk -v tolerance="${BENCH_TOLERANCE:-0.15}" -v prev="$prev" '
	FILENAME != ARGV[ARGC - 1] {
		# accepted-slowdowns.txt: "BenchmarkName: reason"
		if ($0 !~ /^#/ && index($0, ":") > 0) accepted[substr($0, 1, index($0, ":") - 1)] = substr($0, index($0, ":") + 2)
		next
	}
	{ times[$1, $2] = times[$1, $2] " " $3; names[$2] = 1 }
	function median(list,    n, v, i, j, t) {
		n = split(list, v, " ")
		for (i = 1; i <= n; i++) for (j = i + 1; j <= n; j++) if (v[j] + 0 < v[i] + 0) { t = v[i]; v[i] = v[j]; v[j] = t }
		return n % 2 ? v[(n + 1) / 2] : (v[n / 2] + v[n / 2 + 1]) / 2
	}
	END {
		failed = 0
		for (name in names) {
			if (times["prev", name] == "" || times["head", name] == "") continue
			a = median(times["prev", name]); b = median(times["head", name])
			ratio = b / a
			line = sprintf("%-32s %s %.0f ns/op, working tree %.0f ns/op (%+.0f%%)", name, prev, a, b, (ratio - 1) * 100)
			if (ratio <= 1 + tolerance) print line
			else if (name in accepted) print line " accepted: " accepted[name]
			else { print line " SLOWER"; failed = 1 }
		}
		exit failed
	}' "$accepted" "$work/bench.txt"; then
	echo "SLOWER than $prev by more than ${BENCH_TOLERANCE:-0.15} of the time: see above (tools/accepted-slowdowns.txt lists the accepted ones)"
	exit 1
fi
echo "no slowdown against $prev"
