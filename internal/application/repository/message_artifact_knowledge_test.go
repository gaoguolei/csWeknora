package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestArtifactKnowledgeLinkPersistsAndSurvivesStaleCollectorWrites(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewMessageRepository(db)
	ctx := context.Background()
	message, err := repo.CreateMessage(ctx, &types.Message{SessionID: "session", Role: "assistant", Artifacts: types.MessageArtifacts{
		testArtifact("cases.xlsx", "/output/cases.xlsx", time.Now()),
		testArtifact("other.xlsx", "/output/other.xlsx", time.Now()),
	}})
	require.NoError(t, err)
	stale, err := repo.GetMessage(ctx, "session", message.ID)
	require.NoError(t, err)
	require.NoError(t, repo.LinkArtifactKnowledge(ctx, "session", message.ID, 0, "knowledge", "kb"))
	require.NoError(t, repo.UpdateMessage(ctx, stale))
	loaded, err := NewMessageRepository(db).GetMessage(ctx, "session", message.ID)
	require.NoError(t, err)
	require.Equal(t, "knowledge", loaded.Artifacts[0].KnowledgeID)
	require.Equal(t, "kb", loaded.Artifacts[0].KnowledgeBaseID)
	require.Empty(t, loaded.Artifacts[1].KnowledgeID)
	require.Equal(t, message.Artifacts[0].URL, loaded.Artifacts[0].URL, "linking does not overwrite the original stored bytes")
	require.ErrorIs(t, repo.LinkArtifactKnowledge(ctx, "other-session", message.ID, 0, "wrong", "kb"), gorm.ErrRecordNotFound)
	_, err = repo.SoftDeleteSessionArtifacts(ctx, "session", []types.ArtifactRef{{MessageID: message.ID, Position: 0}}, time.Now())
	require.NoError(t, err)
	require.ErrorIs(t, repo.LinkArtifactKnowledge(ctx, "session", message.ID, 0, "wrong", "kb"), gorm.ErrRecordNotFound)
}

func TestArtifactKnowledgeLinkDoesNotMoveToAReplacementAtTheSameIndex(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewMessageRepository(db)
	ctx := context.Background()
	message, err := repo.CreateMessage(ctx, &types.Message{SessionID: "session", Role: "assistant", Artifacts: types.MessageArtifacts{
		testArtifact("old.xlsx", "/output/old.xlsx", time.Now()),
	}})
	require.NoError(t, err)
	require.NoError(t, repo.LinkArtifactKnowledge(ctx, "session", message.ID, 0, "knowledge", "kb"))
	message.Artifacts = types.MessageArtifacts{testArtifact("different.xlsx", "/output/different.xlsx", time.Now())}
	require.NoError(t, repo.UpdateMessage(ctx, message))
	loaded, err := repo.GetMessage(ctx, "session", message.ID)
	require.NoError(t, err)
	require.Empty(t, loaded.Artifacts[0].KnowledgeID)
}
