#!/usr/bin/env python3
"""
index-rag.py - Index Centrora platform docs into Qdrant for MCP semantic search.

Uses fastembed (same engine as mcp-server-qdrant) so query vectors match index vectors.

Usage:
    python scripts/index-rag.py

Requirements:
    pip install fastembed qdrant-client

Run Qdrant first:
    podman run -d -p 6333:6333 --name qdrant qdrant/qdrant
"""

import pathlib
import sys
import uuid

try:
    from qdrant_client import QdrantClient
    from qdrant_client.models import Distance, PointStruct, VectorParams
except ImportError:
    print("ERROR: Run:  pip install fastembed qdrant-client")
    sys.exit(1)

try:
    from fastembed import TextEmbedding
except ImportError:
    print("ERROR: Run:  pip install fastembed")
    sys.exit(1)

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
QDRANT_URL = "http://localhost:6333"
COLLECTION = "centrora"
MODEL_NAME = "sentence-transformers/all-MiniLM-L6-v2"  # 384-dim, local, no API key
BASE = pathlib.Path(__file__).parent.parent  # repo root

# Files to index
INCLUDE_PATTERNS = [
    "docs/**/*.md",
    "*/CLAUDE.md",
    "CLAUDE.md",
    "AGENTS.md",
    "GEMINI.md",
    "CENTRORA_BLUEPRINT.md",
    "IMPLEMENTATION_PLAN.md",
]


# ---------------------------------------------------------------------------
# Chunking
# ---------------------------------------------------------------------------
def chunk_markdown(text: str, source: str, max_chars: int = 800) -> list:
    """Split markdown into chunks on ## headings, then by paragraph if too large."""
    chunks = []
    sections = text.split("\n## ")

    for i, section in enumerate(sections):
        if i > 0:
            section = "## " + section
        section = section.strip()
        if len(section) < 60:
            continue

        if len(section) > max_chars:
            paragraphs = section.split("\n\n")
            current = ""
            for para in paragraphs:
                if len(current) + len(para) > max_chars and current:
                    chunks.append({"content": current.strip(), "source": source})
                    current = para
                else:
                    current = (current + "\n\n" + para).strip() if current else para
            if current.strip():
                chunks.append({"content": current.strip(), "source": source})
        else:
            chunks.append({"content": section, "source": source})

    return chunks


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
def main():
    print(f"Loading embedding model: {MODEL_NAME}  (first run downloads ~90 MB)")
    model = TextEmbedding(MODEL_NAME)
    vector_size = 384  # all-MiniLM-L6-v2 output dimension

    print(f"\nConnecting to Qdrant at {QDRANT_URL}")
    client = QdrantClient(QDRANT_URL)

    # Recreate collection so re-runs start clean
    existing = [c.name for c in client.get_collections().collections]
    if COLLECTION in existing:
        print(f"Dropping existing collection '{COLLECTION}'")
        client.delete_collection(COLLECTION)

    print(f"Creating collection '{COLLECTION}'  (dim={vector_size}, cosine)")
    client.create_collection(
        collection_name=COLLECTION,
        vectors_config=VectorParams(size=vector_size, distance=Distance.COSINE),
    )

    # Collect all chunks
    print("\nIndexing files:")
    all_chunks = []
    for pattern in INCLUDE_PATTERNS:
        for path in sorted(BASE.glob(pattern)):
            try:
                text = path.read_text(encoding="utf-8")
                rel = str(path.relative_to(BASE)).replace("\\", "/")
                chunks = chunk_markdown(text, rel)
                all_chunks.extend(chunks)
                print(f"  {rel}  ({len(chunks)} chunks)")
            except Exception as exc:
                print(f"  SKIP {path}: {exc}")

    if not all_chunks:
        print("No files found - check that INCLUDE_PATTERNS match files in the repo.")
        sys.exit(1)

    # Embed
    print(f"\nEmbedding {len(all_chunks)} chunks ...")
    texts = [c["content"] for c in all_chunks]
    embeddings = list(model.embed(texts))

    # Upload
    points = [
        PointStruct(
            id=str(uuid.uuid4()),
            vector=embeddings[i].tolist(),
            payload={
                "content": all_chunks[i]["content"],
                "source": all_chunks[i]["source"],
            },
        )
        for i in range(len(all_chunks))
    ]

    print(f"Uploading {len(points)} vectors to Qdrant ...")
    client.upsert(collection_name=COLLECTION, points=points)

    print(f"\nDone. '{COLLECTION}' collection has {len(points)} vectors.")
    print("Start the MCP server and agents can now do semantic search over your docs.")


if __name__ == "__main__":
    main()
