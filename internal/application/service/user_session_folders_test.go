package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// Session folders are stored inside one per-user preferences blob while the
// layout itself is per user+workspace, so the merge has to be per workspace
// key. Getting that wrong silently wipes the sidebar organisation of every
// workspace the current request did not mention — this test pins it down.
func TestSessionFoldersPreferenceMergesPerWorkspace(t *testing.T) {
	repo := &switchTenantUserRepo{users: map[string]types.User{"alice": {ID: "alice"}}}
	users := &userService{userRepo: repo}
	ctx := context.WithValue(t.Context(), types.UserIDContextKey, "alice")

	workspace7 := types.SessionFolders{"7": types.SessionFolderState{
		Folders:           []types.SessionFolderEntry{{ID: "f1", Name: "工作", Collapsed: true}},
		Assignments:       map[string]string{"s1": "f1"},
		SortMode:          "recent",
		ProjectsCollapsed: true,
	}}
	prefs, err := users.UpdateUserPreferences(ctx, "alice", types.UserPreferences{SessionFolders: &workspace7})
	require.NoError(t, err)
	require.Equal(t, "工作", (*prefs.SessionFolders)["7"].Folders[0].Name)
	require.True(t, (*prefs.SessionFolders)["7"].Folders[0].Collapsed)
	require.Equal(t, "f1", (*prefs.SessionFolders)["7"].Assignments["s1"])
	require.Equal(t, "recent", (*prefs.SessionFolders)["7"].SortMode)

	// A request only ever carries the workspace the user is looking at, so a
	// patch for workspace 9 must leave workspace 7 untouched.
	workspace9 := types.SessionFolders{"9": types.SessionFolderState{
		Folders: []types.SessionFolderEntry{{ID: "f9", Name: "个人"}},
	}}
	prefs, err = users.UpdateUserPreferences(ctx, "alice", types.UserPreferences{SessionFolders: &workspace9})
	require.NoError(t, err)
	require.Len(t, *prefs.SessionFolders, 2, "a patch for one workspace must not drop the others")
	require.Equal(t, "工作", (*prefs.SessionFolders)["7"].Folders[0].Name)
	require.Equal(t, "个人", (*prefs.SessionFolders)["9"].Folders[0].Name)

	// Re-patching the same workspace replaces it wholesale (that is how a
	// rename or a delete is reported) instead of appending to it.
	renamed := types.SessionFolders{"7": types.SessionFolderState{
		Folders: []types.SessionFolderEntry{{ID: "f2", Name: "重命名后"}},
	}}
	prefs, err = users.UpdateUserPreferences(ctx, "alice", types.UserPreferences{SessionFolders: &renamed})
	require.NoError(t, err)
	require.Len(t, *prefs.SessionFolders, 2)
	require.Equal(t, []types.SessionFolderEntry{{ID: "f2", Name: "重命名后"}}, (*prefs.SessionFolders)["7"].Folders)

	// Omitting the key entirely leaves the stored layout alone.
	prefs, err = users.UpdateUserPreferences(ctx, "alice", types.UserPreferences{})
	require.NoError(t, err)
	require.Len(t, *prefs.SessionFolders, 2)

	// Oversized payloads are rejected before anything is written.
	huge := types.SessionFolders{"7": types.SessionFolderState{Assignments: map[string]string{}}}
	for i := 0; i < 4000; i++ {
		huge["7"].Assignments[fmt.Sprintf("session-%08d", i)] = "folder-00000001"
	}
	_, err = users.UpdateUserPreferences(ctx, "alice", types.UserPreferences{SessionFolders: &huge})
	require.ErrorContains(t, err, "65536")
	require.Len(t, *repo.users["alice"].Preferences.SessionFolders, 2,
		"a rejected patch must not be persisted")

	// Persisted layout survives the same JSON round-trip as the DB column.
	raw, err := repo.users["alice"].Preferences.Value()
	require.NoError(t, err)
	var decoded types.UserPreferences
	require.NoError(t, decoded.Scan(raw))
	require.Equal(t, "重命名后", (*decoded.SessionFolders)["7"].Folders[0].Name)
	require.Equal(t, "f9", (*decoded.SessionFolders)["9"].Folders[0].ID)
}