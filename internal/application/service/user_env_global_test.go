package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// fakeSystemSetting is the slice of interfaces.SystemSettingService the global
// sandbox secret store touches: GetString reads the persisted blob and Update
// writes it. Everything else is an unused stub. Values are kept in a plain map
// keyed by setting key.
type fakeSystemSetting struct {
	values map[string]string
}

func newFakeSystemSetting() *fakeSystemSetting {
	return &fakeSystemSetting{values: map[string]string{}}
}

func (f *fakeSystemSetting) GetInt(
	_ context.Context, _ string, _ string, def int64,
) int64 {
	return def
}

func (f *fakeSystemSetting) GetString(
	_ context.Context, key string, _ string, def string,
) string {
	if v, ok := f.values[key]; ok {
		return v
	}
	return def
}

func (f *fakeSystemSetting) GetBool(
	_ context.Context, _ string, _ string, def bool,
) bool {
	return def
}

func (f *fakeSystemSetting) GetStringList(
	_ context.Context, _ string, _ string, def []string,
) []string {
	return def
}

func (f *fakeSystemSetting) List(_ context.Context) ([]*types.SystemSetting, error) {
	return nil, nil
}

func (f *fakeSystemSetting) Get(_ context.Context, _ string) (*types.SystemSetting, error) {
	return nil, nil
}

func (f *fakeSystemSetting) Update(
	_ context.Context, key string, rawValue any,
) (*types.SystemSetting, error) {
	s, ok := rawValue.(string)
	if !ok {
		return nil, errors.New("fakeSystemSetting: expected a string value")
	}
	f.values[key] = s
	return &types.SystemSetting{Key: key}, nil
}

func (f *fakeSystemSetting) Reset(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func (f *fakeSystemSetting) SubscribeRedis(_ context.Context) error { return nil }

var _ interfaces.SystemSettingService = (*fakeSystemSetting)(nil)

func newGlobalEnvService() (*UserEnvService, *fakeSystemSetting) {
	settings := newFakeSystemSetting()
	return NewUserEnvService(nil, nil, settings), settings
}

func TestSetGlobalStoresAndListOmitsValue(t *testing.T) {
	ctx := context.Background()
	svc, settings := newGlobalEnvService()

	require.NoError(t, svc.SetGlobal(ctx, "TAVILY_API_KEY", "super-secret"))

	// The persisted blob must carry an entry, but the view returned to the
	// super admin must never include the plaintext value.
	require.NotEmpty(t, settings.values[GlobalEnvKey])

	views, err := svc.ListGlobal(ctx)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, "TAVILY_API_KEY", views[0].Name)
	require.True(t, views[0].HasValue)
	require.NotNil(t, views[0].UpdatedAt)

	// Serialising a view can never leak the secret: there is no value field.
	blob, err := json.Marshal(views)
	require.NoError(t, err)
	require.NotContains(t, string(blob), "super-secret")
	require.NotContains(t, string(blob), "value")
}

func TestSetGlobalRotatesExistingName(t *testing.T) {
	ctx := context.Background()
	svc, _ := newGlobalEnvService()

	require.NoError(t, svc.SetGlobal(ctx, "TOKEN", "first"))
	require.NoError(t, svc.SetGlobal(ctx, "TOKEN", "second"))

	views, err := svc.ListGlobal(ctx)
	require.NoError(t, err)
	require.Len(t, views, 1, "rotating a name does not create a duplicate")

	roundTrip, err := svc.readGlobal(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"TOKEN": "second"}, roundTrip.PlainMap())
}

func TestListGlobalSortedByName(t *testing.T) {
	ctx := context.Background()
	svc, _ := newGlobalEnvService()

	require.NoError(t, svc.SetGlobal(ctx, "ZEBRA", "z"))
	require.NoError(t, svc.SetGlobal(ctx, "ALPHA", "a"))
	require.NoError(t, svc.SetGlobal(ctx, "MIKES", "m"))

	views, err := svc.ListGlobal(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"ALPHA", "MIKES", "ZEBRA"},
		[]string{views[0].Name, views[1].Name, views[2].Name})
}

func TestSetGlobalRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	svc, _ := newGlobalEnvService()

	// Blank value: clearing is a delete, not an empty set.
	require.Error(t, svc.SetGlobal(ctx, "TOKEN", ""))
	// Invalid name (lower-case) is refused by the shared validator.
	require.Error(t, svc.SetGlobal(ctx, "lower_case", "v"))
}

func TestDeleteGlobalRemovesAndReportsNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newGlobalEnvService()

	require.NoError(t, svc.SetGlobal(ctx, "TOKEN", "value"))
	require.NoError(t, svc.DeleteGlobal(ctx, "TOKEN"))

	views, err := svc.ListGlobal(ctx)
	require.NoError(t, err)
	require.Empty(t, views)

	// Deleting a name that is not set surfaces the not-found sentinel.
	require.ErrorIs(t, svc.DeleteGlobal(ctx, "TOKEN"), types.ErrEnvVarNotFound)
}

func TestGlobalRoundTripsThroughPlainMap(t *testing.T) {
	ctx := context.Background()
	svc, _ := newGlobalEnvService()

	require.NoError(t, svc.SetGlobal(ctx, "A_KEY", "a"))
	require.NoError(t, svc.SetGlobal(ctx, "B_KEY", "b"))

	list, err := svc.readGlobal(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"A_KEY": "a", "B_KEY": "b"}, list.PlainMap())
}
