import os
import json
import argparse
from typing import Optional

from mcp.server.fastmcp import FastMCP
from mcp.server.sse import SseServerTransport
from starlette.applications import Starlette
from starlette.routing import Mount, Route
import uvicorn

from mem0 import Memory

mcp = FastMCP("mem0-memory")

_config = {
    "vector_store": {
        "provider": "qdrant",
        "config": {
            "host": os.getenv("QDRANT_HOST", "answer_qdrant"),
            "port": int(os.getenv("QDRANT_PORT", "6333")),
            "collection_name": "mem0_memories",
        },
    },
    "llm": {
        "provider": "openai",
        "config": {
            "model": os.getenv("LLM_MODEL", "glm-4-flash"),
            "openai_base_url": os.getenv("OPENAI_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
        },
    },
    "embedder": {
        "provider": "openai",
        "config": {
            "model": os.getenv("EMBEDDING_MODEL", "embedding-3"),
            "openai_base_url": os.getenv("OPENAI_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
        },
    },
    "version": "v1.1",
}

_mem0_client: Optional[Memory] = None


def _get_client() -> Memory:
    global _mem0_client
    if _mem0_client is None:
        _mem0_client = Memory.from_config(_config)
    return _mem0_client


@mcp.tool()
def add_memory(
    content: str,
    user_id: str,
    run_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    metadata: Optional[dict] = None,
) -> str:
    """Add a memory for a user. Use this to store any information that should be remembered across conversations.

    Args:
        content: The text content to remember
        user_id: The user identifier
        run_id: Optional conversation/session identifier for short-term memory scope
        agent_id: Optional agent identifier
        metadata: Optional metadata dict to attach
    """
    client = _get_client()
    kwargs = {"content": content, "user_id": user_id}
    if run_id:
        kwargs["run_id"] = run_id
    if agent_id:
        kwargs["agent_id"] = agent_id
    if metadata:
        kwargs["metadata"] = metadata
    result = client.add(**kwargs)
    return json.dumps(result, ensure_ascii=False, default=str)


@mcp.tool()
def search_memories(
    query: str,
    user_id: str,
    run_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    limit: int = 10,
) -> str:
    """Search memories semantically. Use this to retrieve relevant memories for a user.

    Args:
        query: The search query text
        user_id: The user identifier
        run_id: Optional conversation/session identifier (for short-term memory scope)
        agent_id: Optional agent identifier
        limit: Maximum number of results to return (default 10)
    """
    client = _get_client()
    kwargs = {"query": query, "user_id": user_id, "limit": limit}
    if run_id:
        kwargs["run_id"] = run_id
    if agent_id:
        kwargs["agent_id"] = agent_id
    results = client.search(**kwargs)
    return json.dumps(results, ensure_ascii=False, default=str)


@mcp.tool()
def get_all_memories(
    user_id: str,
    run_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    limit: int = 100,
) -> str:
    """List all memories for a user, optionally filtered by scope.

    Args:
        user_id: The user identifier
        run_id: Optional conversation/session identifier
        agent_id: Optional agent identifier
        limit: Maximum number of memories to return (default 100)
    """
    client = _get_client()
    kwargs = {"user_id": user_id, "limit": limit}
    if run_id:
        kwargs["run_id"] = run_id
    if agent_id:
        kwargs["agent_id"] = agent_id
    results = client.get_all(**kwargs)
    return json.dumps(results, ensure_ascii=False, default=str)


@mcp.tool()
def delete_memory(memory_id: str) -> str:
    """Delete a specific memory by its ID.

    Args:
        memory_id: The unique identifier of the memory to delete
    """
    client = _get_client()
    client.delete(memory_id)
    return json.dumps({"status": "deleted", "memory_id": memory_id})


@mcp.tool()
def delete_all_memories(
    user_id: str,
    run_id: Optional[str] = None,
    agent_id: Optional[str] = None,
) -> str:
    """Delete all memories for a user within a given scope. Use with caution.

    Args:
        user_id: The user identifier
        run_id: Optional conversation/session identifier
        agent_id: Optional agent identifier
    """
    client = _get_client()
    kwargs = {"user_id": user_id}
    if run_id:
        kwargs["run_id"] = run_id
    if agent_id:
        kwargs["agent_id"] = agent_id
    client.delete_all(**kwargs)
    return json.dumps({"status": "all_deleted", "user_id": user_id})


def create_app():
    sse = SseServerTransport("/messages/")

    async def handle_sse(request):
        async with sse.connect_sse(
            request.scope, request.receive, request._send
        ) as streams:
            await mcp._mcp_server.run(
                streams[0], streams[1], mcp._mcp_server.create_initialization_options()
            )

    return Starlette(
        routes=[
            Route("/sse", endpoint=handle_sse),
            Mount("/messages/", app=sse.handle_post_message),
        ],
    )


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["stdio", "sse"], default="sse")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", type=int, default=8000)
    args = parser.parse_args()

    if args.mode == "stdio":
        mcp.run()
    else:
        app = create_app()
        uvicorn.run(app, host=args.host, port=args.port)
