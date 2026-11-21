"""Read-only PDS fixture and HTTP checks for the isolated RC network."""

import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import URLError
from urllib.parse import parse_qs, urlsplit
from urllib.request import urlopen

DID = "did:plc:fixture"
RECORDS = {
    "site.standard.document": {
        "site": f"at://{DID}/site.standard.publication/local",
        "path": "/articles/sample",
        "title": "RC fixture Article",
    },
    "app.bsky.feed.post": {"text": "RC fixture Note"},
}


class PDS(BaseHTTPRequestHandler):
    def do_GET(self):
        path = urlsplit(self.path)
        query = parse_qs(path.query)
        collection = query.get("collection", [None])[0]
        if path.path == "/healthz":
            self.send_response(200)
            self.end_headers()
        elif (path.path == "/xrpc/com.atproto.repo.listRecords"
              and query.get("repo") == [DID] and collection in RECORDS):
            self.send_response(200)
            self.end_headers()
            # Matching an Article key identifies an Announcement, not a Note.
            key = "article" if collection == "site.standard.document" else "note"
            self.wfile.write(json.dumps({"records": [{
                "uri": f"at://{DID}/{collection}/{key}",
                "cid": "fixture-cid",
                "value": RECORDS[collection],
            }]}).encode())
        else:
            self.send_error(404)


def check_reads():
    for path, marker in [("/articles", "RC fixture Article"), ("/notes", "RC fixture Note")]:
        # Startup includes a backfill attempt, even when the PDS is offline.
        for attempt in range(30):
            try:
                with urlopen("http://site:8080" + path, timeout=3) as response:
                    if response.status != 200 or marker not in response.read().decode():
                        raise ValueError(f"{path} missing {marker!r}")
                print(f"{path}: 200, found {marker!r}")
                break
            except (URLError, TimeoutError):
                if attempt == 29:
                    raise
                time.sleep(1)


if __name__ == "__main__":
    if sys.argv[1:] == ["check"]:
        check_reads()
    else:
        ThreadingHTTPServer(("0.0.0.0", 8000), PDS).serve_forever()
