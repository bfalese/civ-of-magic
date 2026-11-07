"""Smoke test for the web frontend, in a real browser (Playwright, Chromium).

Plays the first turns through the page the way a person would: opens the capital's city screen,
changes production, moves a farmer, closes it, selects the garrison, moves it, ends the turn, and
answers the research question. Fails on any page error.

    pip install --user playwright && python3 -m playwright install chromium
    go run ./util/serve-web &           # needs the exported gamedata and assets
    python3 frontend/web/smoke_test.py [http://localhost:8080] [--shots DIR]
"""

import json
import sys

from playwright.sync_api import sync_playwright


def main():
    arguments = [argument for argument in sys.argv[1:] if not argument.startswith("--")]
    url = arguments[0] if arguments else "http://localhost:8080"
    shots = sys.argv[sys.argv.index("--shots") + 1] if "--shots" in sys.argv else None
    errors = []

    def shot(page, name):
        if shots:
            page.screenshot(path=f"{shots}/{name}.png")

    def check(condition, message):
        if not condition:
            raise AssertionError(message)
        print(f"ok  {message}")

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(args=["--use-angle=swiftshader", "--enable-unsafe-swiftshader"])
        page = browser.new_page(viewport={"width": 1200, "height": 800})
        page.on("pageerror", lambda error: errors.append(str(error)))
        page.on("console", lambda message: errors.append(message.text) if message.type == "error" else None)

        page.goto(url)
        page.wait_for_function("window.civGame && window.civGame.view", timeout=60000)
        state = lambda expression: page.evaluate(f"(() => {{ const s = window.civGame; return {expression}; }})()")
        check(state("s.view.cities.length") == 1, "the game starts with a capital")
        shot(page, "1-start")

        # the camera starts on the capital: its tile is in the middle of the map area
        page.mouse.click(600, 400)
        page.wait_for_selector("#city.open")
        check(state("s.cityOpen !== null"), "clicking the capital opens its city screen")
        shot(page, "2-city")

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
        shot(page, "3-city-changed")

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
        shot(page, "4-moved")

        date = state("s.view.date")
        page.keyboard.press("Enter")
        page.wait_for_function(f"window.civGame.view.date !== {json.dumps(date)}")
        check(True, f"end turn: {date} -> {state('s.view.date')}")
        page.wait_for_selector("#modal.open")
        page.click("#modalChoices button >> nth=0")
        check(state("s.view.you.researching") is not None, f"research chosen: {state('s.view.you.researching')}")
        shot(page, "5-next-turn")

        battle(page, state, check, shot)
        browser.close()

    if errors:
        print("page errors:\n  " + "\n  ".join(errors))
        sys.exit(1)
    print("smoke test passed")


# the nearest guarded site (lair, ruins, ...) to the first stack, in the explored map
FIND_ENCOUNTER = """(() => {
    const s = window.civGame;
    const stack = s.view.stacks.find((stack) => stack.units.length);
    if (!stack) return null;
    const map = s.core.map(stack.location.plane);
    let best = null;
    map.tiles.forEach((row, y) => row.forEach((tile, x) => {
        if (!tile || !tile.encounter || tile.encounter.includes("Node")) return;
        const dx = Math.min(Math.abs(x - stack.location.x), map.width - Math.abs(x - stack.location.x));
        const distance = Math.max(dx, Math.abs(y - stack.location.y));
        if (!best || distance < best.distance) best = { x, y, distance, kind: tile.encounter };
    }));
    return best && { to: { plane: stack.location.plane, x: best.x, y: best.y }, distance: best.distance, kind: best.kind };
})()"""


def battle(page, state, check, shot):
    """March to a guarded site near a fresh capital, enter, fight one move by hand, then auto."""
    url = page.url.split("?")[0]
    for _ in range(10):
        page.goto(url + "?opponents=1")
        page.wait_for_function("window.civGame && window.civGame.view", timeout=60000)
        target = page.evaluate(FIND_ENCOUNTER)
        if target and target["distance"] <= 4:
            break
    else:
        print("skip: no guarded site near the capital in 10 games")
        return

    for _ in range(12):
        status = page.evaluate(f"""(() => {{
            const s = window.civGame;
            const stack = s.view.stacks.find((stack) => stack.units.length);
            return s.execute("move-stack", {{ from: stack.location, to: {json.dumps(target["to"])} }});
        }})()""")
        if status.get("waiting") == "confirm-encounter":
            break
        page.evaluate("window.civGame.execute('end-turn')")
        while page.locator("#modal.open").count():
            page.click("#modalChoices button >> nth=0")
    page.wait_for_selector("#modal.open")
    page.click("#modalChoices button:has-text('Enter')")
    page.wait_for_selector("#battle.open")
    check(True, f"entered the {target['kind']}: the battle screen is open")
    shot(page, "6-battle")

    view = page.evaluate("window.civGame.core.battle()")
    check(len(view["moves"]) > 0 and view.get("selected") is not None, f"one of your units is up with {len(view['moves'])} tiles to move to")

    # click the first reachable tile on the canvas
    x, y = view["moves"][0]
    point = page.evaluate(f"""(() => {{
        const battle = window.civGame.battle;
        const [cx, cy] = battle.center({x}, {y});
        const bounds = battle.canvas.getBoundingClientRect();
        return [bounds.left + cx * bounds.width / battle.canvas.width, bounds.top + cy * bounds.height / battle.canvas.height];
    }})()""")
    selected = view["selected"]
    page.mouse.click(point[0], point[1])
    after = page.evaluate("window.civGame.core.battle()")
    moved = next((unit for unit in after.get("units", []) if unit["id"] == selected), None)
    check(moved is not None and (moved["x"], moved["y"]) == (x, y), f"clicked tile {x},{y}: the unit walked there")
    shot(page, "7-battle-moved")

    page.click("#battle button[data-command='battle-auto']")
    page.wait_for_function("!window.civGame.battle.open", timeout=60000)
    check(True, "auto finished the battle and the screen closed")
    log = page.inner_text("#log")
    check("the battle at" in log, f"the log reports it: {next((line for line in log.splitlines() if 'battle at' in line), '')}")


if __name__ == "__main__":
    main()
