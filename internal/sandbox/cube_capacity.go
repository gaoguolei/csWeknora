package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
)

// Cube remains the authoritative admission controller across app replicas.
// Only explicit capacity rejection is safe to retry: a network timeout may
// have created a sandbox already, so retrying it could duplicate workspaces.
func cubeCapacityWaitFromEnv() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv("WEKNORA_SANDBOX_CUBE_CAPACITY_WAIT_SECONDS"))
	if raw == "" {
		return time.Minute, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 || seconds > 600 {
		return 0, fmt.Errorf("sandbox: WEKNORA_SANDBOX_CUBE_CAPACITY_WAIT_SECONDS must be an integer from 0 to 600")
	}
	return time.Duration(seconds) * time.Second, nil
}

func waitForCubeCapacity[T any](ctx context.Context, operation string, budget, interval time.Duration, call func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	deadline := time.Now().Add(budget)
	for attempt := 0; ; attempt++ {
		value, err := call(ctx)
		if err == nil {
			return value, nil
		}
		normalized := normalizeCubeError(operation, err)
		var remote *RemoteError
		if budget <= 0 || !errors.As(normalized, &remote) || remote.Kind != RemoteErrorKindCapacity {
			return zero, normalized
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return zero, normalized
		}
		if attempt == 0 {
			logger.Infof(ctx, "[CubeCapacity] %s waiting for available sandbox capacity (budget=%s)", operation, budget)
		}
		delay := interval
		if delay > remaining {
			delay = remaining
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
		if time.Now().After(deadline) {
			return zero, normalized
		}
	}
}
