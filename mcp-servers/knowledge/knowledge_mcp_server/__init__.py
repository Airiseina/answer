import os
import json
import logging
from typing import Optional

from mcp.server.fastmcp import FastMCP
from mcp.server.sse import SseServerTransport
from starlette.applications import Starlette
from starlette.routing import Mount, Route
import uvicorn

import httpx
import pymysql

logger = logging.getLogger("knowledge_mcp_server")
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s %(message)s")

mcp = FastMCP("knowledge")

_knowledge_service_addr = os.getenv("KNOWLEDGE_SERVICE_ADDR", "http://127.0.0.1:4326")
_qdrant_host = os.getenv("QDRANT_HOST", "answer_qdrant")
_qdrant_port = int(os.getenv("QDRANT_PORT", "6334"))
_qdrant_http_port = int(os.getenv("QDRANT_HTTP_PORT", "6333"))
_embedding_api_key = os.getenv("EMBEDDING_API_KEY", "")
_embedding_base_url = os.getenv("EMBEDDING_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3")
_embedding_model = os.getenv("EMBEDDING_MODEL", "doubao-embedding-vision-251215")
_embedding_dims = int(os.getenv("EMBEDDING_DIMS", "2048"))

_mysql_host = os.getenv("MYSQL_HOST", "answer_mysql")
_mysql_port = int(os.getenv("MYSQL_PORT", "3306"))
_mysql_user = os.getenv("MYSQL_USER", "root")
_mysql_password = os.getenv("MYSQL_PASSWORD", "123456")
_mysql_db = os.getenv("MYSQL_DB", "answer")

logger.info("knowledge MCP Server 配置:")
logger.info("  KnowledgeService: %s", _knowledge_service_addr)
logger.info("  MySQL: %s:%d/%s", _mysql_host, _mysql_port, _mysql_db)
logger.info("  Qdrant: %s:%d (gRPC), %s:%d (HTTP)", _qdrant_host, _qdrant_port, _qdrant_host, _qdrant_http_port)
logger.info("  Embedding: model=%s, base_url=%s, dims=%d", _embedding_model, _embedding_base_url, _embedding_dims)

_http_client: Optional[httpx.Client] = None


def _get_http_client() -> httpx.Client:
    global _http_client
    if _http_client is None:
        _http_client = httpx.Client(timeout=30.0)
    return _http_client


def _get_mysql_conn():
    return pymysql.connect(
        host=_mysql_host,
        port=_mysql_port,
        user=_mysql_user,
        password=_mysql_password,
        database=_mysql_db,
        charset="utf8mb4",
        cursorclass=pymysql.cursors.DictCursor,
    )


def _call_knowledge_service(method: str, endpoint: str, payload: dict = None) -> dict:
    client = _get_http_client()
    url = f"{_knowledge_service_addr}{endpoint}"
    try:
        if method == "GET":
            resp = client.get(url, params=payload)
        else:
            resp = client.post(url, json=payload)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        logger.error("调用KnowledgeService失败: %s %s, error: %s", method, url, e)
        return {"error": str(e)}


_ark_client: Optional = None


def _get_ark_client():
    global _ark_client
    if _ark_client is None:
        from volcenginesdkarkruntime import AsyncArk
        _ark_client = AsyncArk(
            api_key=_embedding_api_key,
            base_url=_embedding_base_url,
        )
    return _ark_client


async def _get_embedding(text: str) -> list:
    text = text.replace("\n", " ")
    client = _get_ark_client()
    kwargs = {
        "model": _embedding_model,
        "input": [{"type": "text", "text": text}],
        "encoding_format": "float",
    }
    if _embedding_dims in (1024, 2048):
        kwargs["dimensions"] = _embedding_dims
    try:
        resp = await client.multimodal_embeddings.create(**kwargs)
    except Exception as e:
        logger.error("Volcengine multimodal embedding 请求失败: %s", e)
        raise
    if hasattr(resp.data, "embedding"):
        return resp.data.embedding
    elif isinstance(resp.data, list) and len(resp.data) > 0:
        return resp.data[0].embedding
    else:
        raise ValueError(f"Unexpected response format from Volcengine multimodal API: type={type(resp.data).__name__}")


async def _search_qdrant(kb_ids: list, query_vector: list, top_k: int = 5) -> list:
    from qdrant_client import QdrantClient
    from qdrant_client import models

    client = QdrantClient(host=_qdrant_host, port=_qdrant_http_port)
    should_conditions = [
        models.FieldCondition(key="kb_id", match=models.MatchValue(value=kb_id))
        for kb_id in kb_ids
    ]
    results = client.query_points(
        collection_name="kb_chunks",
        query=query_vector,
        query_filter=models.Filter(should=should_conditions),
        limit=top_k,
        with_payload=True,
    )
    chunks = []
    for point in results.points:
        chunk = {
            "score": point.score,
            "content": point.payload.get("content", ""),
            "source": point.payload.get("source", ""),
            "doc_id": point.payload.get("doc_id", 0),
            "kb_id": point.payload.get("kb_id", 0),
            "chunk_index": point.payload.get("chunk_index", 0),
        }
        if "page_number" in point.payload:
            chunk["page_number"] = point.payload["page_number"]
        chunks.append(chunk)
    return chunks


@mcp.tool()
def search_knowledge(
    query: str,
    kb_ids: str,
    top_k: int = 5,
) -> str:
    """Search knowledge bases semantically. Use this when the user asks a question that might be answered by documents in their knowledge base.

    Args:
        query: The search query text
        kb_ids: Comma-separated knowledge base IDs to search in (e.g. "123,456")
        top_k: Maximum number of results to return (default 5)
    """
    import asyncio

    ids = [int(x.strip()) for x in kb_ids.split(",") if x.strip()]

    async def _do_search():
        query_vector = await _get_embedding(query)
        results = await _search_qdrant(ids, query_vector, top_k)
        return results

    try:
        loop = asyncio.get_event_loop()
        if loop.is_running():
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor() as pool:
                results = pool.submit(asyncio.run, _do_search()).result()
        else:
            results = loop.run_until_complete(_do_search())
        return json.dumps(results, ensure_ascii=False, default=str)
    except Exception as e:
        logger.error("search_knowledge 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


@mcp.tool()
def list_knowledge_bases(
    owner_id: str,
) -> str:
    """List all knowledge bases owned by a user. Use this to discover what knowledge bases are available.

    Args:
        owner_id: The user identifier
    """
    try:
        conn = _get_mysql_conn()
        try:
            with conn.cursor() as cursor:
                cursor.execute(
                    "SELECT id, name, description, owner_id, doc_count, created_at, updated_at "
                    "FROM knowledge_base WHERE owner_id = %s",
                    (int(owner_id),),
                )
                rows = cursor.fetchall()
            return json.dumps({"knowledge_bases": rows}, ensure_ascii=False, default=str)
        finally:
            conn.close()
    except Exception as e:
        logger.error("list_knowledge_bases 失败: %s", e)
        return json.dumps({"error": str(e)}, ensure_ascii=False)


@mcp.tool()
def get_bot_knowledge_bases(
    bot_id: str,
) -> str:
    """Get all knowledge bases bound to a specific bot. Use this to find which knowledge bases a bot can access.

    Args:
        bot_id: The bot identifier
    """
    try:
        conn = _get_mysql_conn()
        try:
            with conn.cursor() as cursor:
                cursor.execute(
                    "SELECT kb.id, kb.name, kb.description, kb.owner_id, kb.doc_count, kb.created_at, kb.updated_at "
                    "FROM knowledge_base kb "
                    "INNER JOIN bot_knowledge bk ON kb.id = bk.kb_id "
                    "WHERE bk.bot_id = %s",
                    (int(bot_id),),
                )
                rows = cursor.fetchall()
            return json.dumps({"knowledge_bases": rows}, ensure_ascii=False, default=str)
        finally:
            conn.close()
    except Exception as e:
        logger.error("get_bot_knowledge_bases 失败: %s", e)
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
