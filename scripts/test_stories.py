import contextlib
import os
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("stories", Path(__file__).with_name("stories.py"))
stories = importlib.util.module_from_spec(spec)
spec.loader.exec_module(stories)


def entry(identifier, title="A story"):
    return dict(id=identifier, title=title, year=2000, month=1, day=1,
                content="Content", geek_quote="Quote", tags=["Science"])


class StorySyncTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.data = self.root / "data"
        self.data.mkdir()
        self.source = self.root / "export"
        self.source.mkdir()
        self.patch = patch.object(stories, "DATA", self.data)
        self.patch.start()
        self.addCleanup(self.patch.stop)
        stories.write_json(self.data / "stories_test.json", [entry("old"), entry("keep")])

    def command(self, command):
        with patch("sys.argv", ["stories.py", command, str(self.source)]), contextlib.redirect_stdout(io.StringIO()):
            stories.main()

    def test_updates_additions_deduplication_and_idempotency(self):
        incoming = [entry("old", "Updated"), entry("new")]
        stories.write_json(self.source / "all.json", incoming)
        stories.write_json(self.source / "duplicate.json", incoming)
        self.command("sync")
        records, _, _, _ = stories.inventory(sorted(self.data.glob("stories_*.json")))
        self.assertEqual(set(records), {"old", "keep", "new"})
        self.assertEqual(records["old"]["title"], "Updated")
        first_manifest = stories.manifest()
        self.command("sync")
        self.assertEqual(first_manifest, stories.manifest())
        self.assertEqual(json.loads((self.data / "manifest.json").read_text()), first_manifest)
        self.command("check")

    def test_conflicting_exports_do_not_modify_repository(self):
        before = (self.data / "stories_test.json").read_bytes()
        stories.write_json(self.source / "one.json", [entry("old", "First")])
        stories.write_json(self.source / "two.json", [entry("old", "Conflicting")])
        with self.assertRaisesRegex(ValueError, "conflicting/duplicate ID"):
            self.command("sync")
        self.assertEqual(before, (self.data / "stories_test.json").read_bytes())

    def test_stale_manifest_and_bad_date_fail(self):
        stories.write_json(self.data / "manifest.json", stories.manifest())
        stories.write_json(self.data / "stories_test.json", [entry("different")])
        with self.assertRaisesRegex(ValueError, "manifest is stale"):
            self.command("check")
        invalid = entry("invalid")
        invalid.update(month=2, day=30)
        stories.write_json(self.source / "invalid.json", [invalid])
        with self.assertRaisesRegex(ValueError, "invalid date"):
            self.command("sync")


class PublishSelectionTests(unittest.TestCase):
    """collect_publish_entries picks stories without touching the network."""

    class Args:
        def __init__(self, file=None, id=None, all=False):
            self.file = file
            self.id = id
            self.all = all

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.data = Path(self.temp.name)
        self.patch = patch.object(stories, "DATA", self.data)
        self.patch.start()
        self.addCleanup(self.patch.stop)
        stories.write_json(self.data / "stories_test.json", [entry("a"), entry("b")])

    def test_default_publishes_whole_corpus(self):
        entries = stories.collect_publish_entries(self.Args())
        self.assertEqual([row["id"] for row in entries], ["a", "b"])

    def test_file_and_id_selection(self):
        export = self.data.parent / "export.json"
        stories.write_json(export, [entry("c")])
        args = self.Args(file=[export], id=["a"])
        self.assertEqual([row["id"] for row in stories.collect_publish_entries(args)], ["a", "c"])

    def test_unknown_id_fails(self):
        with self.assertRaisesRegex(ValueError, "not found"):
            stories.collect_publish_entries(self.Args(id=["missing"]))

    def test_credentials_required(self):
        with patch.dict("os.environ", {}, clear=False):
            os.environ.pop("BIODAYS_ADMIN_API_TOKEN", None)
            args = self.Args()
            args.api = ""
            args.token = ""
            with self.assertRaisesRegex(ValueError, "--api and --token"):
                stories.require_credentials(args)


if __name__ == "__main__":
    unittest.main()
