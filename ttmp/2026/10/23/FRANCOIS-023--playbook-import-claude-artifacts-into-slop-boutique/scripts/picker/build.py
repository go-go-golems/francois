import json, sys, pathlib
d = pathlib.Path(sys.argv[1])
rows = []
for i in range(4):
    rows += json.loads((d / f"out0{i}.json").read_text())
for r in rows:
    note = r.get("note") or ""
    if "Docs" in note and r.get("files", 0) > 50:
        r["format"], r["size"], r["contents"], r["capabilities"], r["note"] = "docs", "", "", [], "Made from the Docs type: the text lives in the Docs store, so it exports as Markdown, not as an HTML app"
    elif "Design type" in note:
        r["format"] = "design"
    elif r["title"].startswith("[shared]"):
        r["title"], r["format"], r["shared"] = r["title"][9:], "shared", True
    if r.get("size") == "?":
        r["size"] = ""
# claude.ai URL -> slop.boutique slug, for apps already in the catalog (matched by title)
onsite = json.loads((d / "onsite.json").read_text())
html = (d / "template.html").read_text()
html = html.replace("/*DATA*/[]", json.dumps(rows, ensure_ascii=False).replace("</", "<\\/"))
html = html.replace("/*ONSITE*/{}", json.dumps(onsite))
(d / "import-picker.html").write_text(html)
print(len(rows), "rows")
