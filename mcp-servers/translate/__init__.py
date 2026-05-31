import argparse
import json
import os
from typing import Optional

import httpx
import uvicorn
from mcp.server.fastmcp import FastMCP
from mcp.server.sse import SseServerTransport
from starlette.applications import Starlette
from starlette.routing import Mount, Route

mcp = FastMCP("translate")

MYMEMORY_API = "https://api.mymemory.translated.net/get"
DEEPL_FREE_API = "https://api-free.deepl.com/v2/translate"


@mcp.tool()
def translate_text(
    text: str,
    source_lang: str,
    target_lang: str,
    provider: Optional[str] = None,
) -> str:
    """Translate text from one language to another.

    Args:
        text: The text to translate
        source_lang: Source language code (e.g. en, zh, ja, ko, fr, de, es)
        target_lang: Target language code (e.g. en, zh, ja, ko, fr, de, es)
        provider: Translation provider: 'mymemory' (default, free) or 'deepl_free' (requires DEEPL_API_KEY env var)
    """
    prov = provider or os.getenv("TRANSLATE_PROVIDER", "mymemory")

    if prov == "deepl_free":
        api_key = os.getenv("DEEPL_API_KEY", "")
        if not api_key:
            return json.dumps({"error": "DEEPL_API_KEY environment variable is required for deepl_free provider"}, ensure_ascii=False)
        resp = httpx.post(
            DEEPL_FREE_API,
            data={
                "auth_key": api_key,
                "text": text,
                "source_lang": source_lang.upper(),
                "target_lang": target_lang.upper(),
            },
            timeout=30,
        )
        resp.raise_for_status()
        data = resp.json()
        translated = data["translations"][0]["text"]
        return json.dumps({
            "translated_text": translated,
            "source_lang": source_lang,
            "target_lang": target_lang,
            "provider": "deepl_free",
        }, ensure_ascii=False)

    resp = httpx.get(
        MYMEMORY_API,
        params={
            "q": text,
            "langpair": f"{source_lang}|{target_lang}",
        },
        timeout=30,
    )
    resp.raise_for_status()
    data = resp.json()
    translated = data.get("responseData", {}).get("translatedText", "")
    if not translated or "MYMEMORY WARNING" in translated.upper():
        matches = data.get("matches", [])
        if matches:
            translated = matches[0].get("translation", text)
        else:
            translated = text
    return json.dumps({
        "translated_text": translated,
        "source_lang": source_lang,
        "target_lang": target_lang,
        "provider": "mymemory",
    }, ensure_ascii=False)


@mcp.tool()
def detect_language(text: str) -> str:
    """Detect the language of the given text using heuristics and common patterns.

    Args:
        text: The text to detect the language of
    """
    import re
    cjk = len(re.findall(r'[\u4e00-\u9fff]', text))
    hiragana = len(re.findall(r'[\u3040-\u309f]', text))
    katakana = len(re.findall(r'[\u30a0-\u30ff]', text))
    hangul = len(re.findall(r'[\uac00-\ud7af]', text))
    cyrillic = len(re.findall(r'[\u0400-\u04ff]', text))
    arabic = len(re.findall(r'[\u0600-\u06ff]', text))
    latin = len(re.findall(r'[a-zA-Z]', text))
    scores = {
        "zh": cjk,
        "ja": hiragana + katakana,
        "ko": hangul,
        "ru": cyrillic,
        "ar": arabic,
        "en": latin,
    }
    detected = max(scores, key=scores.get)
    if scores[detected] == 0:
        detected = "unknown"
    return json.dumps({"detected_lang": detected, "text_sample": text[:100]}, ensure_ascii=False)


@mcp.tool()
def list_supported_languages() -> str:
    """List all supported language codes and their names for translation."""
    languages = [
        {"code": "en", "name": "English"},
        {"code": "zh", "name": "Chinese"},
        {"code": "ja", "name": "Japanese"},
        {"code": "ko", "name": "Korean"},
        {"code": "fr", "name": "French"},
        {"code": "de", "name": "German"},
        {"code": "es", "name": "Spanish"},
        {"code": "pt", "name": "Portuguese"},
        {"code": "it", "name": "Italian"},
        {"code": "ru", "name": "Russian"},
        {"code": "ar", "name": "Arabic"},
        {"code": "hi", "name": "Hindi"},
        {"code": "th", "name": "Thai"},
        {"code": "vi", "name": "Vietnamese"},
        {"code": "id", "name": "Indonesian"},
        {"code": "nl", "name": "Dutch"},
        {"code": "pl", "name": "Polish"},
        {"code": "tr", "name": "Turkish"},
        {"code": "uk", "name": "Ukrainian"},
        {"code": "sv", "name": "Swedish"},
    ]
    return json.dumps(languages, ensure_ascii=False)


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
