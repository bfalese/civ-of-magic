#!/usr/bin/env python3
"""Move methods of a UI type (e.g. Game) onto the model type it wraps (e.g. GameModel) in another package.

usage: promote-methods.py SRC_FILE DST_FILE UI_PKG_DIR MODEL_ALIAS NAME [NAME...]

  SRC_FILE     file in the UI package holding the methods/functions (e.g. game/magic/game/game.go)
  DST_FILE     file in the model package to append to (created if missing)
  UI_PKG_DIR   the UI package directory (all its .go files get their callers updated)
  MODEL_ALIAS  how the UI package imports the model package (e.g. gamecore)
  NAME         methods of `game *Game` or free functions to move

For each moved method `func (game *Game) foo(...)`:
  - it becomes `func (model *GameModel) Foo(...)` (exported, so the UI can call it)
  - inside moved code: `game.Model.` -> `model.`, `game.<moved or model method>(` -> `model.<...>(`
  - exported originals keep a one-line delegating wrapper on Game (callers and interfaces keep
    working); unexported ones have their UI callers rewritten to `game.Model.Foo(`
Free functions move as-is (exported if needed) and UI callers become `MODEL_ALIAS.Foo(`.

Model methods called through `game.X(` inside moved code must already exist on the model, or be in
NAME. Anything else left as `game.` aborts. Imports are not fixed: use prune-imports.sh after adding
what the compiler asks for.
"""
import glob, os, re, sys

UI_RECV = 'game'
UI_TYPE = 'Game'
MODEL_RECV = 'model'
MODEL_TYPE = 'GameModel'

def export(name):
    return name[0].upper() + name[1:]

def find_func(src, name):
    m = re.search(r'^(//[^\n]*\n)*func (\(%s \*%s\) )?%s\(.*?^\}\n' % (UI_RECV, UI_TYPE, re.escape(name)), src, re.M | re.S)
    if not m:
        sys.exit('not found: %s' % name)
    return m

def params_to_args(params):
    # "a int, b, c string, rest ...T" -> "a, b, c, rest..."
    args = []
    depth = 0
    current = ''
    parts = []
    for ch in params:
        if ch in '([{': depth += 1
        if ch in ')]}': depth -= 1
        if ch == ',' and depth == 0:
            parts.append(current); current = ''
        else:
            current += ch
    if current.strip():
        parts.append(current)
    for part in parts:
        part = part.strip()
        name = part.split()[0]
        if '...' in part:
            name += '...'
        args.append(name)
    return ', '.join(args)

def main():
    src_file, dst_file, ui_dir, alias = sys.argv[1:5]
    names = sys.argv[5:]
    src = open(src_file).read()

    model_src = ''.join(open(f).read() for f in glob.glob(os.path.join(os.path.dirname(dst_file), '*.go')) if not f.endswith('_test.go'))
    model_methods = set(re.findall(r'^func \(\w+ \*%s\) (\w+)\(' % MODEL_TYPE, model_src, re.M))

    found = {}
    for name in names:
        m = find_func(src, name)
        found[name] = (m.group(0), m.group(2) is not None)

    moved_methods = {n for n, (_, is_method) in found.items() if is_method}
    callable_on_model = model_methods | {export(n) for n in moved_methods}

    out = []
    wrappers = {}
    for name in names:
        text, is_method = found[name]
        new_name = export(name)
        if is_method:
            m = re.search(r'^func \(%s \*%s\) %s\((.*?)\)(.*?)\{\n' % (UI_RECV, UI_TYPE, name), text, re.M | re.S)
            params, results = m.group(1), m.group(2).strip()
            text = text.replace(m.group(0), 'func (%s *%s) %s(%s) %s{\n' % (MODEL_RECV, MODEL_TYPE, new_name, params, (results + ' ') if results else ''), 1)
            text = text.replace('%s.Model.' % UI_RECV, '%s.' % MODEL_RECV)
            text = re.sub(r'\b%s\.Model\b' % UI_RECV, MODEL_RECV, text)
            def call(mm):
                target = mm.group(1)
                if target in moved_methods:
                    return '%s.%s(' % (MODEL_RECV, export(target))
                if target in callable_on_model:
                    return '%s.%s(' % (MODEL_RECV, target)
                return mm.group(0)
            text = re.sub(r'\b%s\.(\w+)\(' % UI_RECV, call, text)
            if name[0].isupper():
                ret = 'return ' if results else ''
                wrappers[name] = 'func (%s *%s) %s(%s) %s{\n    %s%s.Model.%s(%s)\n}\n' % (
                    UI_RECV, UI_TYPE, name, params, (results + ' ') if results else '', ret, UI_RECV, new_name, params_to_args(params))
        else:
            text = re.sub(r'^func %s\(' % name, 'func %s(' % new_name, text, flags=re.M)
        for other in names:
            if not found[other][1] and other != export(other):
                text = re.sub(r'(?<![\w.])%s\(' % other, export(other) + '(', text)
        leftover = re.findall(r'\b%s\.\w+' % UI_RECV, text)
        if leftover:
            sys.exit('%s still uses %s' % (name, sorted(set(leftover))))
        out.append(text)
        src = src.replace(found[name][0], wrappers.get(name, ''), 1)

    open(src_file, 'w').write(src)

    body = '\n'.join(out)
    if os.path.exists(dst_file):
        existing = open(dst_file).read().rstrip('\n')
        open(dst_file, 'w').write(existing + '\n\n' + body)
    else:
        pkg = os.path.basename(os.path.dirname(os.path.abspath(dst_file)))
        open(dst_file, 'w').write('package %s\n\nimport (\n)\n\n%s' % (pkg, body))

    # update callers in the UI package
    for path in glob.glob(os.path.join(ui_dir, '*.go')):
        text = open(path).read()
        new = text
        for name in names:
            is_method = found[name][1]
            if is_method and not name[0].isupper():
                new = re.sub(r'\b(\w+)\.%s\(' % name, lambda mm: '%s.Model.%s(' % (mm.group(1), export(name)), new)
            if not is_method:
                new = re.sub(r'(?<![\w.])%s\(' % name, '%s.%s(' % (alias, export(name)), new)
        if new != text:
            open(path, 'w').write(new)
            print('updated', path)

main()
