#!/usr/bin/env python3
"""Controlled LanceDB vector search smoke helper for DeonClaw.

Performs top-k vector search using an explicit query vector or an existing chunk row.
Does not perform natural-language retrieval. Never returns full vectors or chunk text.
"""

from __future__ import annotations

import json
import os
import sys
from typing import Any


def fail(message: str, code: int = 1) -> None:
    print(json.dumps({"status": "failed", "message": message}), file=sys.stdout)
    sys.exit(code)


def ok(payload: dict[str, Any]) -> None:
    payload["status"] = "ok"
    print(json.dumps(payload))
    sys.exit(0)


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
    top_k = int(request.get("top_k") or 0)
    query_vector = request.get("query_vector")
    query_chunk_id = request.get("query_chunk_id")

    if not database_path or not table_name:
        fail("database_path and table are required")
    if top_k <= 0:
        fail("top_k must be > 0")
    if not os.path.exists(database_path):
        fail(f"database path does not exist: {database_path}")

    has_vector = isinstance(query_vector, list) and len(query_vector) > 0
    has_chunk = bool(str(query_chunk_id or "").strip())
    if has_vector == has_chunk:
        fail("exactly one of query_vector or query_chunk_id is required")

    db = lancedb.connect(database_path)
    if table_name not in db.table_names():
        fail(f"table {table_name!r} does not exist")

    table = db.open_table(table_name)
    query_mode = "vector" if has_vector else "chunk_id"

    if has_chunk:
        rows = table.head(table.count_rows()).to_pylist()
        found = None
        for row in rows:
            if str(row.get(text_ref_column, "")) == str(query_chunk_id):
                found = row
                break
        if found is None:
            fail(f"query_chunk_id {query_chunk_id!r} not found")
        query_vector = found.get(vector_column)
        if not isinstance(query_vector, list) or len(query_vector) == 0:
            fail(f"chunk row for {query_chunk_id!r} has no vector")

    if not isinstance(query_vector, list) or len(query_vector) == 0:
        fail("query_vector is required")

    search_results = table.search(query_vector).limit(top_k).to_list()
    results: list[dict[str, Any]] = []
    for rank, row in enumerate(search_results, start=1):
        distance = row.get("_distance")
        if distance is None:
            distance = 0.0
        results.append(
            {
                "rank": rank,
                "chunk_id": row.get(text_ref_column) or row.get("chunk_id"),
                "vector_id": row.get("vector_id"),
                "distance": float(distance),
                "domain": row.get("domain"),
                "source_path": row.get("source_path"),
                "source_sha256": row.get("source_sha256"),
                "text_sha256": row.get("text_sha256"),
                "embedding_model": row.get("embedding_model"),
                "provider": row.get("provider"),
            }
        )

    ok(
        {
            "query_mode": query_mode,
            "top_k": top_k,
            "results": results,
        }
    )


if __name__ == "__main__":
    main()
