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

        browser.close()

    if errors:
        print("page errors:\n  " + "\n  ".join(errors))
        sys.exit(1)
    print("smoke test passed")


if __name__ == "__main__":
    main()
