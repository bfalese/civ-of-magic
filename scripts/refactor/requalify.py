#!/usr/bin/env python3
"""After `git mv old/x.go new/x.go`: set the package clause, qualify identifiers exported by
the old package, and add its import.

usage: requalify.py OLD_PKG_DIR OLD_IMPORT_PATH ALIAS NEW_FILE...
"""
import os, re, sys
here = os.path.dirname(os.path.abspath(__file__))
src = open(os.path.join(here, 'movefuncs.py')).read().split('def main():')[0]
exec(src)

old_dir, old_import, alias, files = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
names = exported_idents(old_dir)
for p in files:
    s = open(p).read()
    new_pkg = os.path.basename(os.path.dirname(os.path.abspath(p)))
    s = re.sub(r'^package \w+', 'package ' + new_pkg, s, count=1, flags=re.M)
    start = s.index('import (')
    head_end = s.index('\n)\n', start) + 3
    head, body = s[:head_end], s[head_end:]
    line = '    "%s"\n' % old_import if alias == os.path.basename(old_import) else '    %s "%s"\n' % (alias, old_import)
    head = head[:head_end - 3] + '\n' + line + head[head_end - 3:]
    head = head.replace('\n\n' + line, '\n' + line)
    open(p, 'w').write(head + qualify(body, names, alias))
    print(p)
