#!/bin/sh
# Delete imports the compiler reports as "imported and not used", repeating until none are left.
# Deletes bottom-up within each file so line numbers stay valid.
# usage: scripts/refactor/prune-imports.sh   (from the repo root)

for pass in $(seq 1 20); do
    errors=$(go build ./... 2>&1 \
        | grep -E '^[^ ]+\.go:[0-9]+:[0-9]+: ".*" imported (as [a-zA-Z_][a-zA-Z0-9_]* )?and not used$' \
        | sort -u -t: -k1,1 -k2,2nr)
    [ -z "$errors" ] && exit 0
    echo "$errors"
    echo "$errors" | while IFS=: read file line column rest; do
        # only delete the line if it really is an import spec
        sed -i "${line}{/^[[:space:]]*\([a-zA-Z_][a-zA-Z0-9_]* \)\?\"/d}" "$file"
    done
done
echo "still failing after 20 passes" >&2
exit 1
