"""Smoke test for the web frontend, in a real browser (Playwright, Chromium).

Starts a game from the start screen (wizard, race, banner, settings). Then plays the first turn
through the page the way a person would: opens the capital's city screen,
changes production, moves a farmer, sells a building, selects the garrison, moves it, ends the
turn and answers the research question. Then starts the battle scenario (a guarded lair next to
the capital, Fire Bolt known; see core/game/scenario.go), enters the lair, casts the spell, moves
a unit, checks the replay and unit details, and lets auto finish the battle. Last, the siege
scenario: attacks an AI capital with city walls, a wall of fire and darkness, checks they are
on the field, and flees. Then the spellbook scenario: Spell Ward and Resurrection through the
choice dialog, a sword made with Create Artifact in the item designer, the tax rate, renaming
the capital and cancelling the ward, and diplomacy with the rival. Last, the items
scenario: equips a hero from the items screen, destroys an item, renames the hero, and splits,
inspects and disbands units in the stack panel. Last, the orders scenario: settles an outpost,
starts a road (with its path shown while picking), travels through a tower and patrols, and
checks the city and army lists. Then
saving: the autosave, a save, a downloaded file, loading each back, and Continue on the start
screen.

It fails fast: at the first page error, and after a few seconds when the page doesn't do what a
step expects (the page answers in well under a second). On a failure it saves a screenshot.

    pip install --user playwright && python3 -m playwright install chromium
    go run ./util/serve-web &           # needs the exported gamedata and assets
    python3 frontend/web/smoke_test.py [http://localhost:8080] [--shots DIR]
"""

import json
import os
import sys
import time

from playwright.sync_api import sync_playwright

# how long a step may take: clicks, a screen opening, a command's answer
STEP_TIMEOUT = 5000
# loading the wasm core and starting a game
LOAD_TIMEOUT = 20000
# work that really takes a while: auto fighting a whole battle
BATTLE_TIMEOUT = 20000


class Smoke:
    def __init__(self, page, shots):
        self.page = page
        self.shots = shots
        self.errors = []
        page.on("pageerror", lambda error: self.errors.append(str(error)))
        page.on("console", lambda message: self.errors.append(message.text) if message.type == "error" else None)

    def shot(self, name):
        if self.shots:
            self.page.screenshot(path=os.path.join(self.shots, f"{name}.png"))

    def fail(self, message):
        path = os.path.join(self.shots or ".", "failure.png")
        try:
            self.page.screenshot(path=path)
            message += f" (screenshot: {path})"
        except Exception:
            pass
        if self.errors:
            message += "\npage errors:\n  " + "\n  ".join(self.errors)
        raise AssertionError(message)

    def no_errors(self):
        if self.errors:
            self.fail("page error")

    def check(self, condition, message):
        self.no_errors()
        if not condition:
            self.fail(message)
        print(f"ok  {message}")

    def state(self, expression):
        return self.page.evaluate(f"(() => {{ const s = window.civGame; return {expression}; }})()")

    def wait(self, expression, what, timeout=STEP_TIMEOUT):
        """Poll a JavaScript expression until it's truthy; fail at a page error or the timeout."""
        deadline = time.monotonic() + timeout / 1000
        while True:
            self.no_errors()
            value = self.page.evaluate(f"(() => {{ try {{ return {expression}; }} catch (error) {{ return null; }} }})()")
            if value:
                return value
            if time.monotonic() > deadline:
                self.fail(f"timed out after {timeout / 1000:g}s waiting for {what}")
            self.page.wait_for_timeout(50)

    def visible(self, selector, what, timeout=STEP_TIMEOUT):
        self.wait(f"(() => {{ const element = document.querySelector({json.dumps(selector)}); return !!element && element.getClientRects().length > 0; }})()", what, timeout)

    def load(self, url, answer_research=True):
        self.page.goto(url)
        self.wait("window.civGame && window.civGame.view", "the game to load", LOAD_TIMEOUT)
        # a new game asks for the first research at once
        if answer_research and self.page.locator("#modal.open").count():
            self.page.click("#modalChoices button >> nth=0")


def main():
    arguments = [argument for index, argument in enumerate(sys.argv[1:], 1)
                 if not argument.startswith("--") and sys.argv[index - 1] != "--shots"]
    url = (arguments[0] if arguments else "http://localhost:8080").split("?")[0]
    shots = sys.argv[sys.argv.index("--shots") + 1] if "--shots" in sys.argv else None
    began = time.monotonic()

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(args=["--use-angle=swiftshader", "--enable-unsafe-swiftshader"])
        page = browser.new_page(viewport={"width": 1200, "height": 800})
        page.set_default_timeout(STEP_TIMEOUT)
        smoke = Smoke(page, shots)
        try:
            start_screen(smoke, url)
            first_turn(smoke, url)
            battle(smoke, url)
            siege(smoke, url)
            spellbook(smoke, url)
            items(smoke, url)
            orders(smoke, url)
            saves(smoke, url)
        finally:
            browser.close()

    print(f"smoke test passed in {time.monotonic() - began:.1f}s")


def start_screen(smoke, url):
    """Without URL parameters the page asks for the wizard, race, banner and game settings."""
    page, state, check = smoke.page, smoke.state, smoke.check
    page.goto(url)
    smoke.visible("#start .start-wizard", "the start screen")
    wizards = page.locator("#start .start-wizard").count()
    page.click("#start .start-wizard[data-wizard='Freya']")
    page.select_option("#start [data-race]", "Klackon")
    page.click("#start .swatch[data-banner='purple']")
    page.select_option("#start [data-opponents]", "1")
    page.select_option("#start [data-difficulty]", "easy")
    page.fill("#start [data-name]", "Smoky")
    smoke.shot("0-start")
    page.click("#start [data-start]")
    smoke.wait("window.civGame && window.civGame.view", "the game to load", LOAD_TIMEOUT)
    you = state("s.view.you")
    capital = state("s.view.cities[0]")
    check(you["player"]["wizard"] == "Smoky" and you["player"]["banner"] == "purple" and capital["race"] == "Klackon"
          and len(state("s.core.view()['known-wizards']")) <= 1,
          f"the start screen ({wizards} wizards) made Smoky (Freya), purple, with a Klackon capital {capital['name']}")


def first_turn(smoke, url):
    page, state, check = smoke.page, smoke.state, smoke.check
    smoke.load(url + "?opponents=1", answer_research=False)
    check(state("s.view.cities.length") == 1, "the game starts with a capital")
    check(state("s.map.assets.has('fog/0') && s.map.assets.has('fog/13')"), "the soft fog edges are in the assets (re-export them if not)")
    smoke.visible("#modal.open", "the first research question")
    candidates = state("s.view.you['research-candidates'].length")
    page.click("#modalChoices button >> nth=0")
    check(state("s.view.you.researching") is not None, f"the first turn asks what to research: {state('s.view.you.researching')} (of {candidates})")
    smoke.shot("1-start")

    # the camera starts on the capital: its tile is in the middle of the map area
    page.mouse.click(600, 400)
    smoke.visible("#city.open", "the city screen")
    check(state("s.cityOpen !== null"), "clicking the capital opens its city screen")
    smoke.shot("2-city")

    options = state("s.view.cities[0]['can-build'].map(o => o.name)")
    producing = state("s.view.cities[0].producing")
    choice = next(name for name in options if name != producing and name not in ("Trade Goods", "Housing"))
    page.click(f"#city button.option[data-name='{choice}']")
    check(state("s.view.cities[0].producing") == choice, f"production changed to {choice}")

    farmers = state("s.view.cities[0].farmers")
    if page.is_enabled("#fewerFarmers"):
        page.click("#fewerFarmers")
        check(state("s.view.cities[0].farmers") == farmers - 1, "one farmer fewer (a worker more)")
        page.click("#moreFarmers")
        check(state("s.view.cities[0].farmers") == farmers, "and back")
    elif page.is_enabled("#moreFarmers"):
        page.click("#moreFarmers")
        check(state("s.view.cities[0].farmers") == farmers + 1, "one more farmer")
        page.click("#fewerFarmers")
        check(state("s.view.cities[0].farmers") == farmers, "and back")
    values = state("s.view.cities[0]['sell-values']")
    if values:
        name, value = sorted(values.items())[0]
        gold = state("s.view.you.gold")
        page.click(f"#city button[data-sell='{name}']")
        check(state("s.view.you.gold") == gold + value and name not in state("s.view.cities[0].buildings"), f"sold {name} for {value} gold")
        check(page.locator("#city button[data-sell]:enabled").count() == 0, "one sale per turn: the other sell buttons are off")
    smoke.shot("3-city-changed")

    page.click("#selectGarrison")
    check(not state("s.cityOpen") and state("s.selectedUnit !== null"), "the garrison is selected and the screen closed")

    # maps are random: try the neighbouring tiles (80x72 screen pixels at the start zoom) until
    # one can be entered
    start = state("JSON.stringify(s.view.stacks)")
    for dx, dy in [(80, 0), (-80, 0), (0, -72), (0, 72), (80, 72), (-80, -72), (80, -72), (-80, 72)]:
        if state("s.selectedUnit") is None:
            break
        page.mouse.click(600 + dx, 400 + dy)
        if state("JSON.stringify(s.view.stacks)") != start:
            break
    moved = state("JSON.stringify(s.view.stacks.map(stack => stack.location))")
    check(state("JSON.stringify(s.view.stacks)") != start, f"the garrison moved ({moved})")
    smoke.shot("4-moved")

    date = state("s.view.date")
    page.keyboard.press("Enter")
    smoke.wait(f"window.civGame.view.date !== {json.dumps(date)}", "the next turn")
    check(True, f"end turn: {date} -> {state('s.view.date')}")
    # other questions may come with the new turn (offers, ...): answer them
    while page.locator("#modal.open").count():
        page.click("#modalChoices button >> nth=0")
    check(state("s.view.you.researching") is not None, f"still researching {state('s.view.you.researching')}")
    smoke.shot("5-next-turn")

    magic_screen(smoke)


def magic_screen(smoke):
    """M opens the magic screen: change the power shares, cast or start a spell, alchemy, Esc."""
    page, state, check = smoke.page, smoke.state, smoke.check
    page.keyboard.press("m")
    smoke.visible("#magic.open", "the magic screen")
    spells = state("s.view.you.spells")
    listed = page.locator("#magic .spell").count()
    check(listed == len(spells), f"M opens the magic screen: {listed} spells in the spellbook, power {state('s.view.you.power')}")
    smoke.shot("5b-magic")

    research = state("s.view.you['power-distribution'].research")
    wanted = 60 if round(research * 100) != 60 else 40
    page.evaluate(f"""(() => {{ const input = document.querySelector("#magic input[data-share='research']");
        input.value = {wanted}; input.dispatchEvent(new Event('change')); }})()""")
    share = state("s.view.you['power-distribution']")
    check(abs(share["research"] - wanted / 100) < 0.01 and abs(sum(share.values()) - 1) < 0.01,
          f"research share {round(research * 100)}% -> {round(share['research'] * 100)}%, mana {round(share['mana'] * 100)}%, skill {round(share['skill'] * 100)}%")

    spell = next((spell for spell in spells if not spell.get("target") and not spell.get("needs-design")), None)
    if spell:
        mana = state("s.view.you.mana")
        page.click(f"#magic .spell[data-spell={json.dumps(spell['name'])}]")
        you = state("s.view.you")
        if spell["now"]:
            check(you["mana"] < mana, f"cast {spell['name']} at once for {spell['cost']} mana")
        else:
            check(you.get("casting") == spell["name"], f"started casting {spell['name']} ({you['casting-progress']}/{you['casting-cost']})")
    else:
        print("skip cast: no overland spell without a target")

    gold, mana = state("s.view.you.gold"), state("s.view.you.mana")
    if gold >= 2:
        page.fill("#magic [data-alchemy-amount]", "2")
        page.click("#magic button[data-alchemy='gold-to-mana']")
        you = state("s.view.you")
        check(you["gold"] == gold - 2 and you["mana"] > mana, f"alchemy: 2 gold -> {you['mana'] - mana} mana")
    page.keyboard.press("Escape")
    check(not state("s.magic.open"), "Esc closes the magic screen")


# the lair the battle scenario puts next to the capital
FIND_LAIR = """(() => {
    const s = window.civGame;
    const stack = s.view.stacks.find((stack) => stack.units.length);
    if (!stack) return null;
    const map = s.core.map(stack.location.plane);
    for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1], [1, 1], [-1, -1], [1, -1], [-1, 1]]) {
        const x = (stack.location.x + dx + map.width) % map.width, y = stack.location.y + dy;
        const tile = map.tiles[y] && map.tiles[y][x];
        if (tile && tile.encounter) return { from: stack.location, to: { plane: stack.location.plane, x, y }, kind: tile.encounter };
    }
    return null;
})()"""


def tile_point(page, x, y):
    """A battlefield tile's centre in page pixels."""
    return page.evaluate(f"""(() => {{
        const battle = window.civGame.battle;
        const [cx, cy] = battle.center({x}, {y});
        const bounds = battle.canvas.getBoundingClientRect();
        return [bounds.left + cx * bounds.width / battle.canvas.width, bounds.top + cy * bounds.height / battle.canvas.height];
    }})()""")


def battle_view(page):
    return page.evaluate("window.civGame.core.battle()")


def cast_spell(smoke):
    """Open the spell list and cast Fire Bolt on an enemy: once cancelled with Esc, then on the field."""
    page, check = smoke.page, smoke.check
    magic = battle_view(page)["magic"]
    page.keyboard.press("s")
    smoke.visible("#battle .battle-spells .battle-spell", "the spell list")
    listed = page.locator("#battle .battle-spells .battle-spell").count()
    check(listed >= len(magic["spells"]), f"S opens the spell list: {listed} spells, {magic['available']} mana available")
    smoke.shot("6b-spells")

    spell = next((spell for spell in magic["spells"] if spell["name"] == "Fire Bolt"), None)
    check(spell is not None and spell["castable"], "the scenario's Fire Bolt is castable")
    disrupt = page.locator("#battle .battle-spell[data-spell='Disrupt'] >> nth=0")
    check(disrupt.is_disabled() and "nothing to cast it on" in (disrupt.get_attribute("title") or ""),
          "Disrupt is greyed out at a lair (no walls): nothing to cast it on")
    button = "#battle .battle-spell[data-spell='Fire Bolt'] >> nth=0"
    page.click(button)
    casting = battle_view(page).get("casting")
    check(casting is not None and casting["kind"] == "unit" and casting.get("units"), f"Fire Bolt waits for a unit target: {casting}")
    smoke.shot("6c-casting")

    page.keyboard.press("Escape")
    view = battle_view(page)
    check(not view.get("casting") and not view["magic"]["casted"], "Esc cancels it without paying")

    page.keyboard.press("s")
    page.click(button)
    casting = battle_view(page)["casting"]
    target = next(unit for unit in view["units"] if unit["id"] == casting["units"][0])
    point = tile_point(page, target["x"], target["y"])
    page.mouse.click(point[0], point[1])
    if not smoke.state("s.battle.open"):
        check(True, "Fire Bolt ended the battle")
        return False
    view = battle_view(page)
    spell_step = next((step for step in view["steps"] if step["kind"] == "spell"), None)
    check(view["magic"]["casted"] and not view.get("casting") and spell_step, f"cast Fire Bolt on {target['name']}: {page.inner_text('#battle .battle-hint')}")
    effect = spell_step.get("effect")
    check(effect == "spell-effect/fire-bolt" and spell_step.get("target") == target["id"] and page.evaluate(f"window.civGame.battle.assets.has({json.dumps(effect)})"),
          f"the spell step plays {effect} on {target['name']} (re-export the assets if it's missing)")
    smoke.shot("6d-cast")
    return True


def battle(smoke, url):
    page, check = smoke.page, smoke.check
    smoke.load(url + "?opponents=1&scenario=battle")

    # a spell from the spellbook cast now on a target picked on the map: Wall of Stone on the capital
    page.keyboard.press("m")
    smoke.visible("#magic.open", "the magic screen")
    page.click("#magic .spell[data-spell='Wall of Stone']")
    check(not smoke.state("s.magic.open") and smoke.state("s.spellTarget && s.spellTarget.start"), "Wall of Stone asks for its target on the map")
    page.mouse.click(600, 400)
    buildings = smoke.state("s.view.cities[0].buildings")
    check("City Walls" in buildings and not smoke.state("s.spellTarget"), "clicking the capital casts it: the city has walls")

    lair = page.evaluate(FIND_LAIR)
    check(lair is not None, f"the scenario put a {lair and lair['kind']} next to the capital")

    status = page.evaluate(f"window.civGame.execute('move-stack', {json.dumps({'from': lair['from'], 'to': lair['to']})})")
    check(status.get("waiting") == "confirm-encounter", "walking into it asks whether to enter")
    page.click("#modalChoices button:has-text('Enter')")
    smoke.visible("#battle.open", "the battle screen")
    check(True, "entered: the battle screen is open")
    smoke.shot("6-battle")

    view = battle_view(page)
    check(len(view["moves"]) > 0 and view.get("selected") is not None, f"one of your units is up with {len(view['moves'])} tiles to move to")

    if not cast_spell(smoke):
        return
    view = battle_view(page)
    check(view.get("selected") is not None and view["moves"], "a unit is still up to move after the spell")

    # click the first reachable tile on the canvas
    x, y = view["moves"][0]
    point = tile_point(page, x, y)
    selected = view["selected"]
    page.mouse.click(point[0], point[1])
    after = battle_view(page)
    moved = next((unit for unit in after.get("units", []) if unit["id"] == selected), None)
    check(moved is not None and (moved["x"], moved["y"]) == (x, y), f"clicked tile {x},{y}: the unit walked there")
    smoke.shot("7-battle-moved")

    walk = next((step for step in after.get("steps", []) if step["kind"] == "move" and step.get("unit") == selected), None)
    check(walk is not None and walk["path"][-1] == [x, y], f"the view's steps have the walk: {walk and walk['path']}")
    check(smoke.state("s.battle.replayCount") >= 1, f"the screen replays the {len(after['steps'])} steps")
    smoke.wait("window.civGame.battle.replay === null", "the replay to end")
    check(True, "the replay ends")

    # hovering a unit shows its details
    enemy = next(unit for unit in after["units"] if not unit["yours"])
    point = tile_point(page, enemy["x"], enemy["y"])
    page.mouse.move(point[0], point[1])
    smoke.visible("#battle .battle-info:not([hidden])", "the unit details")
    info = page.inner_text("#battle .battle-info")
    check(enemy["name"] in info and "defense" in info, f"hovering {enemy['name']} shows its details: {' / '.join(info.splitlines()[:3])}")
    smoke.shot("7b-battle-info")

    page.click("#battle button[data-command='battle-auto']")
    smoke.wait("!window.civGame.battle.open", "auto to finish the battle", BATTLE_TIMEOUT)
    check(True, "auto finished the battle and the screen closed")
    log = page.inner_text("#log")
    check("the battle at" in log, f"the log reports it: {next((line for line in log.splitlines() if 'battle at' in line), '')}")


# the siege scenario's army: a stack of yours next to an enemy city
FIND_SIEGE = """(() => {
    const s = window.civGame;
    for (const army of s.view.stacks.filter((stack) => stack.units.length)) {
        // the map wraps east-west
        const width = s.core.map(army.location.plane).width;
        const near = (city) => {
            const dx = Math.abs(city.location.x - army.location.x);
            return Math.min(dx, width - dx) <= 1 && Math.abs(city.location.y - army.location.y) <= 1;
        };
        const city = s.view['foreign-cities'].find((city) => city.location.plane === army.location.plane && near(city));
        if (city) return { from: army.location, to: city.location, name: city.name };
    }
    return null;
})()"""


def siege(smoke, url):
    page, check = smoke.page, smoke.check
    smoke.load(url + "?opponents=1&scenario=siege")
    target = page.evaluate(FIND_SIEGE)
    check(target is not None, f"the siege scenario put an army next to {target and target['name']}")

    status = page.evaluate(f"window.civGame.execute('move-stack', {json.dumps({'from': target['from'], 'to': target['to']})})")
    check(status.get("waiting") == "battle", "attacking the city starts a battle")
    smoke.visible("#battle.open", "the battle screen")
    view = battle_view(page)
    walls = [wall for row in view["tiles"] for tile in row for wall in tile.get("walls", [])]
    stone = sum(1 for wall in walls if not wall.get("animated"))
    check(stone > 0 and len(walls) > stone and view.get("fortress"), f"the field has {stone} wall pieces, {len(walls) - stone} of fire and darkness, and the fortress")
    smoke.wait("window.civGame.battle.replay === null", "the AI's opening moves to replay", BATTLE_TIMEOUT)
    smoke.shot("8-siege")

    page.click("#battle button[data-command='battle-flee']")
    smoke.wait("!window.civGame.battle.open", "the battle to end after fleeing", BATTLE_TIMEOUT)
    check(True, "fled: the battle screen closed")


def spellbook(smoke, url):
    """The spellbook scenario: Spell Ward and Resurrection through the choice dialog, a sword
    designed and made with Create Artifact."""
    page, state, check = smoke.page, smoke.state, smoke.check
    smoke.load(url + "?opponents=1&scenario=spellbook")

    for name, question in [("Spell Ward", "which ward"), ("Resurrection", "which hero")]:
        page.keyboard.press("m")
        smoke.visible("#magic.open", "the magic screen")
        choices = state(f"s.view.you.spells.find(spell => spell.name === {json.dumps(name)}).choices")
        page.click(f"#magic .spell[data-spell={json.dumps(name)}]")
        smoke.visible("#modal.open", f"the {name} choice")
        check(question in page.inner_text("#modalText") and page.locator("#modalChoices button").count() == len(choices) + 1,
              f"{name} asks {question}: {len(choices)} choices")
        if name == "Spell Ward":
            smoke.shot("9-spell-ward")
        page.click("#modalChoices button >> nth=0")
        after = state(f"s.view.you.spells.find(spell => spell.name === {json.dumps(name)}).choices || []")
        check(len(after) < len(choices), f"cast {name} on {choices[0].get('city', '') + ' ' if choices[0].get('city') else ''}{choices[0]['name']}")

    page.keyboard.press("m")
    page.click("#magic .spell[data-spell='Create Artifact']")
    smoke.visible("#magic .designer", "the item designer")
    page.click("#magic button[data-type='Sword']")
    page.click("#magic .item-image >> nth=2")
    select = page.locator("#magic select[data-group='Attack']")
    option = select.locator("option:not([disabled])").nth(1).get_attribute("value")
    select.select_option(option)
    page.fill("#magic [data-name]", "Smoke Blade")
    cost = state("s.magic.designer.cost()")
    check(cost > state("s.view.you['item-design'].types.find(t => t.name === 'Sword')['base-cost']") * 0.4,
          f"designed a sword with {option}: cost {cost}")
    smoke.shot("10-item-designer")
    mana = state("s.view.you.mana")
    page.click("#magic button[data-create]")
    paid = mana - state("s.view.you.mana")
    check(state("!s.magic.designer") and abs(paid - cost) <= 1, f"Create Artifact made it for {paid} mana")
    smoke.no_errors()
    # the new item waits in the vault: whatever the page asks, leave it
    while page.locator("#modal.open").count():
        page.click("#modalChoices button >> nth=-1")
    page.keyboard.press("Escape")

    city_extras(smoke)
    diplomacy(smoke)


def diplomacy(smoke):
    """W opens the wizards screen: a pact, a spell as tribute, and a spell trade with the rival."""
    page, state, check = smoke.page, smoke.state, smoke.check
    page.keyboard.press("w")
    smoke.visible("#diplomacy.open", "the wizards screen")
    rival = state("s.view.diplomacy[0]")
    banner = rival["wizard"]["banner"]
    card = f"#diplomacy .wizard[data-wizard='{banner}']"
    check(page.locator("#diplomacy .wizard").count() == 1, f"W lists {rival['wizard']['wizard']}: {rival['mood']} ({rival['relation']}), {rival['treaty']}")
    page.click(f"{card} button[data-treaty='pact']")
    check(state("s.view.diplomacy[0].treaty") == "pact", "proposed a wizard's pact: accepted")

    gift = page.locator(f"{card} select[data-spell]").input_value()
    page.click(f"{card} button[data-tribute-spell]")
    check(gift not in state("s.view.diplomacy[0].spells"), f"gave {gift} as a tribute")

    if state("s.view.diplomacy[0]['can-teach']"):
        offered = page.locator(f"{card} select[data-spell]").input_value()
        page.click(f"{card} button[data-trade]")
        offer = state("s.view.diplomacy[0]['trade-offer']")
        if offer:
            check(page.locator(f"{card} .offer").count() == 1 and not state("s.modalOpen"), f"offered {offered}: they'd give {offer['receive']} (shown on the screen, no dialog)")
            smoke.shot("14-diplomacy")
            page.click(f"{card} button[data-answer='yes']")
            check(offer["receive"] in state("s.view.you['known-spells']"), f"accepted: learned {offer['receive']}")
        else:
            print(f"skip trade: {page.inner_text('#diplomacy .magic-message')}")
    else:
        print("skip trade: the rival knows no spell you don't")
    page.keyboard.press("Escape")
    check(not state("s.diplomacy.open"), "Esc closes the wizards screen")


def city_extras(smoke):
    """The tax rate in the top bar; renaming the capital and cancelling the ward cast on it."""
    page, state, check = smoke.page, smoke.state, smoke.check
    page.select_option("#tax", "4")
    rate = state("s.view.you['tax-rate']")
    check(rate == "2/1" and page.locator("#tax option >> nth=2").inner_text().startswith("1 ("), f"the top bar sets the tax rate: {rate}, {page.locator('#tax option:checked').inner_text()}")
    check(page.evaluate("document.getElementById('top').getBoundingClientRect().height") < 50, "the top bar still fits on one line")

    page.mouse.click(600, 400)
    smoke.visible("#city.open", "the city screen")
    page.click("#cityRename")
    page.fill("#cityName", "Smoketown")
    page.press("#cityName", "Enter")
    check(state("s.view.cities[0].name") == "Smoketown" and "Smoketown" in page.inner_text("#city h3"), "renamed the capital to Smoketown")

    wards = state("s.view.cities[0].enchantments")
    check(len(wards) >= 1 and wards[0]["yours"], f"the city screen lists {', '.join(ward['name'] for ward in wards)}")
    smoke.shot("13-city-extras")
    page.click("#city button[data-cancel-enchantment] >> nth=0")
    smoke.visible("#modal.open", "the cancel question")
    page.click("#modalChoices button:has-text('Cancel it')")
    left = state("s.view.cities[0].enchantments.length")
    check(left == len(wards) - 1, f"cancelled {wards[0]['name']}")
    page.keyboard.press("Escape")


def items(smoke, url):
    """The items scenario: a hero at the fortress, a sword in the vault, a shield just arrived."""
    page, state, check = smoke.page, smoke.state, smoke.check
    smoke.load(url + "?opponents=1&scenario=items")
    page.keyboard.press("i")
    smoke.visible("#vault.open", "the items screen")
    you = state("s.view.you.items")
    hero = you["heroes"][0]
    check(len(you["new"]) == 1 and page.locator("#vault .item-slot").count() == 1 + 4 + len(hero["slots"]),
          f"I opens the items screen: 1 new item, 4 vault slots, {hero['name']} with {len(hero['slots'])} slots")

    def slot_for(kind):
        return next(index for index, slot in enumerate(hero["slots"]) if kind in slot["takes"])

    def click(slot):
        page.click(f"#vault .item-slot[data-slot='{json.dumps(slot, separators=(',', ':'))}']")

    armor, weapon = slot_for("Shield"), slot_for("Sword")
    click({"new": 0})
    check(page.locator("#vault .item-slot.unfit").count() > 0, "holding the shield greys the slots it doesn't fit")
    smoke.shot("11-items")
    click({"hero": hero["index"], "slot": armor})
    click({"vault": 0})
    click({"hero": hero["index"], "slot": weapon})
    you = state("s.view.you.items")
    slots = you["heroes"][0]["slots"]
    check(not you["new"] and you["vault"][0] is None and slots[armor]["item"] and slots[weapon]["item"],
          f"{hero['name']} now carries {slots[armor]['item']['name']} and {slots[weapon]['item']['name']}")
    mana = state("s.view.you.mana")
    value = slots[weapon]["item"]["value"]
    click({"hero": hero["index"], "slot": weapon})
    page.click("#vault button[data-destroy]")
    check(state("s.view.you.mana") == mana + value and not state("s.view.you.items.heroes[0].slots")[weapon]["item"], f"destroyed the sword for {value} mana")
    page.click("#vault button[data-rename]")
    page.fill("#vault .rename input", "Smokey")
    page.keyboard.press("i")
    check(state("s.vault.open"), "typing an i in the name doesn't close the items screen")
    page.fill("#vault .rename input", "Smokey")
    page.press("#vault .rename input", "Enter")
    check(state("s.view.you.items.heroes[0].name") == "Smokey" and "Smokey" in page.inner_text("#vault .vault-hero"), f"renamed {hero['name']} to Smokey")
    page.keyboard.press("Escape")
    check(not state("s.vault.open"), "Esc closes the items screen")

    # the stack panel: the fortress stack holds the hero and the garrison
    page.mouse.click(600, 400)
    smoke.visible("#city.open", "the city screen")
    page.click("#selectGarrison")
    smoke.visible("#side input[data-active]", "the stack panel")
    page.click(f"#side button[data-details='{hero['unit']}']")
    smoke.visible("#side .unit-details", "the unit details")
    check("defense" in page.inner_text("#side .unit-details"), f"i shows {hero['name']}'s details")
    page.uncheck(f"#side input[data-active='{hero['unit']}']")
    active = state(f"s.view.stacks.flatMap(stack => stack.units).find(unit => unit.id === {hero['unit']}).active")
    check(active is False, f"unchecking {hero['name']} leaves the hero behind when the stack moves")
    smoke.shot("12-stack-panel")
    other = state(f"s.view.stacks.flatMap(stack => stack.units).find(unit => unit.id !== {hero['unit']}).id")
    page.click(f"#side button[data-disband='{other}']")
    smoke.visible("#modal.open", "the disband question")
    page.click("#modalChoices button:has-text('Disband')")
    gone = state(f"!s.view.stacks.flatMap(stack => stack.units).some(unit => unit.id === {other})")
    check(gone, "disbanded a unit of the garrison")
    smoke.no_errors()



def map_point(page, location):
    """A map tile's centre in page pixels, after centring the camera on it."""
    return page.evaluate(f"""(() => {{
        const s = window.civGame;
        const location = {json.dumps(location)};
        if (s.plane !== location.plane) {{ s.plane = location.plane; s.map.setData(s.core.map(s.plane), s.view); }}
        s.map.centerOn(location.x, location.y);
        return s.map.screenPoint(location.x, location.y);
    }})()""")


def orders(smoke, url):
    """The orders scenario: settlers where an outpost can go, engineers next to the capital, and
    swordsmen on an open tower."""
    page, state, check = smoke.page, smoke.state, smoke.check
    smoke.load(url + "?opponents=1&scenario=orders")

    def select(predicate, what):
        stack = state(f"s.view.stacks.find(stack => {predicate})")
        check(stack is not None, f"the scenario has {what}")
        page.mouse.click(*map_point(page, stack["location"]))
        smoke.visible("#side .orders", f"the orders of {what}")
        return stack

    settlers = select("stack.build === 'settle'", "settlers")
    check(page.locator("#side button[data-order='build']").inner_text() == "Settle (B)", "the stack panel offers Settle (B)")
    cities = state("s.view.cities.length")
    page.keyboard.press("b")
    smoke.wait(f"window.civGame.view.cities.length === {cities + 1}", "the new outpost")
    check("New outpost" in page.inner_text("#log"), "B founds an outpost and the log says so")

    engineers = select("stack.road", "engineers")
    page.click("#side button[data-order='road']")
    check(state("!!s.roadPick") and "destination" in page.inner_text("#side"), "Road asks for the destination")
    target = state(f"""(() => {{
        const from = {json.dumps(engineers['location'])};
        for (const [dx, dy] of [[2, 0], [-2, 0], [0, 2], [0, -2], [2, 2], [-2, -2], [2, -2], [-2, 2]]) {{
            const to = {{ plane: from.plane, x: (from.x + dx + s.map.mapView.width) % s.map.mapView.width, y: from.y + dy }};
            if (Array.isArray(s.core.roadPath(from, to))) return to;
        }}
        return null;
    }})()""")
    check(target is not None, f"a road destination near the engineers: {target}")
    point = page.evaluate(f"window.civGame.map.screenPoint({target['x']}, {target['y']})")
    page.mouse.move(*point)
    smoke.wait("window.civGame.map.previewPath && window.civGame.map.previewPath.length >= 2", "the road preview")
    smoke.shot("14-road-preview")
    page.mouse.click(*point)
    building = state("s.view.stacks.flatMap(stack => stack.units).some(unit => unit.busy === 'building road')")
    check(building and not state("s.roadPick") and not state("s.map.previewPath"), "clicking the destination starts the road")

    traveller = select("stack['planar-travel']", "a stack on a tower")
    start = traveller["location"]
    page.click("#side button[data-order='plane']")
    other = "Myrror" if start["plane"] == "Arcanus" else "Arcanus"
    moved = state(f"s.view.stacks.find(stack => stack.location.plane === '{other}' && stack.location.x === {start['x']} && stack.location.y === {start['y']})")
    check(moved is not None and state("s.plane") == other, f"Plane takes the stack to {other} and the map follows")

    page.keyboard.press("p")
    unit = state(f"s.view.stacks.find(stack => stack.location.plane === '{other}' && stack.location.x === {start['x']}).units[0]")
    check(unit["busy"] == "patrolling" and state("s.selectedUnit") is None, "P sets the stack on patrol")
    page.keyboard.press("n")
    selected = state("(() => { const stack = s.view.stacks.find(stack => stack.units.some(unit => unit.id === s.selectedUnit)); return stack ? stack.location : null; })()")
    check(selected is None or selected["plane"] != other or selected["x"] != start["x"], "Next unit skips the patrolling stack")
    smoke.shot("15-orders")

    lists(smoke)
    smoke.no_errors()


def lists(smoke):
    """The city and army lists (C, A): every city and stack, a click opens or selects it."""
    page, state, check = smoke.page, smoke.state, smoke.check
    page.keyboard.press("Escape")
    page.keyboard.press("c")
    smoke.visible("#lists.open", "the city list")
    cities = state("s.view.cities")
    check(page.locator("#lists tbody tr").count() == len(cities) >= 2, f"C lists the {len(cities)} cities")
    smoke.shot("17-city-list")
    page.click("#lists tbody tr >> nth=1")
    smoke.visible("#city.open", "the city screen from the list")
    check(not state("s.lists.open") and cities[1]["name"] in page.inner_text("#city h3"), f"clicking {cities[1]['name']} opens its city screen")
    page.keyboard.press("Escape")

    page.keyboard.press("a")
    smoke.visible("#lists.open", "the army list")
    stacks = state("s.view.stacks.filter(stack => stack.units.length)")
    check(page.locator("#lists tbody tr").count() == len(stacks) and "patrolling" in page.inner_text("#lists"), f"A lists the {len(stacks)} stacks, with their orders")
    page.keyboard.press("c")
    check(state("s.lists.tab") == "cities", "C switches to the cities tab")
    page.keyboard.press("a")
    last = stacks[-1]
    page.click(f"#lists tbody tr >> nth={len(stacks) - 1}")
    selected = state("(() => { const stack = s.view.stacks.find(stack => stack.units.some(unit => unit.id === s.selectedUnit)); return stack ? stack.location : null; })()")
    check(selected == last["location"] and state("s.plane") == last["location"]["plane"] and not state("s.lists.open"), f"clicking a stack selects it and shows {last['location']}")



def saves(smoke, url):
    """Save and load: the autosave of a new game, the save slot, a save file, Continue."""
    page, state, check = smoke.page, smoke.state, smoke.check
    smoke.load(url + "?opponents=1")
    smoke.wait("localStorage.getItem('civ-of-magic-save-autosave-info')", "the autosave of the new game")
    check(True, "a new game is autosaved")

    page.keyboard.press("g")
    smoke.visible("#game.open", "the game menu")
    page.click("#game button[data-save]")
    smoke.wait("document.querySelector('#game .message') && document.querySelector('#game .message').textContent.startsWith('Saved')", "the save")
    date = state("s.view.date")
    check("Load Saved game" in page.inner_text("#game"), f"G, Save keeps the game ({date})")
    with page.expect_download() as download:
        page.click("#game button[data-download]")
    path = download.value.path()
    with open(path) as file:
        saved = json.load(file)
    check(saved["version"] == 1 and download.value.suggested_filename.endswith(".json"), f"downloaded {download.value.suggested_filename}")
    page.keyboard.press("Escape")

    page.evaluate("window.civGame.execute('end-turn')")
    if page.locator("#modal.open").count():
        page.click("#modalChoices button >> nth=0")
    check(state("s.view.date") != date, f"played on to {state('s.view.date')}")
    page.keyboard.press("g")
    page.click("#game button[data-load='save']")
    smoke.wait(f"window.civGame.view.date === {json.dumps(date)} && !document.querySelector('#game.open')", "the loaded game")
    check("Loaded" in page.inner_text("#log"), f"Load Saved game goes back to {date}")

    page.goto(url)
    smoke.visible("#start button[data-continue='save']", "Continue on the start screen")
    check(page.locator("#start button[data-continue]").count() == 2, "the start screen offers the autosave and the saved game")
    smoke.shot("16-continue")
    page.click("#start button[data-continue='save']")
    smoke.wait("window.civGame && window.civGame.view", "the continued game", LOAD_TIMEOUT)
    check(state("s.view.date") == date, f"Continue loads the saved game ({date})")

    page.evaluate("window.civGame.execute('end-turn')")
    if page.locator("#modal.open").count():
        page.click("#modalChoices button >> nth=0")
    page.keyboard.press("g")
    with page.expect_file_chooser() as chooser:
        page.click("#game button[data-load-file]")
    chooser.value.set_files(path)
    smoke.wait(f"window.civGame.view.date === {json.dumps(date)}", "the game from the file")
    check(True, "Load a save file loads the downloaded save")
    smoke.no_errors()


if __name__ == "__main__":
    main()
