#!/usr/bin/env python3
"""Tidy import blocks after automated rewrites (formatting only, never adds or removes imports).

- moves this module's imports that ended up among third-party imports back into the module group
- collapses runs of blank lines and drops leading/trailing blank lines inside the block

usage: tidy-imports.py FILE...      e.g. tidy-imports.py $(git diff --name-only master -- '*.go')
"""
import re, sys

MODULE = 'github.com/bfalese/civ-of-magic/'

def is_module(line):
    return MODULE in line and not line.strip().startswith('//')

def is_third_party(line):
    m = re.match(r'^\s*(?:\w+ |_ |\. )?"([^"]+)"', line)
    return m is not None and '.' in m.group(1).split('/')[0] and MODULE not in m.group(1)

def tidy(text):
    if 'import (' not in text:
        return text
    start = text.index('import (')
    end = text.index('\n)', start)
    lines = text[start:end].split('\n')[1:]

    first_third = next((i for i, l in enumerate(lines) if is_third_party(l)), None)
    if first_third is not None:
        stray = [l for l in lines[first_third:] if is_module(l)]
        if stray:
            lines = lines[:first_third] + [l for l in lines[first_third:] if not is_module(l)]
            last_module = max((i for i, l in enumerate(lines) if is_module(l)), default=None)
            if last_module is None:
                # no module group yet: put them in their own group before third-party imports
                at = next(i for i, l in enumerate(lines) if is_third_party(l))
                lines = lines[:at] + stray + [''] + lines[at:]
            else:
                lines = lines[:last_module + 1] + stray + lines[last_module + 1:]

    out = []
    for line in lines:
        if line.strip() == '' and (not out or out[-1].strip() == ''):
            continue
        out.append(line)
    while out and out[-1].strip() == '':
        out.pop()
    return text[:start] + 'import (\n' + '\n'.join(out) + text[end:]

changed = 0
for path in sys.argv[1:]:
    try:
        text = open(path).read()
    except FileNotFoundError:
        continue
    new = tidy(text)
    if new != text:
        open(path, 'w').write(new)
        changed += 1
print('%d files tidied' % changed)
