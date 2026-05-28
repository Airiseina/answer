import os
import logging
from typing import Optional, Literal

import httpx

from mem0.configs.embeddings.base import BaseEmbedderConfig
from mem0.embeddings.base import EmbeddingBase

logger = logging.getLogger("mem0_mcp_server")


class VolcengineEmbedding(EmbeddingBase):
    def __init__(self, config: Optional[BaseEmbedderConfig] = None):
        super().__init__(config)

        self.config.model = self.config.model or "doubao-embedding-vision-251215"
        self.config.embedding_dims = self.config.embedding_dims or 2048

        self.api_key = (
            self.config.api_key
            or os.getenv("EMBEDDING_API_KEY")
            or os.getenv("ARK_API_KEY")
        )
        self.base_url = (
            self.config.openai_base_url
            or "https://ark.cn-beijing.volces.com/api/v3"
        ).rstrip("/")

        if not self.api_key:
            raise ValueError(
                "Volcengine embedding API key is required. "
                "Set EMBEDDING_API_KEY or ARK_API_KEY environment variable."
            )

        self._client = httpx.Client(timeout=60.0)
        logger.info(
            "VolcengineEmbedding 初始化: model=%s, base_url=%s, dims=%d",
            self.config.model,
            self.base_url,
            self.config.embedding_dims,
        )

    def embed(
        self,
        text,
        memory_action: Optional[Literal["add", "search", "update"]] = None,
    ):
        text = text.replace("\n", " ")
        url = f"{self.base_url}/embeddings/multimodal"
        payload = {
            "model": self.config.model,
            "input": [{"type": "text", "text": text}],
            "encoding_format": "float",
        }
        if self.config.embedding_dims in (1024, 2048):
            payload["dimensions"] = self.config.embedding_dims

        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }

        try:
            response = self._client.post(url, json=payload, headers=headers)
            response.raise_for_status()
            data = response.json()
        except httpx.HTTPStatusError as e:
            logger.error(
                "Volcengine embedding API 错误: status=%d, body=%s",
                e.response.status_code,
                e.response.text[:500],
            )
            raise
        except httpx.RequestError as e:
            logger.error("Volcengine embedding 请求失败: %s", e)
            raise

        if isinstance(data.get("data"), dict):
            return data["data"]["embedding"]
        elif isinstance(data.get("data"), list) and len(data["data"]) > 0:
            return data["data"][0]["embedding"]
        else:
            raise ValueError(
                f"Unexpected response format from Volcengine multimodal API: "
                f"type(data)={type(data.get('data')).__name__}"
            )

    def embed_batch(self, texts, memory_action="add"):
        return [self.embed(text, memory_action) for text in texts]
