package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
)

func (s *messageService) LinkArtifactKnowledge(ctx context.Context, sessionID, messageID string, index int, knowledgeID string) error {
	if sessionID == "" || messageID == "" || index < 0 || knowledgeID == "" {
		return errors.NewBadRequestError("session, message, artifact index and knowledge are required")
	}
	// KB middleware may scope execution to a different tenant. Session writes
	// must still use the authenticated caller's tenant and strict owner scope.
	caller := types.CallerFromContext(ctx)
	sessionCtx := types.WithExecutionTenant(ctx, caller.TenantID)
	if _, err := s.sessionRepo.Get(sessionCtx, caller.TenantID, types.SessionOwnerIDFromContext(sessionCtx), sessionID); err != nil {
		return err
	}
	artifact, err := s.messageRepo.FindSessionArtifact(sessionCtx, sessionID, messageID, index)
	if err != nil {
		return err
	}
	if artifact == nil {
		return ErrArtifactNotFound
	}
	knowledge, err := s.knowService.GetKnowledgeByIDOnly(ctx, knowledgeID)
	if err != nil {
		return err
	}
	if knowledge == nil || knowledge.Type != "file" {
		return errors.NewBadRequestError("only file knowledge can be linked")
	}
	kb, err := knowledgeWriteKB(ctx, s.kbService, knowledge)
	if err != nil {
		return err
	}
	if err := access.RequireKBWrite(ctx, kb); err != nil {
		return errors.NewForbiddenError("knowledge is not writable")
	}
	return s.messageRepo.LinkArtifactKnowledge(sessionCtx, sessionID, messageID, index, knowledge.ID, kb.ID)
}
