# Handoff 009: start here (state at end of 2026-11-05)

Date: 2026-11-05
Previous: [008](008-game-go-turn-loop-in-core.md) (read it next: it has the details of the last round)

This is the entry point for a fresh session. Read in this order: `CLAUDE.md`, this file, 008, then the latest handoff (017 at the time of writing). Smoke tests to do by hand: `docs/smoke-test.md`.

## Where things stand

- **Goal:** pure-Go game logic in `core/` (no Ebitengine, no LBX), screens in the Ebitengine frontend
  (`game/magic/*view`, `game/magic/game`). Plan: [001](001-fork-and-decoupling-plan.md).
- **Done:** every model package is in `core/`: `ai`, `artifact`, `building`, `city`, `combat`, `data`,
  `game`, `hero`, `maplib`, `neural`, `pathfinding`, `player`, `setup`, `spellbook`, `terrain`, `units`.
  Spell-casting logic (from `cast.go`) and the turn loop (from `game.go`) are in `core/game`, with headless tests.
- **Only remaining tangle:** the UI package `game/magic/game`, mainly `game.go` (6.8k lines, mostly
  screens now) and `cast.go` (2.3k, mostly targeting/animation UI).

## Open PR

| PR | Branch | Base | Status |
|---|---|---|---|
| #13 | `game-go-split-1` | `master` | waiting for the user's smoke test + merge approval. **Don't merge without explicit approval.** |

After it merges: `git checkout master && git pull`, then branch for the next round.

Merging stacked PRs without force-pushing: see [003](003-spellbook-artifact-units-hero.md).
Summary: squash-merge, check `git diff <prev-branch> origin/master` is empty,
`git merge -s ours origin/master` on the next branch, push, retarget with
`gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master` (`gh pr edit` fails).

## Running the game

MoM data is now on this machine (a DOS install; lowercase file names are fine):

```sh
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic"
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -start -music=false  # random wizard
```

The data is copyrighted: never copy it into the repo (`data/data/data.zip` is a tracked placeholder).

Headless check (Claude can do this): build, run under `Xvfb :NN` with `DISPLAY=:NN`, screenshot with
`import -window root shot.png`.

## Smoke-test status for #13

- OK (user, by hand): start a game, move units, end a turn, open a city, pick research.
- OK (headless): `-start` loads data and generates the maps without errors.
- **Not tested yet:** combat (attack a lair or enemy stack), casting an overworld spell and a city
  enchantment, a researched spell finishing, save + load, several turns (income, buildings
  finishing, hero/merchant offers), end-of-turn global spells. Highest risk: combat and casting.

## Known issue

`-watch` (AI vs AI) freezes on the first wizard-contact diplomacy dialog, because the "human" player in watch mode is
only flagged Admin/Banished and the dialog waits for a click. Upstream before the refactor
(`0239a0c1`) does the same, so it's not a regression.

## Next steps (pick with the user)

1. **Fix the `-watch` freeze** (small): skip the diplomacy dialog (and other blocking human dialogs)
   when in watch mode. That makes `-watch` a hands-off regression test to run after each refactor step.
2. **Continue splitting `game.go`** (008 "Next"): `doCombat` (applying results to the overworld),
   `doMoveSelectedUnit`, `doAiUpdate` (model-side AI turn so AI runs headless), `doEncounter`,
   `DoBuildAction`, `defeatCity`, `ApplyTreasure`/`createTreasure`. Then the smaller UI files and
   `cityview/city-screen.go`.
3. Later (001): command/event API on `core/game`, then the wasm bridge for the Three.js frontend.

Optional: the game has a loader for original DOS saves (`save1.gam` etc. in the data folder), which
could serve as a quick "game in progress" test.

## How the refactor rounds are done

- Tools in `scripts/refactor/` (README there). Main ones: `movefuncs.py` (move declarations
  between packages), `promote-methods.py` (move `Game` methods onto `GameModel`), `prune-imports.sh`,
  `check-move.sh` / normalized diffs to prove a move changed nothing else.
- Per step: `go build ./... && go vet ./... && go test ./core/... ./lib/... ./game/...`, plus the
  CI guard `go list -deps ./core/... | grep -E 'ebiten|/lbx'` must print nothing.
- Conventions: alias `core/game` as `gamecore` (`gamelib` = UI package); events the model emits live
  in `core/game/gameevents.go`; UI-only events stay in the UI; frontend behaviour is passed into model
  functions as function parameters; exported `Game` methods keep one-line delegating wrappers.
- CI noise to ignore: macOS M1 "job was not acquired by Runner", and an occasional arm64 xvfb flake.
- End each session with a new `docs/handoff/NNN-*.md`.
