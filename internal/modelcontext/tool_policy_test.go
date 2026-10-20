package modelcontext

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeFileEditsDecodeOnlyTheDocumentSelector(t *testing.T) {
	registry := NewRegistry(true)
	registry.RegisterDocument("document-uuid")
	calls := []types.LLMToolCall{{Function: types.FunctionCall{
		Name:      "edit_knowledge_file",
		Arguments: `{"knowledge_id":"d1","action":"edit","edits":[{"old_string":"d1","new_string":"d2"}]}`,
	}}}
	registry.DecodeToolCalls(calls)
	require.JSONEq(t,
		`{"knowledge_id":"document-uuid","action":"edit","edits":[{"old_string":"d1","new_string":"d2"}]}`,
		calls[0].Function.Arguments)
}

// Every key a tool declares as a source-ID argument must have a registration
// space, otherwise its values could be decoded from handles but never
// registered (or vice versa) — the drift this table exists to prevent.
func TestEverySourceIDKeyHasARegistrationSpace(t *testing.T) {
	for tool, policy := range toolHandlePolicies {
		for key := range policy.sourceIDKeys {
			if _, ok := sourceKeySpaces[key]; !ok {
				t.Errorf("tool %s declares source key %q with no entry in sourceKeySpaces", tool, key)
			}
		}
		for key := range policy.sourceTextKeys {
			if _, ok := sourceKeySpaces[key]; ok {
				t.Errorf("tool %s declares %q as free text but sourceKeySpaces also claims it as an ID key", tool, key)
			}
		}
	}
}
