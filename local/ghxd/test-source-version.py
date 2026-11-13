#!/usr/bin/env python3
import importlib.util
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("smoke_source_version", ROOT / "validate-source.py")
source_version = importlib.util.module_from_spec(spec)
spec.loader.exec_module(source_version)

class DatedSourceSelection(unittest.TestCase):
    def fixture(self):
        return {"schema": 1, "sources": {
            "sample": {
                "repository": "xd-dash/sample",
                "revision": "a" * 40,
                "checkout": "sample20261112",
            }
        }}

    def validate(self, value, allowed):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "manifest.json"
            path.write_text(json.dumps(value))
            if allowed:
                source_version.validate(path)
            else:
                with self.assertRaises(ValueError):
                    source_version.validate(path)

    def test_versioned_source(self):
        self.validate(self.fixture(), True)

    def test_feature_named_checkout_rejected(self):
        item = self.fixture()
        item["sources"]["sample"]["checkout"] = "sample-probot"
        self.validate(item, False)

    def test_invalid_date_rejected(self):
        item = self.fixture()
        item["sources"]["sample"]["checkout"] = "sample20260230"
        self.validate(item, False)

    def test_unpinned_commit_rejected(self):
        item = self.fixture()
        item["sources"]["sample"]["revision"] = "v20261112"
        self.validate(item, False)

    def test_mixed_date_rejected(self):
        item = self.fixture()
        item["sources"]["second"] = {
            "repository": "xd-dash/second",
            "revision": "b" * 40,
            "checkout": "second20261113",
        }
        self.validate(item, False)

    def test_stdin_contract(self):
        item = self.fixture()
        prior = sys.stdin
        try:
            sys.stdin = io.StringIO(json.dumps(item))
            source_version.validate("-")
        finally:
            sys.stdin = prior

if __name__ == "__main__":
    unittest.main()
