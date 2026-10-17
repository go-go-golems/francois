#!/usr/bin/env python3
"""Read-only production smoke checks; no sessions, emails or model calls."""
import datetime
import json
from html.parser import HTMLParser
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import urlopen
from urllib.parse import urlparse, parse_qs

class Links(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []
    def handle_starttag(self, tag, attrs):
        if tag == "a":
            self.links += [v for k, v in attrs if k == "href"]

def get(base, path, expected=200):
    try:
        response = urlopen(base + path, timeout=30)
    except HTTPError as response:
        assert response.code == expected, (path, response.code)
        return b""
    assert response.status == expected, (path, response.status)
    return response.read()

checks = []
for base in ["https://slop.boutique", "https://francois.yolo.scapegoat.dev"]:
    get(base, "/healthz")
    get(base, "/readyz")
    home = get(base, "/").decode()
    assert 'site-statement' in home
    assert 'href="/journal"' not in home
    session = json.loads(get(base, "/api/session"))
    assert session["journalEnabled"] is False
    assert session["authenticated"] is False
    get(base, "/journal", 404)
    get(base, "/journal/feed.xml", 404)
    feed = get(base, "/feed.xml").decode()
    assert "<rss" in feed and "/journal/" not in feed
    parser = Links()
    parser.feed(home)
    artifacts = sorted(set(p for p in parser.links if p.startswith("/apps/")))
    assert artifacts, "no published artifact links"
    downloads = []
    for path in artifacts:
        detail = get(base, path).decode()
        detail_links = Links()
        detail_links.feed(detail)
        for download in detail_links.links:
            if download.startswith("/api/releases/") and download.endswith("/download"):
                assert b"<html" in get(base, download).lower()
                downloads.append(download)
    for destination in ["/studio", "/admin"]:
        signin = get(base, "/signin?return=" + destination).decode()
        signin_links = Links()
        signin_links.feed(signin)
        staff = [p for p in signin_links.links if p.startswith("/auth/login?")]
        assert len(staff) == 1
        assert parse_qs(urlparse(staff[0]).query)["return"] == [destination]
    checks.append({"base": base, "ready": True, "journalEnabled": False,
                   "artifactPages": artifacts, "artifactRSS": True,
                   "publicHTMLDownloads": len(downloads), "staffReturnPaths": True})
result = {"at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
          "checks": checks, "mutations": False}
out = Path(__file__).resolve().parent.parent / "sources/production-validation.json"
out.write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps(result, indent=2))
