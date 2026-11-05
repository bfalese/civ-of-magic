# Refactor helpers

Small scripts used to move code into `core/` (see `docs/handoff/003-*.md` for the recipe).
Run from the repo root. They edit files in place, so start from a clean tree and review the diff.
They don't fix imports; let the compiler tell you, then run `prune-imports.sh`.

| Script | What it does |
|---|---|
| `movefuncs.py SRC DST ALIAS NAME...` | Moves top-level funcs/types/consts/vars (methods as `Type.Method`) from `SRC` to `DST`. With an `ALIAS`, identifiers exported by `SRC`'s package get qualified (`Spells` → `spellbook.Spells`); use `-` for a same-package move. |
| `requalify.py OLD_DIR OLD_IMPORT ALIAS FILE...` | After `git mv`-ing whole files to a new package: fixes the package clause, qualifies the old package's exported identifiers and adds its import. |
| `fixcallers.py OLD_ALIAS_REGEX NEW_ALIAS NEW_IMPORT FUNC[,FUNC...] FILE...` | Rewrites `old.Func` to `new.Func` in callers and adds the import (`NEW_IMPORT` may be `alias path`). Unqualified calls inside the old package are not touched. |
| `prune-imports.sh` | Deletes imports `go build` reports as unused, repeating until clean. |
| `check-move.sh REV NEW_FILE QUALIFIERS OLD_FILE...` | Confirms moved code is identical apart from qualifiers. |
| `tidy-imports.py FILE...` | Formatting only: moves this module's imports that landed among third-party ones back into the module group, collapses blank lines. |

Known limits of the qualifier rewrite (regex, not a Go parser):
- Leaves strings, `//` comments and `/* */` blocks alone (so commented-out code keeps old names).
- Skips struct-literal keys (`Name: x`) and struct field names (`Spells Spells`). Map-literal
  keys look the same (`DirectionNorth: 1`) and are skipped too, as is the last item of a `case`
  list that continues on a new line; the compiler reports them as undefined.
- When exporting a field, check multi-line parameter lists: a line starting with the old name
  can be a parameter, not a field.
- If the code uses the package name as a variable (`artifact := ...`), import the package with
  an alias such as `artifactlib`.
