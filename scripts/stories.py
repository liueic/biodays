#!/usr/bin/env python3
"""Audit exports, synchronize stories by ID, generate the build manifest,
and publish stories to a running site via the admin API."""
import argparse
import datetime
import hashlib
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / "data"

BATCH_LIMIT = 200


def read_stories(path):
    rows = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(rows, list):
        raise ValueError(f"{path}: expected a JSON array")
    for row in rows:
        if not isinstance(row, dict):
            raise ValueError(f"{path}: expected story objects")
        for key in ("id", "title", "content", "geek_quote"):
            if not isinstance(row.get(key), str) or not row[key].strip():
                raise ValueError(f"{path}: missing {key}")
        if row["id"] != row["id"].strip() or len(row["id"].encode()) > 16:
            raise ValueError(f"{path}: invalid ID {row['id']!r}")
        for key in ("lab_item", "summary"):
            if key in row and not isinstance(row[key], str):
                raise ValueError(f"{path}: {row['id']}: {key} must be a string")
        if any(type(row.get(key)) is not int for key in ("year", "month", "day")):
            raise ValueError(f"{path}: {row['id']}: date fields must be integers")
        try:
            datetime.date(row["year"], row["month"], row["day"])
        except (KeyError, TypeError, ValueError) as error:
            raise ValueError(f"{path}: {row['id']}: invalid date") from error
        if not isinstance(row.get("tags", []), list) or any(not isinstance(t, str) for t in row.get("tags", [])):
            raise ValueError(f"{path}: {row['id']}: tags must be strings")
        source = row.get("source", {})
        if not isinstance(source, dict) or any(not isinstance(source.get(key, ""), str) for key in ("type", "citation", "url")):
            raise ValueError(f"{path}: {row['id']}: source must contain string fields")
    return rows


def inventory(paths, allow_identical=False):
    records, locations, counts, duplicates = {}, {}, {}, 0
    for path in paths:
        rows = read_stories(path)
        counts[str(path)] = len(rows)
        for row in rows:
            key = row["id"]
            if key in records:
                if not allow_identical or records[key] != row:
                    raise ValueError(f"conflicting/duplicate ID {key}: {locations[key]} and {path}; synchronize the authoritative file explicitly")
                duplicates += 1
            else:
                records[key], locations[key] = row, path
    return records, locations, counts, duplicates


def manifest():
    paths = sorted(DATA.glob("stories_*.json"))
    records, _, counts, _ = inventory(paths)
    if not records:
        raise ValueError("dataset is empty")
    digest = hashlib.sha256()
    for path in paths:
        digest.update(path.name.encode() + b"\0" + path.read_bytes() + b"\0")
    return {"version": "sha256:" + digest.hexdigest(), "total": len(records),
            "files": {Path(path).name: count for path, count in counts.items()}}


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def api_request(api, token, method, path, payload=None):
    url = api.rstrip("/") + path
    data = json.dumps(payload).encode() if payload is not None else None
    headers = {"Authorization": f"Bearer {token}", "Accept": "application/json"}
    if data is not None:
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read().decode()
            return json.loads(body) if body else {}
    except urllib.error.HTTPError as error:
        detail = error.read().decode(errors="replace")
        raise ValueError(f"{method} {url} failed: HTTP {error.code}: {detail[:500]}") from error
    except urllib.error.URLError as error:
        raise ValueError(f"{method} {url} failed: {error.reason}") from error


def collect_publish_entries(args):
    """Select the stories a publish run should push: explicit --file/--id
    selections, or the whole data/ corpus by default (or with --all)."""
    if args.file or args.id:
        records = {}
        for path in args.file or []:
            for row in read_stories(path):
                if row["id"] in records and records[row["id"]] != row:
                    raise ValueError(f"conflicting/duplicate ID {row['id']} in publish selection")
                records[row["id"]] = row
        if args.id:
            corpus, _, _, _ = inventory(sorted(DATA.glob("stories_*.json")))
            for identifier in args.id:
                if identifier not in corpus:
                    raise ValueError(f"ID {identifier!r} not found in {DATA}")
                records[identifier] = corpus[identifier]
        entries = [records[key] for key in sorted(records)]
    else:
        corpus, _, _, _ = inventory(sorted(DATA.glob("stories_*.json")))
        entries = [corpus[key] for key in sorted(corpus)]
    if not entries:
        raise ValueError("nothing to publish")
    return entries


def require_credentials(args):
    token = args.token or os.environ.get("BIODAYS_ADMIN_API_TOKEN", "")
    if not args.api or not token:
        raise ValueError("this command requires --api and --token (or the BIODAYS_ADMIN_API_TOKEN environment variable)")
    return token


def publish(args):
    token = require_credentials(args)
    entries = collect_publish_entries(args)
    accepted = rejected = 0
    for start in range(0, len(entries), BATCH_LIMIT):
        chunk = entries[start:start + BATCH_LIMIT]
        result = api_request(args.api, token, "POST", "/api/v1/admin/stories", {"stories": chunk})["data"]
        accepted += result["accepted"]
        rejected += result["rejected"]
        for item in result["items"]:
            if item.get("problems"):
                print(f"  rejected {item.get('id')}: {'; '.join(item['problems'])}")
    print(f"published: accepted={accepted} rejected={rejected} total={len(entries)}")
    if rejected:
        raise ValueError(f"{rejected} stories were rejected by the server")


def list_remote(args):
    token = require_credentials(args)
    query = f"/api/v1/admin/stories?page={args.page}&page_size=50"
    response = api_request(args.api, token, "GET", query)
    meta, rows = response["meta"], response["data"]
    for row in rows:
        print(f"{row['id']:<16} {row['year']:04d}-{row['month']:02d}-{row['day']:02d}  {row['title']}")
    print(f"page {meta['page']} of {max(1, -(-meta['total'] // meta['page_size']))}, {meta['total']} stories total")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["audit", "sync", "manifest", "check", "publish", "list"])
    parser.add_argument("source", nargs="?", type=Path)
    parser.add_argument("--api", help="站点地址，例如 https://biodays.example")
    parser.add_argument("--token", help="BIODAYS_ADMIN_API_TOKEN 的值（也可用环境变量提供）")
    parser.add_argument("--file", action="append", type=Path, help="要发布的 JSON 文件，可重复给出")
    parser.add_argument("--id", action="append", help="只发布 data/ 中指定 ID 的故事，可重复给出")
    parser.add_argument("--all", action="store_true", help="发布 data/ 中的全部故事（默认行为）")
    parser.add_argument("--page", type=int, default=1, help="list 命令的页码")
    args = parser.parse_args()
    if args.command == "publish":
        publish(args)
    elif args.command == "list":
        list_remote(args)
    elif args.command in ("audit", "sync"):
        source = (args.source or DATA).resolve()
        paths = [source] if source.is_file() else sorted(p for p in source.rglob("*.json") if p.name != "manifest.json")
        if not paths:
            raise ValueError(f"no JSON files in {source}")
        incoming, _, counts, duplicates = inventory(paths, allow_identical=True)
        current, locations, _, _ = inventory(sorted(DATA.glob("stories_*.json")))
        added = sorted(incoming.keys() - current.keys())
        changed = sorted(key for key in incoming.keys() & current.keys() if incoming[key] != current[key])
        print(json.dumps({"source_files": len(paths), "source_rows": sum(counts.values()), "unique_stories": len(incoming),
                          "identical_duplicates": duplicates, "repository_stories": len(current), "new_ids": added, "updated_ids": changed}, ensure_ascii=False, indent=2))
        if args.command == "sync":
            updates = {}
            for key in changed:
                path = locations[key]
                updates.setdefault(path, read_stories(path))
                updates[path] = [incoming[key] if row["id"] == key else row for row in updates[path]]
            if added:
                path = DATA / "stories_imported.json"
                updates.setdefault(path, read_stories(path) if path.exists() else [])
                updates[path].extend(incoming[key] for key in added)
            for path, rows in updates.items():
                write_json(path, rows)
            write_json(DATA / "manifest.json", manifest())
            print(f"Synchronized: added={len(added)} updated={len(changed)}; existing stories not present in the export are preserved.")
    else:
        actual = manifest()
        if args.command == "check":
            if json.loads((DATA / "manifest.json").read_text()) != actual:
                raise ValueError("manifest is stale; run python3 scripts/stories.py manifest")
        else:
            write_json(DATA / "manifest.json", actual)
        print(json.dumps(actual, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, json.JSONDecodeError) as error:
        sys.exit(str(error))
