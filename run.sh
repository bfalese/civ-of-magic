#!/usr/bin/env bash
# Launch civ-of-magic.
#
#   ./run.sh               the web game: builds the wasm core, serves it, opens the browser
#   ./run.sh desktop       the Ebitengine desktop game (needs the original MoM data)
#   ./run.sh export        (re)export gamedata.json and the sprites the web game needs
#   ./run.sh smoke         run the browser smoke test against a running web game
#
# Paths (override with environment variables):
#   MOM_DATA     the original Master of Magic install (LBX files)
#   CIV_SHARE    where the exported data lives (gamedata/ and assets/)
#   PORT         the web game's port (default 8080; 8099 when 8080 is taken, e.g. by Open WebUI)
set -euo pipefail

cd "$(dirname "$0")"

MOM_DATA="${MOM_DATA:-$HOME/ROMs/ms-dos/Master of Magic}"
CIV_SHARE="${CIV_SHARE:-$HOME/.local/share/civ-of-magic}"

port_in_use() {
    ss -ltn 2>/dev/null | awk '{print $4}' | grep -qE "[:.]$1\$"
}

pick_port() {
    if [ -n "${PORT:-}" ]; then
        echo "$PORT"
    elif port_in_use 8080; then
        echo 8099
    else
        echo 8080
    fi
}

need_exports() {
    if [ ! -f "$CIV_SHARE/gamedata/gamedata.json" ] || [ ! -f "$CIV_SHARE/assets/assets.json" ]; then
        echo "No exported data in $CIV_SHARE: exporting it from $MOM_DATA first."
        export_data
    fi
}

export_data() {
    if [ ! -d "$MOM_DATA" ]; then
        echo "Master of Magic data not found at: $MOM_DATA (set MOM_DATA)" >&2
        exit 1
    fi
    go run ./util/export-gamedata -data "$MOM_DATA" -out "$CIV_SHARE/gamedata"
    go run ./util/export-assets -data "$MOM_DATA" -out "$CIV_SHARE/assets"
}

web() {
    need_exports
    local port
    port="$(pick_port)"
    if port_in_use "$port"; then
        echo "Port $port is already in use (an old serve-web serves a stale core: stop it first, or set PORT)." >&2
        exit 1
    fi
    local url="http://localhost:$port"
    echo "Starting the web game on $url (Ctrl+C to stop)…"
    # open the browser once the server answers
    (
        for _ in $(seq 1 120); do
            if curl -s -o /dev/null "$url"; then
                xdg-open "$url" >/dev/null 2>&1 || echo "Open $url in your browser."
                exit 0
            fi
            sleep 1
        done
    ) &
    exec go run ./util/serve-web -addr "localhost:$port" -gamedata "$CIV_SHARE/gamedata" -assets "$CIV_SHARE/assets"
}

desktop() {
    if [ ! -d "$MOM_DATA" ]; then
        echo "Master of Magic data not found at: $MOM_DATA (set MOM_DATA)" >&2
        exit 1
    fi
    exec go run ./game/magic -data "$MOM_DATA" "$@"
}

smoke() {
    local port
    port="${PORT:-$(port_in_use 8099 && echo 8099 || echo 8080)}"
    exec python3 frontend/web/smoke_test.py "http://localhost:$port"
}

case "${1:-web}" in
    web) web ;;
    desktop) shift; desktop "$@" ;;
    export) export_data ;;
    smoke) smoke ;;
    -h|--help|help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//' ;;
    *) echo "unknown mode: $1 (web, desktop, export, smoke)" >&2; exit 1 ;;
esac
