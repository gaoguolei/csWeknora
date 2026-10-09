package service

import (
	"context"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func builtinModelContext(systemAdmin bool) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	return context.WithValue(ctx, types.SystemAdminContextKey, systemAdmin)
}

func TestCreateModel_SystemAdminStoresAsGlobalBuiltin(t *testing.T) {
	repo := &stubModelRepoForDelete{}
	svc := NewModelService(repo, nil, nil, nil, nil, nil)

	model := &types.Model{
		ID: "ui-global-chat", TenantID: 7, Name: "chat", Type: types.ModelTypeKnowledgeQA,
		Source: types.ModelSourceRemote,
	}
	require.NoError(t, svc.CreateModel(builtinModelContext(true), model))

	require.NotNil(t, repo.created)
	assert.True(t, repo.created.IsBuiltin, "system-admin models must be globally visible")
	assert.Equal(t, types.DefaultBuiltinModelTenantID, repo.created.TenantID)
	assert.Empty(t, repo.created.ManagedBy, "UI-created global rows stay deletable")
}

func TestCreateModel_NonSystemAdminKeepsTenantScope(t *testing.T) {
	repo := &stubModelRepoForDelete{}
	svc := NewModelService(repo, nil, nil, nil, nil, nil)

	model := &types.Model{
		ID: "tenant-chat", TenantID: 7, Name: "chat", Type: types.ModelTypeKnowledgeQA,
		Source: types.ModelSourceRemote,
	}
	require.NoError(t, svc.CreateModel(builtinModelContext(false), model))

	require.NotNil(t, repo.created)
	assert.False(t, repo.created.IsBuiltin)
	assert.Equal(t, uint64(7), repo.created.TenantID)
}

func TestCreateModel_DefaultClearsSameTypeBucket(t *testing.T) {
	repo := &stubModelRepoForDelete{}
	svc := NewModelService(repo, nil, nil, nil, nil, nil)

	model := &types.Model{
		ID: "ui-global-embed", TenantID: 7, Name: "embed", Type: types.ModelTypeEmbedding,
		Source: types.ModelSourceRemote, IsDefault: true,
	}
	require.NoError(t, svc.CreateModel(builtinModelContext(true), model))

	require.NotNil(t, repo.clearDefaultCall, "promoting a default must clear the previous one")
	assert.Equal(t, uint(types.DefaultBuiltinModelTenantID), repo.clearDefaultCall.tenantID)
	assert.Equal(t, types.ModelTypeEmbedding, repo.clearDefaultCall.modelType)
	assert.Empty(t, repo.clearDefaultCall.excludeID)
}

func TestDeleteModel(t *testing.T) {
	newSvc := func(stored *types.Model, deleted *string) *modelService {
		repo := &stubModelRepoForDelete{
			model: stored,
			delete: func(id string) error {
				if deleted != nil {
					*deleted = id
				}
				return nil
			},
		}
		svc := NewModelService(
			repo,
			&stubKBRepoForModelDelete{},
			&stubAgentRepoForModelDelete{},
			nil, nil, nil,
		)
		return svc.(*modelService)
	}

	t.Run("system admin deletes UI-created global model", func(t *testing.T) {
		var deleted string
		svc := newSvc(&types.Model{
			ID: "ui-global", TenantID: types.DefaultBuiltinModelTenantID,
			IsBuiltin: true, ManagedBy: "",
		}, &deleted)
		require.NoError(t, svc.DeleteModel(builtinModelContext(true), "ui-global"))
		assert.Equal(t, "ui-global", deleted)
	})

	t.Run("yaml-managed global model is never deletable", func(t *testing.T) {
		svc := newSvc(&types.Model{
			ID: "yaml-model", TenantID: types.DefaultBuiltinModelTenantID,
			IsBuiltin: true, ManagedBy: types.BuiltinModelManagedBy,
		}, nil)
		err := svc.DeleteModel(builtinModelContext(true), "yaml-model")
		require.Error(t, err)
		appErr, ok := apperrors.IsAppError(err)
		require.True(t, ok)
		assert.Equal(t, apperrors.ErrBadRequest, appErr.Code)
	})

	t.Run("non system admin cannot delete global model", func(t *testing.T) {
		svc := newSvc(&types.Model{
			ID: "ui-global", TenantID: types.DefaultBuiltinModelTenantID,
			IsBuiltin: true, ManagedBy: "",
		}, nil)
		err := svc.DeleteModel(builtinModelContext(false), "ui-global")
		require.Error(t, err)
		appErr, ok := apperrors.IsAppError(err)
		require.True(t, ok)
		assert.Equal(t, apperrors.ErrBadRequest, appErr.Code)
	})
}

func TestUpdateBuiltinModel_RequiresSystemAdmin(t *testing.T) {
	stored := &types.Model{
		ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
		ManagedBy: types.BuiltinModelManagedBy,
	}
	updated := false
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(*types.Model) error {
			updated = true
			return nil
		},
	}, nil, nil, nil, nil, nil)

	err := svc.UpdateModel(builtinModelContext(false), &types.Model{ID: stored.ID})
	require.Error(t, err)
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperrors.ErrForbidden, appErr.Code)
	assert.False(t, updated)
}

func TestUpdateBuiltinModel_SystemAdminCreatesRuntimeOverride(t *testing.T) {
	stored := &types.Model{
		ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
		ManagedBy: types.BuiltinModelManagedBy,
	}
	var saved *types.Model
	svc := NewModelService(&stubModelRepoForDelete{
		model: stored,
		update: func(model *types.Model) error {
			copy := *model
			saved = &copy
			return nil
		},
	}, nil, nil, nil, nil, nil)

	input := &types.Model{ID: stored.ID, Name: "edited"}
	require.NoError(t, svc.UpdateModel(builtinModelContext(true), input))
	require.NotNil(t, saved)
	assert.Equal(t, uint64(10000), saved.TenantID)
	assert.True(t, saved.IsBuiltin)
	assert.Empty(t, saved.ManagedBy, "UI edit must stop later YAML reconciliation")
}

func TestUpdateBuiltinModelCredentials_SystemAdminOnly(t *testing.T) {
	newKey := "sk-new"

	t.Run("tenant admin denied", func(t *testing.T) {
		stored := &types.Model{ID: "builtin-chat", TenantID: 10000, IsBuiltin: true}
		svc := NewModelService(&stubModelRepoForDelete{model: stored}, nil, nil, nil, nil, nil)
		_, err := svc.UpdateModelCredentials(builtinModelContext(false), stored.ID, &newKey, nil)
		require.Error(t, err)
		appErr, ok := apperrors.IsAppError(err)
		require.True(t, ok)
		assert.Equal(t, apperrors.ErrForbidden, appErr.Code)
	})

	t.Run("system admin saves runtime override", func(t *testing.T) {
		stored := &types.Model{
			ID: "builtin-chat", TenantID: 10000, IsBuiltin: true,
			ManagedBy: types.BuiltinModelManagedBy,
		}
		var saved *types.Model
		svc := NewModelService(&stubModelRepoForDelete{
			model: stored,
			update: func(model *types.Model) error {
				copy := *model
				saved = &copy
				return nil
			},
		}, nil, nil, nil, nil, nil)

		updated, err := svc.UpdateModelCredentials(
			builtinModelContext(true), stored.ID, &newKey, nil,
		)
		require.NoError(t, err)
		assert.Equal(t, newKey, updated.Parameters.APIKey)
		require.NotNil(t, saved)
		assert.Empty(t, saved.ManagedBy)
	})
}
