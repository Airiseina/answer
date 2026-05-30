import os
import logging
from typing import Optional, Literal

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

        from volcenginesdkarkruntime import Ark
        self._ark_client = Ark(
            api_key=self.api_key,
            base_url=self.base_url,
        )
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
        kwargs = {
            "model": self.config.model,
            "input": [{"type": "text", "text": text}],
            "encoding_format": "float",
        }
        if self.config.embedding_dims in (1024, 2048):
            kwargs["dimensions"] = self.config.embedding_dims

        try:
            resp = self._ark_client.multimodal_embeddings.create(**kwargs)
        except Exception as e:
            logger.error("Volcengine multimodal embedding 请求失败: %s", e)
            raise

        if hasattr(resp.data, "embedding"):
            return resp.data.embedding
        elif isinstance(resp.data, list) and len(resp.data) > 0:
            return resp.data[0].embedding
        else:
            raise ValueError(
                f"Unexpected response format from Volcengine multimodal API: "
                f"type={type(resp.data).__name__}"
            )

    def embed_batch(self, texts, memory_action="add"):
        return [self.embed(text, memory_action) for text in texts]
