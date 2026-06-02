package rpc

import (
	"context"
	"fmt"
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
	fp.WithMaxRetryTimes(2)
	fp.WithFixedBackOff(300)
	cbConfig := circuitbreak.CBConfig{
		Enable:    true,
		ErrRate:   0.5,
		MinSample: 30,
	}
	cbs := circuitbreak.NewCBSuite(circuitbreak.RPCInfo2Key)
	cbs.UpdateServiceCBConfig("knowledge_service", cbConfig)
	c, err := knowledgeservice.NewClient("knowledge_service",
		client.WithResolver(r),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithFailureRetry(fp),
		client.WithRPCTimeout(10*time.Second),
		client.WithCircuitBreaker(cbs),
		client.WithLoadBalancer(loadbalance.NewWeightedRoundRobinBalancer()),
	)
	if err != nil {
		klog.Fatalf("连接knowledge_service失败: %v", err)
	}
	knowledgeCli = c
}

func CreateKnowledgeBase(ctx context.Context, ownerID int64, name, description string) (int64, error) {
	resp, err := knowledgeCli.CreateKnowledgeBase(ctx, &knowledge.CreateKnowledgeBaseReq{
		OwnerId:     ownerID,
		Name:        name,
		Description: &description,
	})
	if err != nil {
		return 0, fmt.Errorf("创建知识库失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("创建知识库返回失败")
	}
	return resp.KbId, nil
}

func BindSystemKnowledgeBase(ctx context.Context, botID, kbID int64) error {
	resp, err := knowledgeCli.BindSystemKnowledgeBase(ctx, &knowledge.BindSystemKnowledgeBaseReq{
		BotId: botID,
		KbId:  kbID,
	})
	if err != nil {
		return fmt.Errorf("系统Bot绑定知识库失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("系统Bot绑定知识库返回失败")
	}
	return nil
}

func AddSystemDocument(ctx context.Context, kbID int64, fileName, fileURL, fileType string, fileSize int64) (int64, error) {
	resp, err := knowledgeCli.AddSystemDocument(ctx, &knowledge.AddSystemDocumentReq{
		KbId:     kbID,
		FileName: fileName,
		FileUrl:  fileURL,
		FileType: fileType,
		FileSize: fileSize,
	})
	if err != nil {
		return 0, fmt.Errorf("系统知识库添加文档失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("系统知识库添加文档返回失败")
	}
	return resp.DocId, nil
}
