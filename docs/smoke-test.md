# Smoke test checklist (playthrough)

Everything the refactor rounds (PRs #7–#15) moved that hasn't been checked by hand yet,
ordered so one playthrough covers it. Tick items as you go; note anything odd under **Problems**.

```sh
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic"
```

Tips: save often (a save before a risky step makes a bug easy to reproduce). For a hero, a node or
a tower nearby, a game on a small land size helps. Already checked: start a game, move units, end
a turn, open a city, pick research, and AI-vs-AI (`-watch`, headless).

## 1. Start and the overworld (first turns)

- [ ] New game: capital naming prompt (a predefined name), camera on the capital, starting units;
      the raiders' towns appear if enabled
- [ ] Overworld map and minimap draw correctly, scrolling works, both planes
- [ ] Cartographer (map of the world) and Surveyor open and show the right info
- [ ] Surveyor (now drawn from core `MakeTileInfo`, PR for handoff 043 group 4): terrain, food /
      production / gold, a bonus, a node and its melder, a lair's guards, a city; "City Resources"
      with max pop / prod / gold, or why no city can go there (water, 3 squares)
- [ ] Plane button with a stack on a tower: the stack goes to the other plane (PlanarTravel now
      takes the stack; group 1)
- [ ] Random events: a few turns in, with random events on in settings (an event popup appears)
- [ ] Meeting a wizard: diplomacy greeting screen when an enemy wizard is first seen
- [ ] Several turns in a row: gold/mana/food income changes as expected, research progresses
- [ ] A researched spell finishes: learn-spell animation, then the research choice opens

## 2. Cities

- [ ] A building finishes: new building popup, then the city screen
- [ ] City grows: growth message (scroll / chancellor)
- [ ] City screen: sell a building (gold received, rubble shown, only once per turn)
- [ ] City screen: buy production (price shown, gold taken, production full)
- [ ] City screen: click farmers and workers to move citizens between them; numbers add up
- [ ] City screen: the resource dialogs (click the food, production, gold, power, research rows)
      list sensible sources
- [ ] Settle a new outpost with settlers (build button `B`): name prompt, outpost appears

## 3. Units and the map

- [ ] **Move your stacks** by clicking the map and with the keys (this order now goes through the
      model): the stack walks; with some units deselected in the stack panel, only the selected ones
      go and the rest stay; a blocked destination blinks red; Earth Gate between two of your cities
      moves instantly

- [ ] Walk a path that takes several turns (the stack continues on the next turns)
- [ ] Build a road with engineers (build button, pick the destination, road grows over turns)
- [ ] Meld a node with a magic spirit (build button on the node)
- [ ] Planar travel through a tower (plane button with the stack on the tower)
- [ ] Planar travel with Plane Shift (a unit with the ability, or the spell)
- [ ] Toggle units in the stack panel (click a unit to deselect / select it; units in stasis can't be
      toggled)
- [ ] Patrol button: the stack patrols and the next unit is selected
- [ ] Disband a unit from its context menu (right click on the unit in the stack panel)
- [ ] Change the tax rate (Info / tax collector): rate and unrest update

## 4. Combat

- [ ] Attack a lair: confirm dialog, battle, treasure popup, vault if an item drops
- [ ] Chaos node battle (node aura effects), Fireball / Lightning projectiles draw correctly
- [ ] Attack an enemy stack: battle, end screen with fame
- [ ] Attack an enemy city: battle, raze/capture choice, end screen, city changes owner
- [ ] Retreat from a battle (the stack steps back)
- [ ] Lose a hero in battle: "redistribute their equipment" notice, then the vault
- [ ] Strategic combat only (settings): battles resolve without the battle screen

## 5. Heroes, offers and the vault

- [ ] Hero for hire offer: hire (name prompt, hero appears at the fortress) and decline
- [ ] Mercenaries offer: hire (units appear, gold taken)
- [ ] Merchant offer: buy (gold taken, item goes to the vault)
- [ ] Alchemy in the magic screen: gold to mana and mana to gold (amounts as shown)
- [ ] Diplomacy: propose a pact, offer a gold tribute and a spell tribute, exchange spells
- [ ] Vault: destroy an item for mana (amount shown, mana received)
- [ ] Vault: give an item to a hero somewhere else (20 mana transfer prompt)
- [ ] Hero gains a level: level-up popup

## 6. Spells (overworld)

- [ ] Summon a creature (summon animation, unit appears)
- [ ] City enchantment on own city (e.g. Wall of Fire / Heavenly Light); Spell Ward
- [ ] Unit enchantment on a unit (e.g. Bless); casting it again on the same unit shows "already has"
- [ ] Lycanthropy on a summoned unit or a hero is refused with a notice
- [ ] Map spell (e.g. Change Terrain, Transmute, Corruption), on Arcanus and on Myrror
- [ ] Curse on an enemy city (e.g. Famine) or unit
- [ ] Disenchant / Disjunction; Spell Binding
- [ ] Word of Recall (unit returns to the summoning circle; notice if already there)
- [ ] Resurrection (needs a dead hero)
- [ ] Global enchantment: casting animation, effect active
- [ ] Awareness (cities revealed, owners met) and Nature Awareness (map revealed)
- [ ] End-of-turn global spells: Armageddon, Meteor Storm, Great Wasting, Chaos Rift (if reachable;
      an AI casting them in `-watch` also exercises the code)

## 7. Saving and loading

- [ ] Save, quit, load: the game continues where it was (units, cities, research, gold)
- [ ] Load an original DOS save (`save1.gam` etc. in the data folder)

## Problems

<!-- turn, what you did, what happened, what you expected; a save file if you have one -->
