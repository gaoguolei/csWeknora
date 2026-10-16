package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"time"
)

type workspaceTestRunRepository struct{ db *gorm.DB }

func mustTestJSON(value any) string { data, _ := json.Marshal(value); return string(data) }

func NewWorkspaceTestRunRepository(db *gorm.DB) interfaces.WorkspaceTestRunRepository {
	return &workspaceTestRunRepository{db: db}
}

func (r *workspaceTestRunRepository) Create(ctx context.Context, run *types.WorkspaceTestRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *workspaceTestRunRepository) Save(ctx context.Context, run *types.WorkspaceTestRun) error {
	status := gorm.Expr("CASE WHEN cancel_requested = ? AND ? IN ('queued','preparing','running','cancelling') THEN 'cancelling' ELSE ? END", true, run.Status, run.Status)
	return r.db.WithContext(ctx).Model(&types.WorkspaceTestRun{}).Where("id = ? AND tenant_id = ? AND user_id = ? AND session_id = ?", run.ID, run.TenantID, run.UserID, run.SessionID).Updates(map[string]any{"status": status, "runner_started": run.RunnerStarted, "sandbox_id": run.SandboxID, "result": mustTestJSON(run.Result), "report_message_id": run.ReportMessageID, "report_error": run.ReportError, "updated_at": run.UpdatedAt}).Error
}

func (r *workspaceTestRunRepository) RequestCancel(ctx context.Context, run *types.WorkspaceTestRun) error {
	return r.db.WithContext(ctx).Model(&types.WorkspaceTestRun{}).Where("id = ? AND tenant_id = ? AND user_id = ? AND session_id = ? AND status IN ?", run.ID, run.TenantID, run.UserID, run.SessionID, []string{"queued", "preparing", "running", "cancelling"}).Updates(map[string]any{"cancel_requested": true, "status": "cancelling", "updated_at": time.Now().UTC()}).Error
}

func (r *workspaceTestRunRepository) Get(ctx context.Context, tenant uint64, user, session, id string) (*types.WorkspaceTestRun, error) {
	var run types.WorkspaceTestRun
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND session_id = ? AND id = ?", tenant, user, session, id).First(&run).Error
	return &run, err
}

func (r *workspaceTestRunRepository) List(ctx context.Context, tenant uint64, user, session string) ([]types.WorkspaceTestRun, error) {
	var runs []types.WorkspaceTestRun
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND session_id = ?", tenant, user, session).Order("created_at DESC").Limit(30).Find(&runs).Error
	return runs, err
}

func (r *workspaceTestRunRepository) Active(ctx context.Context, tenant uint64, session string) (*types.WorkspaceTestRun, error) {
	var run types.WorkspaceTestRun
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND status IN ?", tenant, session, []string{"queued", "preparing", "running", "cancelling"}).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &run, err
}

// The claim, assistant message and artifact rows commit together. Message's
// BeforeCreate assigns its real ID; callers must never assume run_id equals it.
func (r *workspaceTestRunRepository) PublishReport(ctx context.Context, run *types.WorkspaceTestRun, artifacts types.MessageArtifacts) (id string, created bool, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claim := tx.Model(&types.WorkspaceTestRun{}).Where("id = ? AND tenant_id = ? AND user_id = ? AND session_id = ? AND report_message_id = ''", run.ID, run.TenantID, run.UserID, run.SessionID).Update("report_message_id", "publishing")
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			var existing types.WorkspaceTestRun
			if err := tx.Where("id = ? AND tenant_id = ? AND user_id = ? AND session_id = ?", run.ID, run.TenantID, run.UserID, run.SessionID).First(&existing).Error; err != nil {
				return err
			}
			id = existing.ReportMessageID
			return nil
		}
		message := &types.Message{SessionID: run.SessionID, RequestID: run.ID, Role: "assistant", Channel: "workspace_test", Content: fmt.Sprintf("项目验证已结束：%s。测试报告已保存到产物。", run.Result.Outcome), IsCompleted: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), Artifacts: artifacts}
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		if err := insertMessageArtifacts(tx, []*types.Message{message}); err != nil {
			return err
		}
		id, created = message.ID, true
		return tx.Model(&types.WorkspaceTestRun{}).Where("id = ?", run.ID).Updates(map[string]any{"report_message_id": id, "report_error": "", "updated_at": time.Now().UTC()}).Error
	})
	return
}
