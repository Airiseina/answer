import json
import logging
import os
from typing import Optional

import httpx
from mcp.server.fastmcp import FastMCP
from mcp.server.sse import SseServerTransport
from starlette.applications import Starlette
from starlette.responses import Response
from starlette.routing import Mount, Route

logger = logging.getLogger("searxng_mcp_server")
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s %(message)s"
)

mcp = FastMCP("searxng")

_searxng_base_url = os.getenv("SEARXNG_BASE_URL", "http://answer_searxng:8080")
_searxng_timeout = float(os.getenv("SEARXNG_TIMEOUT", "8.0"))

logger.info("SearXNG MCP Server 配置: URL=%s, Timeout=%.1fs", _searxng_base_url, _searxng_timeout)

_http_client: Optional[httpx.Client] = None


def _get_http_client() -> httpx.Client:
    global _http_client
    if _http_client is None:
        _http_client = httpx.Client(
            timeout=_searxng_timeout + 3.0,
            limits=httpx.Limits(max_keepalive_connections=5, max_connections=10),
        )
    return _http_client


@mcp.tool()
def web_search(query: str, max_results: int = 5) -> str:
    """搜索联网获取最新信息，用于需要实时数据的场合。返回标题、链接、摘要。

    Args:
        query: 搜索关键词
        max_results: 返回结果数(默认5,最多10)
    """
    max_results = min(max(max_results, 1), 10)

    try:
        client = _get_http_client()
        resp = client.get(
            f"{_searxng_base_url}/search",
            params={
                "q": query,
                "format": "json",
                "categories": "general",
                "language": "zh-CN",
                "pageno": 1,
            },
        )
        resp.raise_for_status()
        data = resp.json()

        results = []
        for item in data.get("results", [])[:max_results]:
            results.append({
                "title": item.get("title", ""),
                "url": item.get("url", ""),
                "snippet": item.get("content", ""),
            })

        return json.dumps({"results": results, "total": len(results)}, ensure_ascii=False)
    except httpx.HTTPStatusError as e:
        logger.error("SearXNG HTTP错误: %s", e)
        return f"搜索服务暂时不可用(HTTP {e.response.status_code})，请稍后再试或换一种方式提问。"
    except httpx.ConnectError:
        return "搜索服务连接失败，请稍后再试。"
    except httpx.TimeoutException:
        return "搜索请求超时，请稍后再试或简化搜索词。"
    except Exception as e:
        logger.error("web_search 失败: %s", e)
        return "搜索时发生错误，请稍后再试。"


@mcp.tool()
def news_search(query: str, max_results: int = 5) -> str:
    """搜索最新新闻。用于获取近期报道。

    Args:
        query: 搜索关键词
        max_results: 返回结果数(默认5,最多10)
    """
    max_results = min(max(max_results, 1), 10)

    try:
        client = _get_http_client()
        resp = client.get(
            f"{_searxng_base_url}/search",
            params={
                "q": query,
                "format": "json",
                "categories": "news",
                "language": "zh-CN",
                "pageno": 1,
            },
        )
        resp.raise_for_status()
        data = resp.json()

        results = []
        for item in data.get("results", [])[:max_results]:
            r = {
                "title": item.get("title", ""),
                "url": item.get("url", ""),
                "snippet": item.get("content", ""),
            }
            if item.get("publishedDate"):
                r["date"] = item["publishedDate"]
            results.append(r)

        return json.dumps({"results": results, "total": len(results)}, ensure_ascii=False)
    except httpx.HTTPStatusError as e:
        logger.error("SearXNG HTTP错误: %s", e)
        return f"新闻搜索服务暂时不可用(HTTP {e.response.status_code})，请稍后再试。"
    except httpx.ConnectError:
        return "新闻搜索服务连接失败，请稍后再试。"
    except httpx.TimeoutException:
        return "新闻搜索请求超时，请稍后再试或简化搜索词。"
    except Exception as e:
        logger.error("news_search 失败: %s", e)
        return "新闻搜索时发生错误，请稍后再试。"


def create_app():
    sse = SseServerTransport("/messages/")

    async def handle_sse(request):
        async with sse.connect_sse(
            request.scope, request.receive, request._send
        ) as streams:
            await mcp._mcp_server.run(
                streams[0], streams[1], mcp._mcp_server.create_initialization_options()
            )
        return Response()

    return Starlette(
        routes=[
            Route("/sse", endpoint=handle_sse, methods=["GET"]),
            Mount("/messages/", app=sse.handle_post_message),
        ],
    )
