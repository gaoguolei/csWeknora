package sandbox

import "context"

// RecoverSessionWorkspace is called by the programming panel, not ordinary file
// tools. It preserves live/paused guests and only recreates a missing Cube guest
// when the bootstrapper can restore this session. Transport/auth failures never
// trigger replacement. Resolve serialises concurrent panel requests and waits
// for AfterCreate (including recovery) before returning.
func (m *SessionBoundManager) RecoverSessionWorkspace(ctx context.Context, sessionID string) error {
	if m.remoteDisabled() || m.client.Provider() != SandboxTypeCube {
		return nil
	}
	recovery, ok := m.lifecycle.bootstrapper.(SessionWorkspaceRecoveryChecker)
	if !ok {
		return nil
	}
	key, err := m.sessionKey(ctx, sessionID)
	if err != nil {
		return err
	}
	return m.bindings.WithLifecycleLock(ctx, key, func(lockCtx context.Context) error {
		// The lock must cover the probe too: another request may have already
		// bound a new guest but still be extracting its backup.
		handle, found, err := m.lookupSessionHandle(lockCtx, sessionID)
		if err != nil {
			return err
		}
		if found {
			if repair, ok := m.lifecycle.bootstrapper.(SessionWorkspaceRepairer); ok {
				return repair.RepairWorkspace(lockCtx, key, handle)
			}
			return nil
		}
		available, err := recovery.CanRecoverWorkspace(lockCtx, key)
		if err != nil || !available {
			return err
		}
		_, err = m.lifecycle.resolveLocked(lockCtx, key)
		return err
	})
}
