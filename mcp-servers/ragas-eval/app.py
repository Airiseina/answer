"""
RAGAS 评估服务：对RAG系统进行自动化评估

提供REST API接口，支持：
1. 上传评估数据集（question, answer, contexts, ground_truth）
2. 运行RAGAS评估指标（faithfulness, answer_relevance, context_precision, context_recall）
3. 查询评估结果
4. 对接知识库检索服务进行端到端评估

兼容 RAGAS v0.4.x API
"""

import json
import logging
import os
import sys
import types
import uuid
from typing import Optional

# Monkey-patch: RAGAS 0.4.3 硬编码 from langchain_community.chat_models.vertexai import ChatVertexAI
# 但 langchain_community 0.4.2+ 已移除 vertexai 模块，需要手动注入
try:
    from langchain_google_vertexai import ChatVertexAI
    import langchain_community.chat_models
    vertexai_module = types.ModuleType("langchain_community.chat_models.vertexai")
    vertexai_module.ChatVertexAI = ChatVertexAI
    sys.modules["langchain_community.chat_models.vertexai"] = vertexai_module
except ImportError:
    pass

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

logger = logging.getLogger("ragas_eval_service")
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s %(message)s"
)

app = FastAPI(
    title="RAGAS Evaluation Service",
    description="Docker化的RAGAS评估服务，用于评估RAG系统的检索和生成质量",
    version="1.0.0",
)

# LLM配置（用于RAGAS评估的LLM judge）
_llm_api_key = os.getenv("LLM_API_KEY", "")
_llm_base_url = os.getenv("LLM_BASE_URL", "https://api.openai.com/v1")
_llm_model = os.getenv("LLM_MODEL", "gpt-4o-mini")
# Embedding配置（用于RAGAS评估的embedding）
_embedding_api_key = os.getenv("EMBEDDING_API_KEY", "")
_embedding_base_url = os.getenv("EMBEDDING_BASE_URL", "https://api.openai.com/v1")
_embedding_model = os.getenv("EMBEDDING_MODEL", "text-embedding-3-small")

# 评估结果存储（生产环境应替换为数据库）
_eval_results: dict = {}


class EvalSample(BaseModel):
    """单条评估样本"""

    question: str = Field(..., description="用户问题")
    answer: str = Field(default="", description="生成的答案（留空则自动检索生成）")
    contexts: list[str] = Field(
        default_factory=list, description="检索到的上下文（留空则自动检索）"
    )
    ground_truth: str = Field(default="", description="标准答案")


class EvalDataset(BaseModel):
    """评估数据集"""

    samples: list[EvalSample] = Field(..., description="评估样本列表")
    kb_ids: str = Field(
        default="", description="知识库ID列表（逗号分隔），用于自动检索"
    )


class EvalRequest(BaseModel):
    """评估请求"""

    dataset: EvalDataset
    metrics: list[str] = Field(
        default=["faithfulness", "answer_relevance", "context_precision", "context_recall"],
        description="要评估的指标列表",
    )


class EvalResult(BaseModel):
    """评估结果"""

    eval_id: str
    status: str
    metrics: dict = Field(default_factory=dict)
    details: list[dict] = Field(default_factory=list)
    error: Optional[str] = None


async def _search_knowledge(query: str, kb_ids: str, top_k: int = 5) -> list[str]:
    """调用知识库检索服务获取上下文"""
    import httpx

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            knowledge_rpc_addr = os.getenv(
                "KNOWLEDGE_RPC_ADDR", "http://answer_knowledge_service:4326"
            )
            resp = await client.post(
                f"{knowledge_rpc_addr}/api/knowledge/search",
                json={"kb_ids": kb_ids, "query": query, "top_k": top_k},
            )
            resp.raise_for_status()
            data = resp.json()
            return [item.get("content", "") for item in data.get("results", [])]
    except Exception as e:
        logger.error("知识库检索失败: %s", e)
        return []


async def _run_ragas_eval(dataset: EvalDataset, metrics: list[str], eval_id: str):
    """异步运行RAGAS评估（兼容v0.4.x API）"""
    try:
        import asyncio
        from concurrent.futures import ThreadPoolExecutor
        from datasets import Dataset
        from langchain_openai import ChatOpenAI, OpenAIEmbeddings

        # RAGAS v0.4.x: 直接传入LangChain模型，不需要Wrapper
        llm = ChatOpenAI(
            api_key=_llm_api_key,
            base_url=_llm_base_url,
            model=_llm_model,
        )
        embeddings = OpenAIEmbeddings(
            api_key=_embedding_api_key,
            base_url=_embedding_base_url,
            model=_embedding_model,
        )

        # 准备评估数据 - RAGAS v0.4.3 使用新字段名
        user_inputs = []
        responses = []
        retrieved_contexts_list = []
        references = []

        for sample in dataset.samples:
            user_inputs.append(sample.question)

            # 如果没有提供contexts，自动检索
            ctxs = sample.contexts
            if not ctxs and dataset.kb_ids:
                ctxs = await _search_knowledge(sample.question, dataset.kb_ids)

            retrieved_contexts_list.append(ctxs)
            responses.append(sample.answer or "")
            references.append(sample.ground_truth)

        eval_data = {
            "user_input": user_inputs,
            "response": responses,
            "retrieved_contexts": retrieved_contexts_list,
            "reference": references,
        }

        # 选择评估指标 - RAGAS v0.4.3: 类名和路径已变更
        from ragas import evaluate
        from ragas.metrics._faithfulness import Faithfulness
        from ragas.metrics._answer_relevance import AnswerRelevancy
        from ragas.metrics._context_precision import ContextPrecision
        from ragas.metrics._context_recall import ContextRecall

        metrics_map = {
            "faithfulness": Faithfulness(),
            "answer_relevance": AnswerRelevancy(),
            "context_precision": ContextPrecision(),
            "context_recall": ContextRecall(),
        }
        selected_metrics = [metrics_map[m] for m in metrics if m in metrics_map]

        if not selected_metrics:
            selected_metrics = list(metrics_map.values())

        # 运行评估 - RAGAS v0.4.x: evaluate() 是同步阻塞调用，需要在线程中运行
        dataset_obj = Dataset.from_dict(eval_data)

        def _sync_evaluate():
            return evaluate(
                dataset_obj,
                metrics=selected_metrics,
                llm=llm,
                embeddings=embeddings,
            )

        loop = asyncio.get_event_loop()
        result = await loop.run_in_executor(ThreadPoolExecutor(max_workers=1), _sync_evaluate)

        # 存储结果 - RAGAS v0.4.3 返回 EvaluationResult 对象
        _eval_results[eval_id]["status"] = "completed"
        # 尝试从 result 中提取指标分数
        try:
            if hasattr(result, "scores"):
                scores = result.scores
                if isinstance(scores, dict):
                    _eval_results[eval_id]["metrics"] = {
                        k: float(v) for k, v in scores.items()
                    }
                elif isinstance(scores, list):
                    # scores 可能是 list of dict
                    if scores and isinstance(scores[0], dict):
                        merged = {}
                        for s in scores:
                            for k, v in s.items():
                                merged[k] = float(v) if isinstance(v, (int, float)) else v
                        _eval_results[eval_id]["metrics"] = merged
                    else:
                        _eval_results[eval_id]["metrics"] = {"scores": str(scores)}
                else:
                    _eval_results[eval_id]["metrics"] = {"scores": str(scores)}
            elif hasattr(result, "to_pandas"):
                df = result.to_pandas()
                metric_cols = [c for c in df.columns if c not in ("question", "answer", "contexts", "ground_truth")]
                _eval_results[eval_id]["metrics"] = {
                    col: float(df[col].mean()) for col in metric_cols if df[col].dtype in ("float64", "int64", "float32")
                }
                details = []
                for _, row in df.iterrows():
                    detail = {}
                    for col in df.columns:
                        val = row[col]
                        if isinstance(val, (int, float)):
                            detail[col] = float(val)
                        elif isinstance(val, list):
                            detail[col] = val
                        else:
                            detail[col] = str(val)
                    details.append(detail)
                _eval_results[eval_id]["details"] = details
            else:
                try:
                    _eval_results[eval_id]["metrics"] = {
                        k: float(v) for k, v in result.items() if k != "question"
                    }
                except (AttributeError, TypeError):
                    _eval_results[eval_id]["metrics"] = {"raw": str(result)}
        except Exception as parse_err:
            logger.warning("解析评估结果时出错: %s, 原始结果: %s", parse_err, result)
            _eval_results[eval_id]["metrics"] = {"raw": str(result)}

        logger.info("评估[%s]完成: %s", eval_id, _eval_results[eval_id]["metrics"])

    except Exception as e:
        logger.error("评估[%s]失败: %s", eval_id, e, exc_info=True)
        _eval_results[eval_id]["status"] = "failed"
        _eval_results[eval_id]["error"] = str(e)


@app.post("/evaluate", response_model=EvalResult)
async def create_evaluation(request: EvalRequest):
    """创建RAGAS评估任务"""
    eval_id = str(uuid.uuid4())[:8]
    sample_count = len(request.dataset.samples) if request.dataset and request.dataset.samples else 0
    logger.info(
        "收到评估请求: eval_id=%s, metrics=%s, samples=%d, kb_ids=%s",
        eval_id, request.metrics, sample_count, request.dataset.kb_ids if request.dataset else "",
    )
    for i, sample in enumerate(request.dataset.samples or []):
        logger.info(
            "  样本[%d]: question=%s, answer长度=%d, contexts数量=%d",
            i, sample.question[:50] if sample.question else "",
            len(sample.answer) if sample.answer else 0,
            len(sample.contexts) if sample.contexts else 0,
        )
    _eval_results[eval_id] = {
        "eval_id": eval_id,
        "status": "running",
        "metrics": {},
        "details": [],
        "error": None,
    }

    # 异步运行评估
    import asyncio

    asyncio.create_task(_run_ragas_eval(request.dataset, request.metrics, eval_id))

    return EvalResult(
        eval_id=eval_id,
        status="running",
    )


@app.get("/evaluate/{eval_id}", response_model=EvalResult)
async def get_evaluation(eval_id: str):
    """查询评估结果"""
    if eval_id not in _eval_results:
        raise HTTPException(status_code=404, detail="评估任务不存在")
    result = _eval_results[eval_id]
    return EvalResult(**result)


@app.get("/evaluate", response_model=list[EvalResult])
async def list_evaluations():
    """列出所有评估任务"""
    return [EvalResult(**v) for v in _eval_results.values()]


@app.delete("/evaluate/{eval_id}")
async def delete_evaluation(eval_id: str):
    """删除评估结果"""
    if eval_id not in _eval_results:
        raise HTTPException(status_code=404, detail="评估任务不存在")
    del _eval_results[eval_id]
    return {"message": "已删除"}


@app.get("/health")
async def health_check():
    """健康检查"""
    return {"status": "ok", "service": "ragas-eval"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8090)
