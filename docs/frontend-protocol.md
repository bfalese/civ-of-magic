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

A spell target is `{"location": {...}, "unit": 2}`: a tile (a city or a stack), plus for spells on
a single unit the unit's index in your stack there (the order of the player view's stack units).
Spells castable this way: unit enchantments (and Black/Chaos Channels, Lycanthropy), city
enchantments and curses, summons, Summon Hero/Champion, Incarnation, global enchantments, Death
Wish, Great Unsummoning, Black Wind, Stasis, Ice/Fire Storm, Nature's Cures, Plane Shift, Spell of
Return, Earthquake. The rest (Spell Ward, Subversion, Spell Binding, Disjunction, Resurrection,
Create Artifact, map spells such as Change Terrain) are refused for now.

Example: `{"type": "move-stack", "command": {"from": {"plane": "Arcanus", "x": 3, "y": 3}, "to": {"plane": "Arcanus", "x": 6, "y": 4}}}`

Not yet: planar travel, answering offers (hire hero, mercenaries, merchant), unit orders (patrol,
wait, disband), the spells listed above as refused.

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
| `choose-spell-target` | `player`, `spell`, `target` | a finished spell needs a target of kind `target` (`friendly-unit`, `enemy-unit`, `friendly-city`, `enemy-city`); answer with `cast-spell-target` |
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
