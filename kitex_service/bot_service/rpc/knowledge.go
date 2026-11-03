package rpc

import (
	"context"
	"fmt"
	"time"

	knowledge "github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge"
	"github.com/Airiseina/answer/kitex_service/knowledge_service/kitex_gen/knowledge/knowledgeservice"
	"github.com/Airiseina/answer/pkg/tenant"

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
	// 该入口仅服务于系统 Bot 的 Skill 知识库，统一归属平台租户。
	return CreateKnowledgeBaseInTenant(ctx, tenant.PlatformTenantID, ownerID, name, description)
}

// CreateKnowledgeBaseInTenant 按指定租户创建知识库，供用户个人 Skill 使用。
func CreateKnowledgeBaseInTenant(ctx context.Context, tenantID, ownerID int64, name, description string) (int64, error) {
	resp, err := knowledgeCli.CreateKnowledgeBase(ctx, &knowledge.CreateKnowledgeBaseReq{
		OwnerId:     ownerID,
		Name:        name,
		Description: &description,
		TenantId:    &tenantID,
	})
	if err != nil {
		return 0, fmt.Errorf("创建知识库失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("创建知识库返回失败")
	}
	return resp.KbId, nil
}

// DeleteKnowledgeBase 删除知识库及其文档与向量，用于 Skill 删除时回收其独占知识库。
func DeleteKnowledgeBase(ctx context.Context, tenantID, kbID, operatorID int64) error {
	resp, err := knowledgeCli.DeleteKnowledgeBase(ctx, &knowledge.DeleteKnowledgeBaseReq{
		KbId:       kbID,
		OperatorId: operatorID,
		TenantId:   &tenantID,
	})
	if err != nil {
		return fmt.Errorf("删除知识库失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("删除知识库返回失败")
	}
	return nil
}

// AddUserDocument 以用户身份向自己名下的知识库添加文档（个人 Skill 的资料文件）。
func AddUserDocument(ctx context.Context, tenantID, kbID, operatorID int64, fileName, fileURL, fileType string, fileSize int64) (int64, error) {
	resp, err := knowledgeCli.AddDocument(ctx, &knowledge.AddDocumentReq{
		KbId:       kbID,
		OperatorId: operatorID,
		FileName:   fileName,
		FileUrl:    fileURL,
		FileType:   fileType,
		FileSize:   fileSize,
		TenantId:   &tenantID,
	})
	if err != nil {
		return 0, fmt.Errorf("知识库添加文档失败: %w", err)
	}
	if !resp.Success {
		return 0, fmt.Errorf("知识库添加文档返回失败")
	}
	return resp.DocId, nil
}

// GetBotKnowledgeBaseIDs 返回 Bot 已绑定的知识库 ID 列表，用于绑定前去重。
func GetBotKnowledgeBaseIDs(ctx context.Context, tenantID, botID int64) ([]int64, error) {
	resp, err := knowledgeCli.GetBotKnowledgeBases(ctx, &knowledge.GetBotKnowledgeBasesReq{
		BotId:    botID,
		TenantId: &tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("查询Bot知识库失败: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("查询Bot知识库返回失败")
	}
	ids := make([]int64, 0, len(resp.KnowledgeBases))
	for _, kb := range resp.KnowledgeBases {
		ids = append(ids, kb.KbId)
	}
	return ids, nil
}

// BindKnowledgeBase 把用户自己的知识库绑定到 Bot，Skill 绑定后即并入既有检索链路。
func BindKnowledgeBase(ctx context.Context, tenantID, botID, operatorID, kbID int64) error {
	resp, err := knowledgeCli.BindKnowledgeBase(ctx, &knowledge.BindKnowledgeBaseReq{
		BotId:      botID,
		OperatorId: operatorID,
		KbId:       kbID,
		TenantId:   &tenantID,
	})
	if err != nil {
		return fmt.Errorf("绑定知识库失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("绑定知识库返回失败")
	}
	return nil
}

// UnbindKnowledgeBase 解除 Bot 与知识库的绑定，用于 Skill 解绑或删除。
func UnbindKnowledgeBase(ctx context.Context, tenantID, botID, operatorID, kbID int64) error {
	resp, err := knowledgeCli.UnbindKnowledgeBase(ctx, &knowledge.UnbindKnowledgeBaseReq{
		BotId:      botID,
		OperatorId: operatorID,
		KbId:       kbID,
		TenantId:   &tenantID,
	})
	if err != nil {
		return fmt.Errorf("解绑知识库失败: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("解绑知识库返回失败")
	}
	return nil
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
