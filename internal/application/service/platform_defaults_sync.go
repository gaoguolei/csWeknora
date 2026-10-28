package service

import (
	"context"
	"reflect"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// PlatformDefaultsSyncService fans a saved platform default out to every
// workspace that already exists.
//
// The agent editor no longer exposes 模型 / 多轮对话 / 检索策略 / 网络搜索 / 附件上传
// (项目经理第 11/12 条), and knowledge-base models are maintained centrally as
// well, so a super admin's edit has to reach objects created earlier — not just
// the next one. Callers run this after the system setting row is persisted; a
// failure here does not roll the setting back, it is reported to the operator.
type PlatformDefaultsSyncService struct {
	tenantSvc     interfaces.TenantService
	agentRepo     interfaces.CustomAgentRepository
	kbRepo        interfaces.KnowledgeBaseRepository
	knowledgeRepo interfaces.KnowledgeRepository
}

// PlatformDefaultsSyncResult summarises what one sync touched.
type PlatformDefaultsSyncResult struct {
	Tenants        int `json:"tenants"`
	Agents         int `json:"agents"`
	KnowledgeBases int `json:"knowledge_bases"`
	Knowledges     int `json:"knowledges"`
}

// NewPlatformDefaultsSyncService creates the sync helper.
func NewPlatformDefaultsSyncService(
	tenantSvc interfaces.TenantService,
	agentRepo interfaces.CustomAgentRepository,
	kbRepo interfaces.KnowledgeBaseRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
) *PlatformDefaultsSyncService {
	return &PlatformDefaultsSyncService{
		tenantSvc:     tenantSvc,
		agentRepo:     agentRepo,
		kbRepo:        kbRepo,
		knowledgeRepo: knowledgeRepo,
	}
}

// ApplyAgentDefaults overrides the platform-managed fields on every agent row in
// every tenant. Fields the operator left unset are preserved, and agents whose
// config would not change are skipped, so the call is idempotent.
//
// Built-in agents that have no database row are synthesized from code and are
// intentionally left alone: there is nothing to write, and their behaviour is
// defined by the built-in definition rather than a stored config.
func (s *PlatformDefaultsSyncService) ApplyAgentDefaults(
	ctx context.Context, defaults *types.AgentDefaultsConfig,
) (PlatformDefaultsSyncResult, error) {
	var result PlatformDefaultsSyncResult
	if s == nil || defaults == nil {
		return result, nil
	}

	tenants, err := s.tenantSvc.ListAllTenants(ctx)
	if err != nil {
		return result, err
	}
	result.Tenants = len(tenants)

	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		agents, err := s.agentRepo.ListAgentsByTenantID(ctx, tenant.ID)
		if err != nil {
			logger.Warnf(ctx, "platform defaults: list agents of tenant %d failed: %v", tenant.ID, err)
			continue
		}
		for _, agent := range agents {
			if agent == nil {
				continue
			}
			before := agent.Config
			defaults.OverrideTo(&agent.Config)
			if reflect.DeepEqual(before, agent.Config) {
				continue
			}
			if err := s.agentRepo.UpdateAgentConfig(ctx, agent.ID, agent.TenantID, agent.Config); err != nil {
				logger.Warnf(ctx, "platform defaults: update agent %s failed: %v", agent.ID, err)
				continue
			}
			result.Agents++
		}
	}
	return result, nil
}

// ApplyKBModelDefaults rewrites the summary / embedding model of every existing
// knowledge base (and the embedding model recorded on its knowledge rows).
//
// An empty model ID leaves that role untouched. NOTE: switching the embedding
// model of a knowledge base that already holds vectors leaves those vectors in
// the previous model's namespace — retrieval for them returns nothing until the
// content is re-parsed. The UI warns about this before the setting is saved.
func (s *PlatformDefaultsSyncService) ApplyKBModelDefaults(
	ctx context.Context, summaryModelID, embeddingModelID string,
) (PlatformDefaultsSyncResult, error) {
	var result PlatformDefaultsSyncResult
	if s == nil || (summaryModelID == "" && embeddingModelID == "") {
		return result, nil
	}

	tenants, err := s.tenantSvc.ListAllTenants(ctx)
	if err != nil {
		return result, err
	}
	result.Tenants = len(tenants)

	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		updated, err := s.kbRepo.UpdateModelReferences(ctx, tenant.ID, summaryModelID, embeddingModelID)
		if err != nil {
			logger.Warnf(ctx, "platform defaults: update KB models of tenant %d failed: %v", tenant.ID, err)
		} else {
			result.KnowledgeBases += int(updated)
		}
		knowledges, err := s.knowledgeRepo.UpdateEmbeddingModelForTenant(ctx, tenant.ID, embeddingModelID)
		if err != nil {
			logger.Warnf(ctx, "platform defaults: update knowledge embedding model of tenant %d failed: %v", tenant.ID, err)
			continue
		}
		result.Knowledges += int(knowledges)
	}
	return result, nil
}
