# Handoff 037: tax rate, renaming cities, enchantments you can cancel

Date: 2026-11-09
Previous: [036](036-heroes-items.md). Start-here doc: [034](034-start-here.md) (this is its step 3).

## State

- #41 is merged. **#42 (`heroes-items`, 036) passed CI and targets `master`, not merged** (only
  #41 was approved). This round's branch `city-extras` is stacked on it and its PR targets
  `heroes-items`: merge #42, then retarget (`gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N
  -f base=master`) and merge `master` into the branch (no force push needed; the conflicts are
  only the stacked commits, take the branch's side after checking `git diff origin/master
  origin/heroes-items` is empty).

## What changed

1. **`cancel-enchantment`** (`core/game/enchantments.go`): cancel one of your global enchantments,
   or one you cast on a city (yours, or a curse on someone else's); unrest is updated. The
   original game allows this from the city and magic screens; the core had no command for it.
2. **Views**: city `enchantments` and `you.global-enchantments` are now objects `{name, upkeep,
   owner, yours}` (they were strings; nothing read them); `you.tax-rates` lists the rates with
   their label and unrest. `city.TaxUnrest(rate)` is the old inline table, shared.
3. **Page**: a tax dropdown in the top bar ("2 (45%)": gold per citizen and the unrest); in the
   city screen ✎ to rename (Enter saves, Esc cancels) and an Enchantments list with cancel for
   yours (asks first), curses on the city in red with their caster; in the magic screen the
   global enchantments with their upkeep and cancel.
4. The top bar fits one line at 1200 px again: the research text is cut with an ellipsis (full
   name in the tooltip). The smoke test checks it.
5. `TestHostCityExtras`; smoke steps in the spellbook run (tax, rename, cancel the ward).

## Next

034's list from step 4: diplomacy for known wizards (commands exist), then the start screen and
soft fog edges, battle spell animations and a target-aware `castable`.
