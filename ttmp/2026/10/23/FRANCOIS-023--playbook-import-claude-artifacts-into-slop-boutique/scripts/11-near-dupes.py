"""Find near-duplicate artifacts before preparing them.

usage: python3 -I 11-near-dupes.py <dir>   (each <dir>/<name>/index.html is one downloaded artifact)

Prints each page's <title> and size, then pairs whose sets of source lines
(20–300 chars) overlap by more than 15 % (Jaccard), and for those pairs the
lines one has that the other lacks. Bundled libraries (three.js) inflate the
overlap; the unique-lines count is what tells a copy from a sibling: on
2026-10-23 "Plant 7" had 1 line that "Lab 104" lacked (a copy), while "kiln
studio" shared 6 lines with "Kiln Workbench" (a different app).
"""
import itertools, pathlib, re, sys
docs = {}
for p in sorted(pathlib.Path(sys.argv[1]).glob('*/index.html')):
    t = p.read_text(errors='replace')
    title = (re.search(r'<title>([^<]*)', t) or [None, ''])[1]
    lines = [l.strip() for l in t.splitlines() if 20 < len(l.strip()) < 300]
    docs[p.parent.name] = lines
    print(f'{p.parent.name:32} {len(t)//1024:6} KB  {title!r}')
print()
for a, b in itertools.combinations(docs, 2):
    A, B = set(docs[a]), set(docs[b])
    j = len(A & B) / max(1, len(A | B))
    if j > 0.15:
        print(f'{j:.2f}  {a} ~ {b}: {len(A - B)} lines only in {a}, {len(B - A)} only in {b}')
        for l in [l for l in docs[a] if l not in B][:3]:
            print('      +', l[:120])
