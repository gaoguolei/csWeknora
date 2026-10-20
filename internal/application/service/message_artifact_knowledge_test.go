package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type artifactLinkRepo struct {
	interfaces.MessageRepository
	called bool
}

func (r *artifactLinkRepo) FindSessionArtifact(context.Context, string, string, int) (*types.MessageArtifactRecord, error) {
	return &types.MessageArtifactRecord{}, nil
}
func (r *artifactLinkRepo) LinkArtifactKnowledge(context.Context, string, string, int, string, string) error {
	r.called = true
	return nil
}

type artifactLinkKnowledge struct {
	interfaces.KnowledgeService
	tenant uint64
}

func (s *artifactLinkKnowledge) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	return &types.Knowledge{ID: "knowledge", KnowledgeBaseID: "kb", TenantID: s.tenant, Type: "file"}, nil
}

type artifactLinkKB struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s *artifactLinkKB) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

type artifactLinkShare struct{}

func (artifactLinkShare) CheckTenantKBPermission(context.Context, string, uint64, types.TenantRole) (types.OrgMemberRole, bool, error) {
	return types.OrgRoleEditor, true, nil
}

func TestArtifactKnowledgeLinkChecksBothSessionOwnershipAndKnowledgeGrant(t *testing.T) {
	for _, mode := range []string{"cross-tenant", "no-knowledge-grant", "not-session-owner"} {
		t.Run(mode, func(t *testing.T) {
			ctx := types.WithCaller(deleteCallerCtx(), types.Caller{TenantID: 7, UserID: "alice", Role: types.TenantRoleAdmin})
			kb := &types.KnowledgeBase{ID: "kb", TenantID: 9}
			grant, err := access.ResolveKB(ctx, access.KBRequest{Caller: types.CallerFromContext(ctx)}, kb, types.OrgRoleEditor, artifactLinkShare{}, nil)
			require.NoError(t, err)
			if mode != "no-knowledge-grant" {
				ctx = grant.Context(ctx)
			}
			sessions := &deleteSessionRepo{}
			if mode == "not-session-owner" {
				sessions.err = errors.ErrSessionNotFound
			}
			repo := &artifactLinkRepo{}
			svc := &messageService{sessionRepo: sessions, messageRepo: repo, knowService: &artifactLinkKnowledge{tenant: 9}, kbService: &artifactLinkKB{kb: kb}}
			err = svc.LinkArtifactKnowledge(ctx, "session", "message", 0, "knowledge")
			if mode == "cross-tenant" {
				require.NoError(t, err)
				require.True(t, repo.called)
			} else {
				require.Error(t, err)
				require.False(t, repo.called)
			}
			require.Equal(t, uint64(7), sessions.gotTenant, "source session belongs to the caller tenant, even after KB middleware changes execution scope")
			require.Equal(t, "alice", sessions.gotUser)
		})
	}
}
