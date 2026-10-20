package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type knowledgeFileShare struct {
	interfaces.KBShareService
	permission types.OrgMemberRole
}

func (s *knowledgeFileShare) CheckTenantKBPermission(context.Context, string, uint64, types.TenantRole) (types.OrgMemberRole, bool, error) {
	return s.permission, true, nil
}

func TestKnowledgeFileWriteContextPreservesCallerAndEnforcesOwnership(t *testing.T) {
	enforced := true
	kb := &types.KnowledgeBase{ID: "kb", TenantID: 7, CreatorID: "creator"}
	svc := &sessionService{
		cfg:                  &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enforced}},
		knowledgeBaseService: &toolSurfaceKBService{fakeAgentKnowledgeBaseService{kb: kb}},
	}
	knowledge := &types.Knowledge{ID: "doc", KnowledgeBaseID: "kb", TenantID: 7}
	for _, test := range []struct {
		user string
		role types.TenantRole
		want bool
	}{
		{"creator", types.TenantRoleContributor, true},
		{"other", types.TenantRoleContributor, false},
		{"admin", types.TenantRoleAdmin, true},
	} {
		ctx := types.WithCaller(t.Context(), types.Caller{TenantID: 7, UserID: test.user, Role: test.role})
		scoped, err := svc.knowledgeFileWriteContext(ctx, knowledge)
		if !test.want {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.NoError(t, access.RequireKBWrite(scoped, kb))
		require.Equal(t, types.CallerFromContext(ctx), types.CallerFromContext(scoped))
		require.Equal(t, uint64(7), types.MustTenantIDFromContext(scoped))
	}
}

func TestKnowledgeFileWriteContextRechecksSharedPermissionAndAPIKeyScope(t *testing.T) {
	enforced := true
	kb := &types.KnowledgeBase{ID: "shared", TenantID: 8, CreatorID: "source-owner"}
	shares := &knowledgeFileShare{permission: types.OrgRoleEditor}
	svc := &sessionService{
		cfg:                  &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enforced}},
		knowledgeBaseService: &toolSurfaceKBService{fakeAgentKnowledgeBaseService{kb: kb}},
		kbShareService:       shares,
	}
	ctx := types.WithCaller(t.Context(), types.Caller{TenantID: 7, UserID: "admin", Role: types.TenantRoleAdmin})
	knowledge := &types.Knowledge{ID: "doc", KnowledgeBaseID: "shared", TenantID: 8}
	scoped, err := svc.knowledgeFileWriteContext(ctx, knowledge)
	require.NoError(t, err)
	require.Equal(t, uint64(8), types.MustTenantIDFromContext(scoped))
	require.Equal(t, uint64(7), types.CallerFromContext(scoped).TenantID)
	require.NoError(t, access.RequireKBWrite(scoped, kb))
	shares.permission = types.OrgRoleViewer
	_, err = svc.knowledgeFileWriteContext(ctx, knowledge)
	require.Error(t, err, "a previously writable share must not survive a permission change")
	shares.permission = types.OrgRoleEditor
	ctx = types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{KnowledgeBaseIDs: []string{"shared"}})
	_, err = svc.knowledgeFileWriteContext(ctx, knowledge)
	require.Error(t, err, "a read-only API key must not gain ingest through the agent tool")
}
