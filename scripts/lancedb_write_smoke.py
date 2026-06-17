#!/usr/bin/env python3
"""Controlled LanceDB write-smoke helper for DeonClaw.

Reads a JSON write request from stdin and creates/overwrites one LanceDB table.
Does not perform retrieval/search. Stdout returns JSON only; no vector payloads.
"""

from __future__ import annotations

import json
import sys
from typing import Any


def fail(message: str, code: int = 1) -> None:
    print(json.dumps({"status": "failed", "message": message}), file=sys.stdout)
    sys.exit(code)


def main() -> None:
    try:
        request = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        fail(f"invalid stdin json: {exc}")

    try:
        import lancedb
    except ImportError:
        fail("lancedb Python package is not installed; run: pip install lancedb pyarrow")

    database_path = request.get("database_path")
    table_name = request.get("table")
    vector_column = request.get("vector_column", "vector")
    text_ref_column = request.get("text_ref_column", "chunk_id")
    metadata_columns = request.get("metadata_columns") or []
    rows = request.get("rows") or []

    if not database_path or not table_name:
        fail("database_path and table are required")
    if not isinstance(rows, list):
        fail("rows must be a list")

    records: list[dict[str, Any]] = []
    for row in rows:
        if not isinstance(row, dict):
            fail("each row must be an object")
        vector = row.get("vector")
        if vector is None:
            fail("each row requires a vector")
        record = {
            vector_column: vector,
            text_ref_column: row.get("chunk_id") or row.get("vector_id"),
            "vector_id": row.get("vector_id"),
        }
        for column in metadata_columns:
            if column in row:
                record[column] = row[column]
        records.append(record)

    db = lancedb.connect(database_path)
    db.create_table(table_name, records, mode="overwrite")
    print(json.dumps({"status": "ok", "row_count": len(records)}))


if __name__ == "__main__":
    main()
