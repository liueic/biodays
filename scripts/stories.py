#!/usr/bin/env python3
"""Audit exports, synchronize stories by ID, and generate the build manifest."""
import argparse
import datetime
import hashlib
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / "data"


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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["audit", "sync", "manifest", "check"])
    parser.add_argument("source", nargs="?", type=Path)
    args = parser.parse_args()
    if args.command in ("audit", "sync"):
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
