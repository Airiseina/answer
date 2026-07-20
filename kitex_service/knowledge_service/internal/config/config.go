package config

import (
	pkgconfig "github.com/Airiseina/answer/pkg/config"
	"github.com/spf13/viper"
)

var V *viper.Viper

func GetConfig() {
	V = pkgconfig.LoadConfig()
	V.SetDefault("mysql.host", "localhost")
	V.SetDefault("mysql.port", "3306")
	V.SetDefault("mysql.user", "root")
	V.SetDefault("mysql.password", "123456")
	V.SetDefault("mysql.name", "answer")
	V.SetDefault("etcd.Addr", "127.0.0.1:2379")
	V.SetDefault("otel.Addr", "localhost:4317")
	V.SetDefault("kafka.brokers", "localhost:9094")
	V.SetDefault("kafka.topic.doc_parse", "doc-parse")
	V.SetDefault("kafka.group.knowledge", "knowledge-group")
	V.SetDefault("qdrant.host", "localhost")
	V.SetDefault("qdrant.port", 6334)
	V.SetDefault("meilisearch.host", "http://localhost:7700")
	V.SetDefault("meilisearch.api_key", "")
	V.SetDefault("seaweedfs.filer_url", "http://127.0.0.1:8888")
	V.SetDefault("seaweedfs.base_path", "/chat")
	V.SetDefault("seaweedfs.public_url", "/files")
	V.SetDefault("embedding.api_key", "")
	V.SetDefault("embedding.base_url", "https://ark.cn-beijing.volces.com/api/v3")
	// 使用文本向量化模型（支持标准 /embeddings 端点的批量调用）
	// 维度 2048 与 Qdrant collection 配置兼容
	V.SetDefault("embedding.model", "doubao-embedding-text-240715")
	// LLM 配置：用于实体抽取/关键词抽取等图谱构建任务
	// 配置缺失时实体抽取将降级为 N-gram 正则方案
	V.SetDefault("llm.api_key", "")
	V.SetDefault("llm.base_url", "https://ark.cn-beijing.volces.com/api/v3")
	V.SetDefault("llm.model", "")
	V.SetDefault("rerank.enabled", false)
	V.SetDefault("rerank.mode", "jina")
	V.SetDefault("rerank.base_url", "https://api.jina.ai/v1")
	V.SetDefault("rerank.model", "jina-reranker-v2-base-multilingual")
	V.SetDefault("rerank.top_n", 3)
	V.SetDefault("rerank.api_key", "")
	V.SetDefault("neo4j.uri", "bolt://localhost:7687")
	V.SetDefault("neo4j.username", "neo4j")
	V.SetDefault("neo4j.password", "password")
	V.SetDefault("neo4j.max_connections", 10)
}
