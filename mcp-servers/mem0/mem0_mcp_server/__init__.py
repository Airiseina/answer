import os
import json
import argparse
import logging
from typing import Optional

from mcp.server.fastmcp import FastMCP
from mcp.server.sse import SseServerTransport
from starlette.applications import Starlette
from starlette.routing import Mount, Route
import uvicorn

from mem0 import Memory

logger = logging.getLogger("mem0_mcp_server")
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s %(message)s")

mcp = FastMCP("mem0-memory")

_llm_api_key = os.getenv("LLM_API_KEY", "")
_embedding_api_key = os.getenv("EMBEDDING_API_KEY", "")

if not _llm_api_key:
    raise ValueError("LLM_API_KEY environment variable is not set. Please set it before starting the server.")
if not _embedding_api_key:
    raise ValueError("EMBEDDING_API_KEY environment variable is not set. Please set it before starting the server.")

os.environ.setdefault("OPENAI_API_KEY", _llm_api_key)

_embedding_provider = os.getenv("EMBEDDING_PROVIDER", "openai")
_embedding_dims = int(os.getenv("EMBEDDING_DIMS", "2048"))

_config = {
    "vector_store": {
        "provider": "qdrant",
        "config": {
            "host": os.getenv("QDRANT_HOST", "answer_qdrant"),
            "port": int(os.getenv("QDRANT_PORT", "6333")),
            "collection_name": "mem0_memories",
            "embedding_model_dims": _embedding_dims,
        },
    },
    "llm": {
        "provider": "openai",
        "config": {
            "model": os.getenv("LLM_MODEL", "deepseek-v4-flash"),
            "openai_base_url": os.getenv("LLM_BASE_URL", "https://api.deepseek.com/v1"),
            "api_key": _llm_api_key,
        },
    },
    "embedder": {
        "provider": "openai",
        "config": {
            "model": os.getenv("EMBEDDING_MODEL", "doubao-embedding-vision-251215"),
            "openai_base_url": os.getenv("EMBEDDING_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"),
            "api_key": _embedding_api_key,
            "embedding_dims": _embedding_dims,
        },
    },
    "version": "v1.1",
}

logger.info("mem0 MCP Server 配置:")
logger.info("  LLM: model=%s, base_url=%s, api_key=%s...%s",
            _config["llm"]["config"]["model"],
            _config["llm"]["config"]["openai_base_url"],
            _llm_api_key[:4] if len(_llm_api_key) >= 4 else "****",
            _llm_api_key[-4:] if len(_llm_api_key) >= 4 else "")
logger.info("  Embedder: provider=%s, model=%s, base_url=%s, api_key=%s...%s, dims=%d",
            _embedding_provider,
            _config["embedder"]["config"]["model"],
            _config["embedder"]["config"]["openai_base_url"],
            _embedding_api_key[:4] if len(_embedding_api_key) >= 4 else "****",
            _embedding_api_key[-4:] if len(_embedding_api_key) >= 4 else "",
            _embedding_dims)
logger.info("  VectorStore: qdrant@%s:%s",
            _config["vector_store"]["config"]["host"],
            _config["vector_store"]["config"]["port"])

_custom_embedder = None
if _embedding_provider == "volcengine":
    from mem0.configs.embeddings.base import BaseEmbedderConfig
    from mem0_mcp_server.volcengine_embedder import VolcengineEmbedding

    _embedder_config = BaseEmbedderConfig(
        model=os.getenv("EMBEDDING_MODEL", "doubao-embedding-vision-251215"),
        api_key=_embedding_api_key,
        openai_base_url=os.getenv("EMBEDDING_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"),
        embedding_dims=_embedding_dims,
    )
    _custom_embedder = VolcengineEmbedding(_embedder_config)
    logger.info("已创建 VolcengineEmbedding 自定义嵌入器")

_mem0_client: Optional[Memory] = None


def _get_client() -> Memory:
    global _mem0_client
    if _mem0_client is None:
        logger.info("正在初始化 mem0 客户端...")
        _mem0_client = Memory.from_config(_config)
        if _custom_embedder is not None:
            _mem0_client.embedding_model = _custom_embedder
            logger.info("已替换为 VolcengineEmbedding 自定义嵌入器")
        logger.info("mem0 客户端初始化完成")
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
    messages = [{"role": "user", "content": content}]
    kwargs = {"messages": messages, "user_id": user_id}
    if run_id:
        kwargs["run_id"] = run_id
    if agent_id:
        kwargs["agent_id"] = agent_id
    if metadata:
        kwargs["metadata"] = metadata
    try:
        result = client.add(**kwargs)
        return json.dumps(result, ensure_ascii=False, default=str)
    except Exception as e:
        logger.error("add_memory 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


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
    filters = {"user_id": user_id}
    if run_id:
        filters["run_id"] = run_id
    if agent_id:
        filters["agent_id"] = agent_id
    try:
        results = client.search(query=query, filters=filters, top_k=limit)
        return json.dumps(results, ensure_ascii=False, default=str)
    except Exception as e:
        logger.error("search_memories 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


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
    filters = {"user_id": user_id}
    if run_id:
        filters["run_id"] = run_id
    if agent_id:
        filters["agent_id"] = agent_id
    try:
        results = client.get_all(filters=filters, limit=limit)
        return json.dumps(results, ensure_ascii=False, default=str)
    except Exception as e:
        logger.error("get_all_memories 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


@mcp.tool()
def delete_memory(memory_id: str) -> str:
    """Delete a specific memory by its ID.

    Args:
        memory_id: The unique identifier of the memory to delete
    """
    client = _get_client()
    try:
        client.delete(memory_id)
        return json.dumps({"status": "deleted", "memory_id": memory_id})
    except Exception as e:
        logger.error("delete_memory 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


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
    filters = {"user_id": user_id}
    if run_id:
        filters["run_id"] = run_id
    if agent_id:
        filters["agent_id"] = agent_id
    try:
        client.delete_all(filters=filters)
        return json.dumps({"status": "all_deleted", "user_id": user_id})
    except Exception as e:
        logger.error("delete_all_memories 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


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
