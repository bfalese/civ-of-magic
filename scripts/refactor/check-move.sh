#!/bin/sh
# Check that code moved between packages is unchanged apart from package qualifiers.
# Compares the lines removed from the old files (vs a git revision) with the lines in the new file,
# after stripping the given qualifiers, import specs, blank lines and package clauses.
#
# usage: scripts/refactor/check-move.sh REV NEW_FILE QUALIFIER[,QUALIFIER...] OLD_FILE...
# example: scripts/refactor/check-move.sh HEAD game/magic/lbxdata/hero.go herolib game/magic/hero/names.go

rev=$1; new=$2; qualifiers=$(echo "$3" | sed 's/,/|/g'); shift 3
filter() {
    sed -E "s/\b($qualifiers)\.//g" \
        | grep -vE '^[[:space:]]*(_ |[a-zA-Z_][a-zA-Z0-9_]* )?"[^"]*"$|^import \($|^\)$|^package |^[[:space:]]*$' \
        | sort
}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for old in "$@"; do
    if git cat-file -e "$rev:$old" 2>/dev/null && [ ! -e "$old" ]; then
        git show "$rev:$old"
    else
        git diff -U0 "$rev" -- "$old" | grep '^-[^-]' | sed 's/^-//'
    fi
done | filter > "$tmp/removed"
filter < "$new" > "$tmp/added"
if diff "$tmp/removed" "$tmp/added"; then
    echo "identical apart from qualifiers"
else
    echo "(lines starting with < were removed but not found in $new; > are new)" >&2
    exit 1
fi
