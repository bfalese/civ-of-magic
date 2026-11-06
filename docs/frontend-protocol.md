# Frontend protocol (JSON)

How a frontend that doesn't share memory with the Go model talks to it: the future wasm bridge
for the Three.js renderer, a network client, or a test harness. The Ebitengine frontend calls the
model directly and doesn't need this.

```
frontend ──command──▶ GameModel.Execute ──▶ model changes
frontend ◀──events─── model.Events (EncodeEvent)
frontend ◀──views──── MakePlayerView / MakeMapView (ask again after a "refresh" event)
```

Code: `core/game/command.go`, `core/game/eventjson.go`, `core/game/view.go`.

## Sessions: commands that wait for the player

A frontend plays through a `CommandSession` (`core/game/session.go`): `session.Execute(command)`.
Some commands can't finish at once: a stack walking into a lair asks whether to enter, a battle
needs orders, a captured city asks raze-or-keep. The command then pauses: `Execute` returns,
`session.WaitingFor()` is `confirm-encounter`, `battle` or `raze-city` (an event announces it),
and the next commands answer it. When the answer is complete, the paused command carries on to
its end. While a command is paused, only answers are accepted.

Frontends also pass every event they take from `model.Events` through `session.ProcessEvent`
(it applies treasure for the player: items go to the vault as new items, prisoners become an
offer).

## Conventions

- **Players** are identified by banner: `{"banner": "blue", "wizard": "Merlin", "human": true}`.
  Banners are unique per player (`blue`, `green`, `purple`, `red`, `yellow`; `brown` = raiders).
- **Locations**: `{"plane": "Arcanus" | "Myrror", "x": 12, "y": 7}`. A city is identified by its
  location; so is a player's stack (one stack per tile per player).
- Buildings, units and spells by name (as shown in the game, e.g. `"Barracks"`, `"Spearmen"`,
  `"Fireball"`).
- Lists are `[]` when empty, never `null`. Optional fields are omitted when empty.

## Commands (frontend → model)

`{"type": <type>, "command": {...}}`. Rejected with an error if it isn't the player's turn or the
command isn't legal (not your city, not enough gold, no path...).

| type | command | effect |
|---|---|---|
| `move-stack` | `from`, `to` (locations) | find a path, walk as far as moves go; battles/encounters on the way |
| `set-research` | `spell` | must be one of the research candidates |
| `set-tax-rate` | `numerator`, `denominator` | 0, 1/2, 1, 3/2, 2, 5/2 or 3 gold per citizen |
| `set-city-production` | `city`, `building` or `unit` | what the city builds |
| `set-city-farmers` | `city`, `farmers` | at least the subsistence farmers |
| `buy-production` | `city` | pay gold to finish the current production |
| `sell-building` | `city`, `building` | one sale per city per turn |
| `build-action` | `stack` | settle an outpost / meld a node / purify |
| `build-road` | `stack`, `to` | engineers build a road over the next turns |
| `end-turn` | | the frontend's turn loop moves on |
| `cast-spell` | `spell`, `target?` | cast now if mana and casting skill allow (spells that need a target must then come with one), otherwise start casting it over the next turns |
| `cast-spell-target` | `target` | the target for a finished spell announced by `choose-spell-target` |
| `answer-offer` | `accept` | accept or decline the open `hire-hero` / `hire-mercenaries` / `merchant` offer (a bought item then comes as a `vault` event) |
| `planar-travel` | `stack` | move the stack's active units to the other plane (tower, Astral Gate, Plane Shift) |
| `patrol` | `stack` | the stack's active units patrol their tile |
| `disband-unit` | `stack`, `unit` | dismiss the unit at that index in the stack |
| `set-unit-active` | `stack`, `unit`, `active` | whether the unit moves with the stack; `move-stack` only moves active units, the others stay behind as their own stack (that's how stacks split; they merge when they stop on the same tile) |
| `set-power-distribution` | `mana`, `research`, `skill` | shares of power, 0-1 each, adding up to 1 |
| `alchemy` | `direction` (`mana-to-gold`, `gold-to-mana`), `amount` | convert at half rate (full rate with the Alchemy retort) |
| `rename-city` | `city`, `name` | |
| `rename-hero` | `hero` (index 0-5), `name` | |
| `move-item` | `from`, `to` (item slots) | swap two slots; 20 mana for each item that arrives at a hero somewhere else |
| `destroy-item` | `item` (item slot) | destroy for mana (half its cost, full cost with Artificer) |
| `propose-treaty` | `wizard`, `treaty` (`pact`, `alliance`, `peace`) | the other wizard currently always agrees |
| `offer-tribute` | `wizard`, `gold: true` or `spell` | a tenth of your gold, or a spell they don't know |
| `trade-spells` | `wizard`, `spell` | if they're willing, they give a spell of their choice back (`learned-spell` event) |

Item slots are `{"vault": 0-3}`, `{"hero": 0-5, "slot": 0-2}` or `{"new": i}` (an item that just
arrived at the vault, announced by a `vault` event, waiting to be placed or destroyed). Create
Artifact and Enchant Item take the item as `"artifact": {"type": "Sword", "name": "...",
"image": 3, "powers": ["+2 Attack", "Flaming"], "spell": "Fire Bolt", "charges": 2}` in
`cast-spell` (at most 4 powers, one per stat, abilities need spellbooks, Enchant Item only cheap
powers, spell charges on staffs and wands).

Answers while a command is paused (see Sessions):

| type | command | effect |
|---|---|---|
| `confirm-encounter` | `enter` | enter the lair / node / tower (`choose-encounter` event) or stay out |
| `raze-city` | `raze` | raze the captured city or keep it (`choose-raze` event) |
| `battle-act` | `x`, `y` | the selected unit moves to / attacks the battlefield tile (like a click) |
| `battle-done` | | the selected unit is done this turn |
| `battle-wait` | | the selected unit waits, the next one is up |
| `battle-flee` | | your army flees |
| `battle-auto` | | the AI fights the rest of the battle for you |
| `battle-cast` | `spell`, `caster?`, `target?` | cast a combat spell: the wizard's (no `caster`; once per turn, within the army's mana pool and your mana over the range) or the selected unit's (`caster` = its id; its spells within its casting skill, or item charges). `target` is `{"unit": id}` or `{"x", "y"}` when the spell needs one (also for picking from the units Raise/Animate Dead offer). Nothing is paid if the target is refused |

During a battle, `session.Battle.View()` gives the battlefield: `turn`, `selected` (the id of your
unit that is up), `you-attack`, `width`, `height`, `units` (id, name, owner, yours, x, y, health,
max-health, moves-left, ranged-attacks) and `state`. Ask for it again after each battle command.

A spell target is `{"location": {...}, "unit": 2, "wizard": "green", "choice": "..."}`, with the
fields the spell needs:
- unit spells (enchantments, Black/Chaos Channels, Lycanthropy, Word of Recall): `location` of your
  stack and `unit`, the unit's index in it (the order of the player view's stack units);
- city spells (enchantments, curses, Spell Ward, Wall of Stone, Summoning Circle, Move Fortress,
  Spell of Return, Earthquake, Call the Void): `location` of the city; Spell Ward also `choice`,
  the ward's name;
- stack and map spells (Black Wind, Stasis, Ice/Fire Storm, Nature's Cures, Plane Shift, Earth
  Lore, Enchant Road, Change Terrain, Transmute, Raise Volcano, Corruption, Warp Node, Disenchant
  Area/True, Floating Island): `location`;
- spells on a wizard (Spell Blast, Cruel Unminding, Drain Power, Subversion): `wizard` (banner);
- Spell Binding, Disjunction (True): `wizard` (the owner) and `choice`, the global enchantment;
- Resurrection: `choice`, the dead hero's name.

Summons, hero summons, global enchantments, Death Wish and Great Unsummoning need no target.
Refused for now: Spell of Mastery (its victory screens).

Example: `{"type": "move-stack", "command": {"from": {"plane": "Arcanus", "x": 3, "y": 3}, "to": {"plane": "Arcanus", "x": 6, "y": 4}}}`

Not yet: the diplomacy steps the other wizard initiates.

## Events (model → frontend)

`{"type": <type>, "event": {...}}`.

| type | fields | meaning / expected reaction |
|---|---|---|
| `refresh` | | state changed: fetch the views again |
| `notice` | `message` | show a message |
| `scroll` | `title`, `text`, `old` | chancellor / report message |
| `random-event` | `event`, `starting`, `message`, `good`, `city?`, `player?` | random event started or ended |
| `diplomacy` | `player`, `enemy` | first meeting with a wizard |
| `choose-research` | `player`, `candidates` | pick research (answer with `set-research`) |
| `choose-spell-target` | `player`, `spell`, `target` | a finished spell needs a target of kind `target` (`friendly-unit`, `enemy-unit`, `friendly-city`, `enemy-city`, `any`, `land`, `empty-water`, `change-terrain`, `transmute`, `raise-volcano`, `enemy-melded-node`, `disenchant`, `wizard`, `enemy-global-enchantment`, `dead-hero`); answer with `cast-spell-target` |
| `learned-spell` | `player`, `spell`, `magic` | research finished |
| `new-building` | `player`, `city`, `building` | a building finished |
| `new-outpost` | `player`, `city` | an outpost was settled |
| `hire-hero` | `player`, `hero`, `cost` | hero for hire offer |
| `hire-mercenaries` | `player`, `units`, `cost` | mercenaries offer |
| `merchant` | `player`, `artifact`, `cost` | merchant offer |
| `treasure` | `player`, `location`, `items` | treasure from an encounter; items have `kind` (`gold`, `mana`, `item`, `prisoner`, `spell`, `spellbook`, `retort`) and `text` |
| `vault` | `player`, `artifact` | an item went to the vault |
| `hero-level-up` | `hero` | |
| `banish` | `attacker`, `defender` | a wizard's fortress city fell |
| `battle-started` | `attacker`, `defender`, `location` | a battle with you started: send battle commands |
| `battle-ended` | `attacker`, `defender`, `location`, `result` | `attacker-won`, `defender-won`, `attacker-fled`, `defender-fled` |
| `choose-encounter` | `player`, `encounter`, `guards` | enter? answer with `confirm-encounter` |
| `choose-raze` | `player`, `city` | raze or keep? answer with `raze-city` |

Heroes: `{"name", "title", "level", "location"}`. Artifacts: the save-game form
(`core/artifact.SerializedArtifact`).

## Views (model → frontend, on request)

- **Player view** (`MakePlayerView`): `turn`, `date`, `current-player`, `you` (gold, mana, per-turn
  income, fame, tax rate, casting skill, research and casting progress, known spells, global
  enchantments), `known-wizards`, `cities` (full detail: citizens, production, buy cost,
  buildings, enchantments), `stacks` (units with health and moves, remaining path),
  `foreign-cities` and `foreign-stacks` (only where the player's fog shows them).
- **Map view** (`MakeMapView`, per plane): `width`, `height`, `tiles[y][x]` with `terrain`, `road`,
  `bonus`, `encounter`, `corrupted`, `visible`; `null` for unexplored tiles.

## Trying it

```sh
go run ./util/headless-game -years 100 -events     # every event as JSON
go run ./util/headless-game -years 50 -view        # a wizard's view at the end
```
