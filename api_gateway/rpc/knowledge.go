package rpc

import (
	"context"
	"time"

	knowledge "github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge/knowledgeservice"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
)

var knowledgeCli knowledgeservice.Client

func ConnectKnowledgeService(r discovery.Resolver) {
	fp := retry.NewFailurePolicy()
	fp.WithMaxRetryTimes(3)
	fp.WithFixedBackOff(100)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.1,
		MinSample: 10,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("knowledge_service", cbConfig)
	c, err := knowledgeservice.NewClient("knowledge_service",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(5*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接knowledge_service失败: %v", err)
	}
	knowledgeCli = c
}

func CreateKnowledgeBase(ctx context.Context, req *knowledge.CreateKnowledgeBaseReq) (*knowledge.CreateKnowledgeBaseRes, error) {
	return knowledgeCli.CreateKnowledgeBase(ctx, req)
}

func GetKnowledgeBase(ctx context.Context, req *knowledge.GetKnowledgeBaseReq) (*knowledge.GetKnowledgeBaseRes, error) {
	return knowledgeCli.GetKnowledgeBase(ctx, req)
}

func GetUserKnowledgeBases(ctx context.Context, req *knowledge.GetUserKnowledgeBasesReq) (*knowledge.GetUserKnowledgeBasesRes, error) {
	return knowledgeCli.GetUserKnowledgeBases(ctx, req)
}

func UpdateKnowledgeBase(ctx context.Context, req *knowledge.UpdateKnowledgeBaseReq) (*knowledge.CommonRes, error) {
	return knowledgeCli.UpdateKnowledgeBase(ctx, req)
}

func DeleteKnowledgeBase(ctx context.Context, req *knowledge.DeleteKnowledgeBaseReq) (*knowledge.CommonRes, error) {
	return knowledgeCli.DeleteKnowledgeBase(ctx, req)
}

func AddDocument(ctx context.Context, req *knowledge.AddDocumentReq) (*knowledge.AddDocumentRes, error) {
	return knowledgeCli.AddDocument(ctx, req)
}

func GetDocuments(ctx context.Context, req *knowledge.GetDocumentsReq) (*knowledge.GetDocumentsRes, error) {
	return knowledgeCli.GetDocuments(ctx, req)
}

func DeleteDocument(ctx context.Context, req *knowledge.DeleteDocumentReq) (*knowledge.CommonRes, error) {
	return knowledgeCli.DeleteDocument(ctx, req)
}

func RetryDocument(ctx context.Context, req *knowledge.RetryDocumentReq) (*knowledge.CommonRes, error) {
	return knowledgeCli.RetryDocument(ctx, req)
}

func BindKnowledgeBase(ctx context.Context, req *knowledge.BindKnowledgeBaseReq) (*knowledge.CommonRes, error) {
	return knowledgeCli.BindKnowledgeBase(ctx, req)
}

func UnbindKnowledgeBase(ctx context.Context, req *knowledge.UnbindKnowledgeBaseReq) (*knowledge.CommonRes, error) {
	return knowledgeCli.UnbindKnowledgeBase(ctx, req)
}

func GetBotKnowledgeBases(ctx context.Context, req *knowledge.GetBotKnowledgeBasesReq) (*knowledge.GetBotKnowledgeBasesRes, error) {
	return knowledgeCli.GetBotKnowledgeBases(ctx, req)
}
