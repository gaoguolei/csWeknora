package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type storageBackendRepository struct{ db *gorm.DB }

func NewStorageBackendRepository(db *gorm.DB) interfaces.StorageBackendRepository {
	return &storageBackendRepository{db: db}
}

func (r *storageBackendRepository) Create(ctx context.Context, backend *types.StorageBackend) error {
	return r.db.WithContext(ctx).Create(backend).Error
}

func (r *storageBackendRepository) GetByID(ctx context.Context, _ uint64, id string) (*types.StorageBackend, error) {
	var backend types.StorageBackend
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&backend).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &backend, nil
}

// List returns the backends every workspace should see: the platform-wide
// user-configured instances plus this workspace's own environment/legacy rows.
//
// Legacy aliases are per-workspace artifacts of the storage migration (one row
// per workspace pointing at the process-wide STORAGE_TYPE target), so listing
// them platform-wide would repeat the same "System LOCAL" entry once per
// workspace. Keeping them scoped to their owner leaves one visible copy each
// while the real instances stay shared.
func (r *storageBackendRepository) List(ctx context.Context, tenantID uint64) ([]*types.StorageBackend, error) {
	var backends []*types.StorageBackend
	err := r.db.WithContext(ctx).
		Where("legacy_alias = ? OR tenant_id = ?", false, tenantID).
		Order("created_at DESC").Find(&backends).Error
	return backends, err
}

func (r *storageBackendRepository) Update(ctx context.Context, backend *types.StorageBackend) error {
	return r.db.WithContext(ctx).Model(&types.StorageBackend{}).
		Where("id = ?", backend.ID).
		Select("name", "config", "status", "updated_at").Updates(backend).Error
}

func (r *storageBackendRepository) Delete(ctx context.Context, _ uint64, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&types.StorageBackend{}).Error
}

// FindLegacyAlias stays workspace-scoped on purpose.
//
// A legacy alias is the per-workspace row that keeps env-only deployments on the
// instance-resolution path. Resolving it globally would hand a workspace another
// workspace's alias for the same provider, which means the wrong bucket or path
// prefix for every file written through the env fallback.
func (r *storageBackendRepository) FindLegacyAlias(ctx context.Context, tenantID uint64, provider string) (*types.StorageBackend, error) {
	var backend types.StorageBackend
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND provider = ? AND legacy_alias = ?", tenantID, provider, true).First(&backend).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &backend, nil
}
