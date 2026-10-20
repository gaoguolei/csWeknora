package service

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/types"
)

// Apply the same live ownership and Editor gates as the knowledge file API.
// Search scope and the agent's explicit tool selection are checked separately.
func (s *sessionService) knowledgeFileWriteContext(ctx context.Context, knowledge *types.Knowledge) (context.Context, error) {
	if knowledge == nil || s.knowledgeBaseService == nil {
		return ctx, fmt.Errorf("knowledge file is unavailable")
	}
	kb, err := s.knowledgeBaseService.GetKnowledgeBaseByIDOnly(ctx, knowledge.KnowledgeBaseID)
	if err != nil || kb == nil || kb.ID != knowledge.KnowledgeBaseID || kb.TenantID != knowledge.TenantID {
		return ctx, fmt.Errorf("knowledge base is unavailable")
	}
	caller := types.CallerFromContext(ctx)
	_, apiKey := types.TenantAPIKeyScopeFromContext(ctx)
	user, _ := ctx.Value(types.UserContextKey).(*types.User)
	superuser := user != nil && (user.IsOperationsAdmin ||
		(s.cfg != nil && s.cfg.Tenant != nil && s.cfg.Tenant.EnableCrossTenantAccess && user.CanAccessAllTenants))
	_, err = access.CheckOwnershipOrRole(access.OwnershipRequest{
		UserID: caller.UserID, Role: caller.Role, APIKey: apiKey, CrossTenantSuperuser: superuser,
		Enforce: s.cfg != nil && s.cfg.Tenant.IsRBACEnforced(),
	}, types.TenantRoleAdmin, func() (string, error) {
		return kb.CreatorID, nil
	})
	if err != nil {
		return ctx, fmt.Errorf("no permission to modify this knowledge base")
	}
	grant, err := access.ResolveKB(ctx, access.KBRequest{Caller: caller}, kb, types.OrgRoleEditor, s.kbShareService, nil)
	if err != nil {
		return ctx, fmt.Errorf("no permission to edit this knowledge base")
	}
	scoped := grant.Context(ctx)
	if err := access.RequireKBWrite(scoped, kb); err != nil {
		return ctx, err
	}
	// File storage and the asynchronous parser must use the resource owner's
	// tenant configuration without replacing the authenticated caller.
	if s.tenantService != nil {
		owner, err := s.tenantService.GetTenantByID(ctx, kb.TenantID)
		if err != nil || owner == nil {
			return ctx, fmt.Errorf("knowledge base workspace is unavailable")
		}
		scoped = context.WithValue(scoped, types.TenantInfoContextKey, owner)
	}
	return scoped, nil
}
