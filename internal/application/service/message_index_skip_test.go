package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubChatHistorySettings overrides GetString so tests can control the
// platform-global embedding model returned for chat_history.embedding_model_id
// without wiring a full system-setting service.
type stubChatHistorySettings struct {
	interfaces.SystemSettingService
	embeddingModelID string
}

func (s *stubChatHistorySettings) GetString(_ context.Context, key string, _ string, def string) string {
	if key == "chat_history.embedding_model_id" {
		return s.embeddingModelID
	}
	return def
}

// newSkipTestService builds a messageService whose global embedding model is
// `model`, so describeChatHistorySkip can be exercised as a method.
func newSkipTestService(model string) *messageService {
	return &messageService{
		systemSettingSvc: &stubChatHistorySettings{embeddingModelID: model},
	}
}

// describeChatHistorySkip now keys off the platform-global embedding model
// first, then the workspace KB. These cases pin the reason string so each skip
// is attributable from the logs.
func TestDescribeChatHistorySkip(t *testing.T) {
	tests := []struct {
		name  string
		model string
		ctx   func() context.Context
		want  string
	}{
		{
			name:  "global model not configured",
			model: "",
			ctx: func() context.Context {
				return contextWithTenant(&types.Tenant{})
			},
			want: "global embedding model not configured (chat_history.embedding_model_id is empty)",
		},
		{
			name:  "no tenant in context",
			model: "emb-1",
			ctx:   func() context.Context { return context.Background() },
			want:  "no tenant in context",
		},
		{
			name:  "model set but config not created",
			model: "emb-1",
			ctx: func() context.Context {
				return contextWithTenant(&types.Tenant{})
			},
			want: "global model set but chat history knowledge base not created yet",
		},
		{
			name:  "model set but knowledge base missing",
			model: "emb-1",
			ctx: func() context.Context {
				return contextWithTenant(&types.Tenant{
					ChatHistoryConfig: &types.ChatHistoryConfig{
						Enabled:          true,
						EmbeddingModelID: "emb-1",
					},
				})
			},
			want: "global model set but chat history knowledge base not created yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newSkipTestService(tt.model)
			if got := svc.describeChatHistorySkip(tt.ctx()); got != tt.want {
				t.Errorf("describeChatHistorySkip() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A workspace already provisioned with a KB for the current global model must
// not be reported as a missing-KB skip: it is the one case where
// IndexMessageToKB proceeds, so the fallback string here would mean the helper
// and getChatHistoryConfig disagree.
func TestDescribeChatHistorySkipFullyConfigured(t *testing.T) {
	cfg := &types.ChatHistoryConfig{
		Enabled:          true,
		EmbeddingModelID: "emb-1",
		KnowledgeBaseID:  "kb-1",
	}
	svc := newSkipTestService("emb-1")
	got := svc.describeChatHistorySkip(contextWithTenant(&types.Tenant{ChatHistoryConfig: cfg}))
	if got != "chat history config incomplete" {
		t.Errorf("unexpected reason for a configured tenant: %q", got)
	}
}

func contextWithTenant(tenant *types.Tenant) context.Context {
	return context.WithValue(context.Background(), types.TenantInfoContextKey, tenant)
}
