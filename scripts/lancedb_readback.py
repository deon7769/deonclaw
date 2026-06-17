#!/usr/bin/env python3
"""Structural LanceDB readback helper for DeonClaw.

Opens a local LanceDB database/table and returns metadata only.
Does not perform search/retrieval. Never returns full vectors or chunk text.
"""

from __future__ import annotations

import json
import os
import sys
from typing import Any


def ok(payload: dict[str, Any]) -> None:
    payload["status"] = "ok"
    print(json.dumps(payload))
    sys.exit(0)


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

    if not database_path or not table_name:
        fail("database_path and table are required")

    if not os.path.exists(database_path):
        ok(
            {
                "table_exists": False,
                "row_count": 0,
                "columns": [],
                "vector_column_exists": False,
                "text_ref_column_exists": False,
                "metadata_columns_present": [],
                "inferred_dimensions": 0,
                "sample_chunk_ids": [],
            }
        )

    db = lancedb.connect(database_path)
    table_names = db.table_names()
    table_exists = table_name in table_names
    columns: list[str] = []
    row_count = 0
    vector_column_exists = False
    text_ref_column_exists = False
    metadata_present: list[str] = []
    inferred_dimensions = 0
    sample_chunk_ids: list[str] = []

    if table_exists:
        table = db.open_table(table_name)
        schema = table.schema
        columns = [field.name for field in schema]
        row_count = int(table.count_rows())
        vector_column_exists = vector_column in columns
        text_ref_column_exists = text_ref_column in columns
        metadata_present = [name for name in metadata_columns if name in columns]

        if row_count > 0 and vector_column_exists:
            sample_rows = table.head(1).to_pylist()
            if sample_rows:
                vector_value = sample_rows[0].get(vector_column)
                if isinstance(vector_value, list):
                    inferred_dimensions = len(vector_value)

        if row_count > 0 and text_ref_column_exists:
            head_rows = table.head(5).to_pylist()
            for row in head_rows:
                chunk_id = row.get(text_ref_column)
                if chunk_id is None:
                    continue
                chunk_id_text = str(chunk_id)
                if chunk_id_text and chunk_id_text not in sample_chunk_ids:
                    sample_chunk_ids.append(chunk_id_text)
                if len(sample_chunk_ids) >= 5:
                    break

    ok(
        {
            "table_exists": table_exists,
            "row_count": row_count,
            "columns": columns,
            "vector_column_exists": vector_column_exists,
            "text_ref_column_exists": text_ref_column_exists,
            "metadata_columns_present": metadata_present,
            "inferred_dimensions": inferred_dimensions,
            "sample_chunk_ids": sample_chunk_ids,
        }
    )


if __name__ == "__main__":
    main()
