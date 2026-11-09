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

- **Units** are identified by `id` (the player view lists it per unit). A unit keeps its id when
  stacks split, merge or reorder; ids aren't reused within a game (they aren't saved with it).
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
| `set-city-production` | `city`, `building` or `unit` | what the city builds: one of the city view's `can-build` (prerequisites met) |
| `set-city-farmers` | `city`, `farmers` | at least the subsistence farmers |
| `buy-production` | `city` | pay gold to finish the current production |
| `sell-building` | `city`, `building` | one sale per city per turn |
| `build-action` | `stack` | settle an outpost / meld a node / purify |
| `build-road` | `stack`, `to` | engineers build a road over the next turns |
| `end-turn` | | the frontend's turn loop moves on (with `GameHost`: the AI turns; may wait for a battle an AI starts against you) |
| `cast-spell` | `spell`, `target?` | cast now if mana and casting skill allow (spells that need a target must then come with one), otherwise start casting it over the next turns |
| `cast-spell-target` | `target` | the target for a finished spell announced by `choose-spell-target` |
| `answer-offer` | `accept` | accept or decline the open `hire-hero` / `hire-mercenaries` / `merchant` offer (a bought item then comes as a `vault` event) |
| `planar-travel` | `stack` | move the stack's active units to the other plane (tower, Astral Gate, Plane Shift) |
| `patrol` | `stack` | the stack's active units patrol their tile |
| `disband-unit` | `unit` (id) | dismiss the unit |
| `set-unit-active` | `unit` (id), `active` | whether the unit moves with the stack; `move-stack` only moves active units, the others stay behind as their own stack (that's how stacks split; they merge when they stop on the same tile) |
| `set-power-distribution` | `mana`, `research`, `skill` | shares of power, 0-1 each, adding up to 1 |
| `alchemy` | `direction` (`mana-to-gold`, `gold-to-mana`), `amount` | convert at half rate (full rate with the Alchemy retort) |
| `rename-city` | `city`, `name` | |
| `rename-hero` | `hero` (index 0-5), `name` | |
| `move-item` | `from`, `to` (item slots) | swap two slots; 20 mana for each item that arrives at a hero somewhere else |
| `destroy-item` | `item` (item slot) | destroy for mana (half its cost, full cost with Artificer) |
| `propose-treaty` | `wizard`, `treaty` (`pact`, `alliance`, `peace`) | the other wizard currently always agrees |
| `offer-tribute` | `wizard`, `gold: true` or `spell` | a tenth of your gold, or a spell they don't know |
| `trade-spells` | `wizard`, `spell` | if they're willing, they name the spell they'd give back (`trade-offer` event) |
| `answer-trade` | `accept` | make the offered exchange (`learned-spell` event) or decline it; an offer lasts until your next turn |

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
| `battle-cast` | `spell`, `caster?`, `target?` | cast a combat spell: the wizard's (no `caster`; once per turn, within the army's mana pool and your mana over the range) or the selected unit's (`caster` = its id; its spells within its casting skill, or item charges). `target` is `{"unit": id}` or `{"x", "y"}` when the spell needs one (also for picking from the units Raise/Animate Dead offer). Nothing is paid if the target is refused. Without `target`, a spell that needs one waits for it: the view's `casting` lists what it accepts |
| `battle-target` | `unit?`, `x?`, `y?` | the target for the spell waiting in `casting`; a refused target leaves it waiting |
| `battle-cancel` | | drop the spell waiting for a target (nothing is paid); any other battle command drops it too |

During a battle, `session.Battle.View()` gives the battlefield: `turn`, `selected` (the id of your
unit that is up), `you-attack`, `width`, `height`, `units` (id, name, owner, yours, x, y, health,
max-health, moves-left, ranged-attacks, `asset`/`banner`/`facing` for its combat sprite, `figures`
(pixel offsets of its living figures from the tile centre), `flying`, and for unit details
`melee`, `ranged`, `ranged-type`, `defense`, `resistance`, `hit-points` (per figure),
`figure-count`, `level`, `abilities`, `enchantments`), `tiles[y][x]` (`asset`, `top`
and `top-align` for a tree/rock/building, `mud`), `road`/`clouds` for a town, `moves` (tiles the
selected unit can walk to), `targets` (`{id, kind}`, enemies it can attack: `ranged` or `melee`)
and `state`. Ask for it again after each battle command.

What happened: `steps` lists what the last battle command and the AI moves after it did, in
order, so a frontend can replay them from the previous view (the core resolves everything at
once). A step is `{kind, unit?, path?, target?, spell?, changes?}`: `move` (`path`: the tiles
walked), `teleport` (`path`: the destination), `melee` / `ranged` (`target`), `spell` (cast by
you; `unit` is the casting unit, absent for the wizard), `effect` (health changed otherwise: the
AI's spells, walls, vortexes). `changes` are `{unit, damage, killed?}` (negative damage heals),
counterattacks included. `log` holds the latest battle log lines (`{turn, text}`, oldest first).
Units turn to face where they walk and whom they attack.

Magic in the battle view: `magic` is the wizard's (`mana-pool`, `mana`, `range`, `available` = the
most a spell can cost now, `casted` this turn, `spells`); `unit-magic` the selected unit's when it
has spells or item charges (`caster` id, `casting-skill`, `spells`). A spell is `{name, magic
(realm), cost, castable, charges?}`; `castable` means a battle-cast would be accepted now.
`casting` is the spell waiting for a target: `spell`, `caster?`, `kind` and the targets it accepts,
by the model's own checks: `units` (ids, for `kind: "unit"`), `tiles` (`[x, y]`, for `"tile"`) or
`choices` (`{id, name, asset, banner}` with a `title`, for `"choice"`: Raise/Animate Dead, which
offer killed units).

A spell target is `{"location": {...}, "unit": 2, "wizard": "green", "choice": "..."}`, with the
fields the spell needs:
- unit spells (enchantments, Black/Chaos Channels, Lycanthropy, Word of Recall): `unit`, the
  unit's id (`location` may be left out; if given it must be the unit's tile);
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
| `learned-spell` | `player`, `spell`, `magic` | research finished (or a spell trade, treasure) |
| `trade-offer` | `player`, `wizard`, `give`, `receive` | the wizard would trade `receive` for your `give`: answer with `answer-trade` |
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
  buildings, enchantments; `subsistence-farmers`, the fewest farmers allowed; `can-build`, what
  `set-city-production` accepts, as `{name, unit?, cost}`, buildings in the game's order then units;
  `sell-values`, gold per sellable building), `stacks` (units with id, health and moves, remaining path),
  `foreign-cities` and `foreign-stacks` (only where the player's fog shows them).
- **Map view** (`MakeMapView`, per plane): `width`, `height`, `tiles[y][x]` with `terrain`, `road`,
  `bonus`, `encounter`, `corrupted`, `visible`; `null` for unexplored tiles.
- **Sprites**: views name their images with asset IDs (`core/assets`): a tile's `asset` (terrain)
  and `overlays` (road pieces, bonus, lair/tower, corruption, a melded node's zone, in drawing
  order); a unit's `asset`; a stack's `background` (the banner square behind its leader, who is
  drawn 1 pixel right and down); a city's `asset`; a foreign stack's `asset` and `background`.

## Assets

`util/export-assets` writes the overland sprites from an original install (never committed):

```sh
go run ./util/export-assets -data "/path/to/Master of Magic" -out ~/.local/share/civ-of-magic/assets
```

`assets.json`: `{"version": 1, "tileWidth": 20, "tileHeight": 18, "sheets": ["sheet-0.png", ...],
"sprites": {"<asset id>": [[sheet, x, y, width, height], ...]}}`, one rectangle per animation frame
(animated terrain has 4). IDs: `terrain/<tile index>`, `unit/<race>/<unit>/<banner>`,
`unit-background/<banner>`, `city/<hamlet|village|town|city|capital>/<open|walled>/<banner>`,
`road/<center|n|ne|...>`, `road-enchanted/...`, `bonus/<name>`, `encounter/<name>`,
`encounter/open-tower`, `corruption`, `node-zone/<banner>`; battles: `battle-tile/<lbx>/<index>`,
`battle-top/<lbx>/<index>` (cropped), `battle-mud`, `battle-road`, `battle-clouds`,
`battle-unit/<race>/<unit>/<facing>` (facing `up`, `up-right`, ..., all frames). Battle units are
banner-neutral: pixels in the manifest's `bannerMarker` colours (4) are replaced with
`bannerColors[banner]` by the frontend. Replacing art = replacing the
rectangle behind an ID.

## Trying it

```sh
go run ./util/headless-game -years 100 -events     # every event as JSON
go run ./util/headless-game -years 50 -view        # a wizard's view at the end
```

## A whole game for one player: `GameHost` and the wasm core

`core/game/host.go`: `MakeGameHost(gameData, HostSettings)` sets up a game where player 0 is a
human who plays through commands, and owns the turn loop. `host.Execute` / `ExecuteJSON` runs a
command through the player's `CommandSession`; an `end-turn` plays the AI wizards' turns and
returns at the start of the human's next turn. `host.TakeEvents()` returns the events since the
last call (all of the model's events, including the AI players': show the ones that concern you).
Views: `PlayerView()`, `MapView(plane)`, `BattleView()`. `GameOver()` is `""`, `"won"` or `"lost"`.

Settings (JSON): `opponents` (1-4, default 3), `landSize` (0-2), `raiders` (default true),
`difficulty` (`intro` … `impossible`, default `average`), `magic` (`weak`, `normal`, `powerful`),
`wizardName`.

When an AI player (a wizard or raiders) attacks the human during `end-turn`, the AI turns pause:
`end-turn` returns with `WaitingFor()` = `battle` (and a `battle-started` event with the human as
`defender`). Answer with battle commands as for your own attacks (`battle-auto` lets the AI fight
it for you). The AI turns go on after the battle, so the command that ends it returns once the human's
next turn has started. Until then only battle commands are accepted.

`frontend/wasm` exposes a host to JavaScript as `globalThis.civ` (core only, no Ebitengine):

| function | returns |
|---|---|
| `civ.newGame(gamedataJSON, settingsJSON)` | status |
| `civ.execute(commandJSON)` | status |
| `civ.events()` | JSON array of events since the last call |
| `civ.view()`, `civ.map("Arcanus" \| "Myrror")`, `civ.battle()` | the view as JSON (`{"error": ...}` on failure) |
| `civ.setLogging(bool)` | the Go log on the console (off by default) |

Status is `{"error"?, "waiting", "gameOver"?}`. Every function returns a JSON string. The page
calls `window.onCivReady` (if defined) once the API is there. The page loads the user's own
`gamedata.json`; it is never shipped.

```sh
make core-wasm                                     # frontend/wasm/core.wasm + wasm_exec.js
node frontend/wasm/smoke.cjs frontend/wasm/core.wasm ~/.local/share/civ-of-magic/gamedata/gamedata.json 30
python3 -m http.server -d frontend/wasm 8080       # test page; pick gamedata.json, or
                                                   # link it into frontend/wasm and open ?gamedata=gamedata.json
```
