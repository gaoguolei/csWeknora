package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An agent with no model_id is only rejected when the workspace has no chat
// model to inherit; "no models at all" is a real configuration gap.
func TestResolveChatModelIDRejectsAgentWithoutModelWhenNoChatModelExists(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"builtin-chat": {
					ID:   "builtin-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
			// ListModels is what the default lookup reads; keep it empty.
			availableModels: []*types.Model{},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.Error(t, err)
	assert.Empty(t, modelID)
	assert.Contains(t, err.Error(), "model_id")
}

// The declared default wins over creation order, so the platform default the
// administrator picked is what an unbound agent actually runs on.
func TestResolveChatModelIDUnboundAgentInheritsDefaultChatModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"default-chat": {ID: "default-chat", Type: types.ModelTypeKnowledgeQA},
			},
			availableModels: []*types.Model{
				{ID: "other-chat", Type: types.ModelTypeKnowledgeQA},
				{ID: "default-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true},
				{ID: "default-embed", Type: types.ModelTypeEmbedding, IsDefault: true},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID:     "agent-1",
			Config: types.CustomAgentConfig{ModelID: ""},
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "default-chat", modelID)
}

// Without an explicit default, the first usable model of the type takes over;
// a model that is still downloading must not be picked.
func TestResolveChatModelIDUnboundAgentFallsBackToFirstActiveChatModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"active-chat": {ID: "active-chat", Type: types.ModelTypeKnowledgeQA},
			},
			availableModels: []*types.Model{
				{ID: "pulling-chat", Type: types.ModelTypeKnowledgeQA, Status: types.ModelStatusDownloading},
				{ID: "active-chat", Type: types.ModelTypeKnowledgeQA, Status: types.ModelStatusActive},
			},
		},
	}
	req := &types.QARequest{
		Session:     &types.Session{},
		CustomAgent: &types.CustomAgent{ID: "agent-1"},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "active-chat", modelID)
}

// A stale binding is a misconfiguration, not an invitation to silently switch
// models: it must still fail instead of inheriting the default.
func TestResolveChatModelIDRejectsUnavailableConfiguredAgentModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID:      map[string]*types.Model{},
			availableModels: []*types.Model{{ID: "some-chat", Type: types.ModelTypeKnowledgeQA}},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "deleted-model",
			},
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.Error(t, err)
	assert.Empty(t, modelID)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestResolveChatModelIDUsesValidConfiguredAgentModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat": {
					ID:   "agent-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "agent-chat",
			},
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "agent-chat", modelID)
}

func TestResolveChatModelIDRejectsNonChatSummaryModelOverride(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat": {
					ID:   "agent-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
				"rerank-only": {
					ID:   "rerank-only",
					Type: types.ModelTypeRerank,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "agent-chat",
			},
		},
		SummaryModelID: "rerank-only",
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "agent-chat", modelID)
}

func TestResolveChatModelIDUsesValidSummaryModelOverride(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat": {
					ID:   "agent-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
				"override-chat": {
					ID:   "override-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "agent-chat",
			},
		},
		SummaryModelID: "override-chat",
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "override-chat", modelID)
}

func TestResolveChatModelIDWikiFixerFallsBackToKnowledgeBaseModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"wiki-chat": {
					ID:   "wiki-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
		knowledgeBaseService: &fakeAgentKnowledgeBaseService{
			kb: &types.KnowledgeBase{
				ID:             "wiki-kb",
				SummaryModelID: "wiki-chat",
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: types.BuiltinWikiFixerID,
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, []string{"wiki-kb"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "wiki-chat", modelID)
}

func TestResolveChatModelIDWikiFixerFallsBackToAvailableModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			availableModels: []*types.Model{
				{
					ID:   "system-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: types.BuiltinWikiFixerID,
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "system-chat", modelID)
}

// A shared agent executes in its owner's workspace; the override would pick
// any of the owner's models there, so the configured model is used instead.
func TestResolveChatModelIDIgnoresOverrideForSharedAgents(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat":        {ID: "agent-chat", Type: types.ModelTypeKnowledgeQA},
				"owner-other-model": {ID: "owner-other-model", Type: types.ModelTypeKnowledgeQA},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID:     "agent-1",
			Config: types.CustomAgentConfig{ModelID: "agent-chat"},
		},
		SummaryModelID:      "owner-other-model",
		SharedAgentReadOnly: true,
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "agent-chat", modelID)
}

// Quick-answer mode follows the same rule as agent mode: the request switch
// only opts a turn in and cannot enable search the agent turns off.
func TestResolveWebSearchEnabled(t *testing.T) {
	agent := func(enabled bool) *types.CustomAgent {
		return &types.CustomAgent{Config: types.CustomAgentConfig{WebSearchEnabled: enabled}}
	}
	assert.False(t, resolveWebSearchEnabled(&types.QARequest{CustomAgent: agent(false), WebSearchEnabled: true}))
	assert.False(t, resolveWebSearchEnabled(&types.QARequest{CustomAgent: agent(true), WebSearchEnabled: false}))
	assert.True(t, resolveWebSearchEnabled(&types.QARequest{CustomAgent: agent(true), WebSearchEnabled: true}))
	assert.True(t, resolveWebSearchEnabled(&types.QARequest{WebSearchEnabled: true}), "no agent keeps the switch")
}
