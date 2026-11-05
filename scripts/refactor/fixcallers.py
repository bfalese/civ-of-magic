#!/usr/bin/env python3
"""usage: fixcallers.py OLDALIAS_REGEX NEWALIAS NEWIMPORT FUNC[,FUNC...] FILE...
Rewrite `<old>.FUNC` to `<new>.FUNC` and add NEWIMPORT (after the first module import) if missing."""
import re, sys
old, new, imp, funcs, files = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4].split(','), sys.argv[5:]
pat = re.compile(r'(?<![\w.])(%s)\.(%s)\b' % (old, '|'.join(funcs)))
for p in files:
    s = open(p).read()
    s2 = pat.sub(new + r'.\2', s)
    if s2 == s:
        continue
    line = '"%s"' % imp
    if line not in s2:
        m = re.search(r'^(\s*)(\w+ )?"github.com/bfalese/civ-of-magic/[^"]+"\n', s2, re.M)
        s2 = s2[:m.start()] + m.group(1) + line + '\n' + s2[m.start():]
    open(p, 'w').write(s2)
    print(p)
