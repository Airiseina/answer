package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jGraph 知识图谱存储，基于Neo4j实现GraphRAG
type Neo4jGraph struct {
	driver neo4j.DriverWithContext
}

// NewNeo4jGraph 创建Neo4j图谱连接
func NewNeo4jGraph(uri, username, password string, maxConns int) (*Neo4jGraph, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""),
		func(c *neo4j.Config) {
			c.MaxConnectionPoolSize = maxConns
		},
	)
	if err != nil {
		return nil, fmt.Errorf("创建Neo4j驱动失败: %w", err)
	}
	return &Neo4jGraph{driver: driver}, nil
}

// Close 关闭Neo4j连接
func (g *Neo4jGraph) Close() error {
	if g.driver != nil {
		return g.driver.Close(context.Background())
	}
	return nil
}

// EnsureConstraints 确保必要的唯一约束存在
func (g *Neo4jGraph) EnsureConstraints(ctx context.Context) error {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Chunk节点唯一约束
	_, err := session.Run(ctx,
		"CREATE CONSTRAINT chunk_id IF NOT EXISTS FOR (c:Chunk) REQUIRE c.chunk_id IS UNIQUE",
		nil,
	)
	if err != nil {
		klog.Warnf("创建Chunk约束失败(可能已存在): %v", err)
	}

	// Entity节点唯一约束
	_, err = session.Run(ctx,
		"CREATE CONSTRAINT entity_name IF NOT EXISTS FOR (e:Entity) REQUIRE e.name IS UNIQUE",
		nil,
	)
	if err != nil {
		klog.Warnf("创建Entity约束失败(可能已存在): %v", err)
	}

	return nil
}

// ChunkData 分块数据，用于图索引
type ChunkData struct {
	ChunkID    string
	KBID       int64
	DocID      int64
	Content    string
	ChunkIndex int
	Source     string
}

// EntityData 实体数据
type EntityData struct {
	Name        string
	Type        string // 实体类型：Person, Organization, Concept, Technology等
	Description string
	ChunkIDs    []string // 关联的分块ID
}

// RelationData 关系数据
type RelationData struct {
	SourceEntity string
	TargetEntity string
	RelationType string
}

// IndexChunks 将文档分块索引到知识图谱
// 为每个Chunk创建节点，并提取实体和关系
func (g *Neo4jGraph) IndexChunks(ctx context.Context, chunks []ChunkData) error {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		for _, chunk := range chunks {
			// 创建Chunk节点
			_, err := tx.Run(ctx,
				`MERGE (c:Chunk {chunk_id: $chunk_id})
				 SET c.kb_id = $kb_id, c.doc_id = $doc_id, c.content = $content,
				     c.chunk_index = $chunk_index, c.source = $source`,
				map[string]interface{}{
					"chunk_id":    chunk.ChunkID,
					"kb_id":       chunk.KBID,
					"doc_id":      chunk.DocID,
					"content":     chunk.Content,
					"chunk_index": chunk.ChunkIndex,
					"source":      chunk.Source,
				},
			)
			if err != nil {
				return nil, fmt.Errorf("创建Chunk节点失败: %w", err)
			}
		}
		return nil, nil
	})
	return err
}

// IndexEntities 将实体和关系索引到知识图谱
func (g *Neo4jGraph) IndexEntities(ctx context.Context, entities []EntityData, relations []RelationData) error {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// 创建实体节点并关联到Chunk
		for _, entity := range entities {
			_, err := tx.Run(ctx,
				`MERGE (e:Entity {name: $name})
				 SET e.type = $type, e.description = $description`,
				map[string]interface{}{
					"name":        entity.Name,
					"type":        entity.Type,
					"description": entity.Description,
				},
			)
			if err != nil {
				klog.Warnf("创建Entity节点失败: %v", err)
				continue
			}

			// 关联实体到Chunk
			for _, chunkID := range entity.ChunkIDs {
				_, err := tx.Run(ctx,
					`MATCH (e:Entity {name: $name}), (c:Chunk {chunk_id: $chunk_id})
					 MERGE (e)-[:MENTIONED_IN]->(c)`,
					map[string]interface{}{
						"name":     entity.Name,
						"chunk_id": chunkID,
					},
				)
				if err != nil {
					klog.Warnf("关联Entity到Chunk失败: %v", err)
				}
			}
		}

		// 创建实体间关系
		for _, rel := range relations {
			_, err := tx.Run(ctx,
				fmt.Sprintf(
					`MATCH (s:Entity {name: $source}), (t:Entity {name: $target})
					 MERGE (s)-[:%s]->(t)`,
					sanitizeRelType(rel.RelationType),
				),
				map[string]interface{}{
					"source": rel.SourceEntity,
					"target": rel.TargetEntity,
				},
			)
			if err != nil {
				klog.Warnf("创建关系失败(%s->%s:%s): %v", rel.SourceEntity, rel.TargetEntity, rel.RelationType, err)
			}
		}

		return nil, nil
	})
	return err
}

// GraphSearchResult 图检索结果
type GraphSearchResult struct {
	Content    string  `json:"content"`
	Source     string  `json:"source"`
	DocID      int64   `json:"doc_id"`
	KBID       int64   `json:"kb_id"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
}

// SearchByKeywords 通过关键词在图谱中检索相关Chunk
// 策略：匹配Chunk内容中的关键词，并通过实体关系扩展检索范围
func (g *Neo4jGraph) SearchByKeywords(ctx context.Context, kbIDs []int64, keywords []string, topK int) ([]GraphSearchResult, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// 构建kb_id过滤条件
	kbIDList := make([]interface{}, len(kbIDs))
	for i, id := range kbIDs {
		kbIDList[i] = id
	}

	// 第一步：直接通过关键词匹配Chunk内容
	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			MATCH (c:Chunk)
			WHERE c.kb_id IN $kb_ids
			  AND ANY(keyword IN $keywords WHERE toLower(c.content) CONTAINS toLower(keyword))
			WITH c, size([kw IN $keywords WHERE toLower(c.content) CONTAINS toLower(kw)]) AS matchCount
			ORDER BY matchCount DESC
			LIMIT $top_k
			RETURN c.content AS content, c.source AS source, c.doc_id AS doc_id,
			       c.kb_id AS kb_id, c.chunk_index AS chunk_index, matchCount AS score
		`
		records, err := tx.Run(ctx, query, map[string]interface{}{
			"kb_ids":   kbIDList,
			"keywords": keywords,
			"top_k":    topK,
		})
		if err != nil {
			return nil, err
		}

		var results []GraphSearchResult
		for records.Next(ctx) {
			record := records.Record()
			content, _ := record.Get("content")
			source, _ := record.Get("source")
			docID, _ := record.Get("doc_id")
			kbID, _ := record.Get("kb_id")
			chunkIndex, _ := record.Get("chunk_index")
			score, _ := record.Get("score")

			results = append(results, GraphSearchResult{
				Content:    toString(content),
				Source:     toString(source),
				DocID:      toInt64(docID),
				KBID:       toInt64(kbID),
				ChunkIndex: toInt(chunkIndex),
				Score:      toFloat64(score),
			})
		}
		return results, nil
	})

	if err != nil {
		return nil, fmt.Errorf("图谱关键词检索失败: %w", err)
	}

	directResults, ok := result.([]GraphSearchResult)
	if !ok {
		return nil, nil
	}

	// 第二步：通过实体关系扩展检索（使用新session）
	expandedResults, err := g.searchByEntityExpansion(ctx, kbIDList, keywords, topK)
	if err != nil {
		klog.Warnf("图谱实体扩展检索失败，仅使用直接匹配结果: %v", err)
		return directResults, nil
	}

	// 合并去重
	return mergeGraphResults(directResults, expandedResults, topK), nil
}

// searchByEntityExpansion 通过实体关系扩展检索
// 找到与关键词匹配的实体，然后通过关系找到关联的Chunk
func (g *Neo4jGraph) searchByEntityExpansion(ctx context.Context, kbIDs []interface{}, keywords []string, topK int) ([]GraphSearchResult, error) {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)
	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := `
			// 找到名称匹配关键词的实体
			MATCH (e:Entity)
			WHERE ANY(kw IN $keywords WHERE toLower(e.name) CONTAINS toLower(kw))
			// 通过实体找到关联的Chunk
			MATCH (e)-[:MENTIONED_IN]->(c:Chunk)
			WHERE c.kb_id IN $kb_ids
			// 也通过实体间关系扩展到相邻实体关联的Chunk
			OPTIONAL MATCH (e)-[]-(related:Entity)-[:MENTIONED_IN]->(rc:Chunk)
			WHERE rc.kb_id IN $kb_ids
			WITH collect(DISTINCT c) + collect(DISTINCT rc) AS allChunks
			UNWIND allChunks AS chunk
			WITH DISTINCT chunk
			LIMIT $top_k
			RETURN chunk.content AS content, chunk.source AS source, chunk.doc_id AS doc_id,
			       chunk.kb_id AS kb_id, chunk.chunk_index AS chunk_index, 0.5 AS score
		`
		records, err := tx.Run(ctx, query, map[string]interface{}{
			"kb_ids":   kbIDs,
			"keywords": keywords,
			"top_k":    topK,
		})
		if err != nil {
			return nil, err
		}

		var results []GraphSearchResult
		for records.Next(ctx) {
			record := records.Record()
			content, _ := record.Get("content")
			source, _ := record.Get("source")
			docID, _ := record.Get("doc_id")
			kbID, _ := record.Get("kb_id")
			chunkIndex, _ := record.Get("chunk_index")
			score, _ := record.Get("score")

			results = append(results, GraphSearchResult{
				Content:    toString(content),
				Source:     toString(source),
				DocID:      toInt64(docID),
				KBID:       toInt64(kbID),
				ChunkIndex: toInt(chunkIndex),
				Score:      toFloat64(score),
			})
		}
		return results, nil
	})

	if err != nil {
		return nil, err
	}

	results, ok := result.([]GraphSearchResult)
	if !ok {
		return nil, nil
	}
	return results, nil
}

// DeleteByDocID 删除文档关联的所有图节点
func (g *Neo4jGraph) DeleteByDocID(ctx context.Context, docID int64) error {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// 删除文档关联的Chunk节点及其关系
		_, err := tx.Run(ctx,
			`MATCH (c:Chunk {doc_id: $doc_id})
			 DETACH DELETE c`,
			map[string]interface{}{"doc_id": docID},
		)
		return nil, err
	})

	// 清理孤立实体（没有关联Chunk的实体）
	if err == nil {
		_, cleanErr := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			_, err := tx.Run(ctx,
				`MATCH (e:Entity)
				 WHERE NOT (e)-[:MENTIONED_IN]->(:Chunk)
				 DETACH DELETE e`,
				nil,
			)
			return nil, err
		})
		if cleanErr != nil {
			klog.Warnf("清理孤立实体失败: %v", cleanErr)
		}
	}

	return err
}

// GetAllChunksByDoc 获取同一文档的所有Chunk数据
func (g *Neo4jGraph) GetAllChunksByDoc(ctx context.Context) (map[int64][]ChunkData, error) {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		records, err := tx.Run(ctx,
			`MATCH (c:Chunk)
			 RETURN c.chunk_id AS chunk_id, c.kb_id AS kb_id, c.doc_id AS doc_id,
			        c.content AS content, c.chunk_index AS chunk_index, c.source AS source`,
			nil,
		)
		if err != nil {
			return nil, err
		}

		docChunks := make(map[int64][]ChunkData)
		for records.Next(ctx) {
			record := records.Record()
			chunkID, _ := record.Get("chunk_id")
			kbID, _ := record.Get("kb_id")
			docID, _ := record.Get("doc_id")
			content, _ := record.Get("content")
			chunkIndex, _ := record.Get("chunk_index")
			source, _ := record.Get("source")

			docIDVal := toInt64(docID)
			docChunks[docIDVal] = append(docChunks[docIDVal], ChunkData{
				ChunkID:    toString(chunkID),
				KBID:       toInt64(kbID),
				DocID:      docIDVal,
				Content:    toString(content),
				ChunkIndex: toInt(chunkIndex),
				Source:     toString(source),
			})
		}
		return docChunks, nil
	})

	if err != nil {
		return nil, err
	}

	docChunks, ok := result.(map[int64][]ChunkData)
	if !ok {
		return nil, nil
	}
	return docChunks, nil
}

// GetEntityCount 获取实体数量
func (g *Neo4jGraph) GetEntityCount(ctx context.Context) (int, error) {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		records, err := tx.Run(ctx, "MATCH (e:Entity) RETURN count(e) AS cnt", nil)
		if err != nil {
			return 0, err
		}
		if records.Next(ctx) {
			cnt, _ := records.Record().Get("cnt")
			return toInt(cnt), nil
		}
		return 0, nil
	})
	if err != nil {
		return 0, err
	}
	return result.(int), nil
}

// mergeGraphResults 合并图检索结果，去重并截取topK
func mergeGraphResults(direct, expanded []GraphSearchResult, topK int) []GraphSearchResult {
	type chunkKey struct {
		DocID      int64
		ChunkIndex int
	}
	seen := make(map[chunkKey]bool)
	var results []GraphSearchResult

	// 直接匹配结果优先
	for _, r := range direct {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if !seen[key] {
			seen[key] = true
			results = append(results, r)
		}
	}

	// 扩展结果补充
	for _, r := range expanded {
		key := chunkKey{DocID: r.DocID, ChunkIndex: r.ChunkIndex}
		if !seen[key] {
			seen[key] = true
			results = append(results, r)
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

// sanitizeRelType 清理关系类型名称，Neo4j关系类型只允许大写字母、数字和下划线
func sanitizeRelType(relType string) string {
	upper := strings.ToUpper(relType)
	var b strings.Builder
	for _, r := range upper {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' {
			b.WriteRune('_')
		}
	}
	result := b.String()
	if result == "" {
		result = "RELATED_TO"
	}
	return result
}

// 辅助类型转换函数
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func toInt(v interface{}) int {
	return int(toInt64(v))
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	default:
		return 0
	}
}
