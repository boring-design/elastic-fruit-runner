#!/bin/sh
# Legacy-pattern ratchet: fail if any ast-grep rule matches more often on HEAD
# than on the merge-base with <base-ref>. Both sides are counted with HEAD's
# .ast-grep/ rules, so no baseline file is committed.
#
# Usage: .ast-grep/ratchet.sh <base-ref>     (e.g. origin/main)
# Needs: git, ast-grep (0.51.1), jq.
set -eu

if [ $# -ne 1 ]; then
  echo "usage: $0 <base-ref>" >&2
  exit 2
fi
base_ref=$1

root=$(git rev-parse --show-toplevel)
cd "$root"

# count <dir> <out>: writes lines "<ruleId> <count>" (ast-grep failure aborts)
count() {
  (cd "$1" && ast-grep scan -c .ast-grep/sgconfig.yml --json=stream .) >"$2.json"
  jq -r .ruleId <"$2.json" | sort | uniq -c | awk '{print $2, $1}' >"$2"
}

merge_base=$(git merge-base "$base_ref" HEAD)
tmp=$(mktemp -d)
wt="$tmp/base"
cleanup() {
  git worktree remove --force "$wt" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

git worktree add --quiet --detach "$wt" "$merge_base"
rm -rf "$wt/.ast-grep"
cp -R "$root/.ast-grep" "$wt/.ast-grep"

count "$root" "$tmp/head"
count "$wt" "$tmp/base.counts"
# every rule id, so rules with no matches on either side still show up as 0
sed -n 's/^id:[[:space:]]*//p' .ast-grep/rules/*.yml | awk '{print $1, 0}' >"$tmp/ids"

echo "ast-grep ratchet: base $base_ref (merge-base $(git rev-parse --short "$merge_base")) vs HEAD"
awk -v ids="$tmp/ids" -v basef="$tmp/base.counts" '
  FILENAME == ids { seen[$1] = 1; next }
  FILENAME == basef { base[$1] = $2; seen[$1] = 1; next }
  { head[$1] = $2; seen[$1] = 1 }
  END {
    printf "%-40s %6s %6s %7s\n", "rule", "base", "head", "change"
    n = 0
    for (r in seen) names[++n] = r
    # simple sort for stable output
    for (i = 1; i <= n; i++) for (j = i + 1; j <= n; j++) if (names[j] < names[i]) { t = names[i]; names[i] = names[j]; names[j] = t }
    rose = ""
    for (i = 1; i <= n; i++) {
      r = names[i]; b = base[r] + 0; h = head[r] + 0
      printf "%-40s %6d %6d %+7d\n", r, b, h, h - b
      if (h > b) rose = rose " " r
    }
    if (rose != "") {
      print ""
      print "FAIL: legacy-pattern count rose for:" rose
      print "Use the preferred pattern named in the rule message (run: ast-grep scan -c .ast-grep/sgconfig.yml)."
      exit 1
    }
    print ""
    print "OK: no legacy-pattern count rose."
  }
' "$tmp/ids" "$tmp/base.counts" "$tmp/head"
