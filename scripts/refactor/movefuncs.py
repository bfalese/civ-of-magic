#!/usr/bin/env python3
"""Move top-level Go declarations from one file to another (possibly another package).

usage: movefuncs.py SRC DST ALIAS NAME [NAME...]
  SRC    source .go file
  DST    destination .go file (appended to; created with `package <dir name>` if missing)
  ALIAS  qualifier for the source package's exported identifiers in moved code, or '-' for same package
  NAME   func / method / type / const / var names to move (methods: Type.Method)

Prints nothing on success. Imports are not fixed; let the compiler tell you.
"""
import os, re, sys

def find_decl(lines, name):
    if '.' in name:
        typ, meth = name.split('.')
        pat = re.compile(r'^func \(\w+ \*?%s\) %s\(' % (typ, meth))
    else:
        pat = re.compile(r'^(func %s[\[(]|type %s\b|const %s\b|var %s\b)' % ((re.escape(name),)*4))
    starts = [i for i, l in enumerate(lines) if pat.match(l)]
    if len(starts) != 1:
        sys.exit('%s: found %d declarations' % (name, len(starts)))
    start = starts[0]
    first = lines[start]
    if first.rstrip().endswith('{') or first.rstrip().endswith('('):
        end = next(i for i in range(start + 1, len(lines)) if lines[i] in ('}', ')'))
    else:
        end = start
    s = start
    while s > 0 and lines[s - 1].startswith('//'):
        s -= 1
    return s, end

def exported_idents(pkg_dir):
    names = set()
    for f in os.listdir(pkg_dir):
        if not f.endswith('.go') or f.endswith('_test.go'):
            continue
        text = open(os.path.join(pkg_dir, f)).read()
        names |= set(re.findall(r'^(?:func|type|const|var) ([A-Z]\w*)', text, re.M))
        # names inside const ( ... ) / var ( ... ) blocks
        for block in re.findall(r'^(?:const|var) \((.*?)^\)', text, re.M | re.S):
            names |= set(re.findall(r'^\s+([A-Z]\w*)\b', block, re.M))
    return names

def qualify(text, names, alias):
    out = []
    # split on string/rune literals and comments so we leave them alone
    # (rune literals stay on one line; /* */ blocks count as comments)
    tokens = re.split(r'("(?:\\.|[^"\\\n])*"|`[^`]*`|\'(?:\\.|[^\'\\\n])*\'|//[^\n]*|/\*.*?\*/)', text, flags=re.S)
    for k, tok in enumerate(tokens):
        if k % 2 == 1:
            out.append(tok)
            continue
        def repl(m):
            word = m.group(1)
            if word not in names:
                return word
            after = tok[m.end():m.end() + 3]
            # composite-literal field key (Field: value), but not :=
            line_start = tok.rfind('\n', 0, m.start()) + 1
            before = tok[line_start:m.start()]
            if re.match(r'\s*:(?!=)', after) and not re.search(r'\bcase\b[^{]*$', before):
                return word
            # struct field declaration: line starts with the name, followed by a type
            if before.strip() == '' and re.match(r'[ \t]+[\w*\[\]]', tok[m.end():m.end() + 2]):
                return word
            return alias + '.' + word
        out.append(re.sub(r'(?<![\w.])([A-Z]\w*)\b', repl, tok))
    return ''.join(out)

def main():
    src, dst, alias, names = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
    lines = open(src).read().split('\n')
    chunks = []
    for name in names:
        s, e = find_decl(lines, name)
        chunks.append('\n'.join(lines[s:e + 1]))
        del lines[s:e + 1]
        while s < len(lines) and s > 0 and lines[s] == '' and lines[s - 1] == '':
            del lines[s]
    open(src, 'w').write('\n'.join(lines))

    text = '\n\n'.join(chunks)
    if alias != '-':
        text = qualify(text, exported_idents(os.path.dirname(src) or '.'), alias)

    if os.path.exists(dst):
        existing = open(dst).read().rstrip('\n')
        open(dst, 'w').write(existing + '\n\n' + text + '\n')
    else:
        pkg = os.path.basename(os.path.dirname(os.path.abspath(dst)))
        open(dst, 'w').write('package %s\n\nimport (\n)\n\n%s\n' % (pkg, text))

main()
