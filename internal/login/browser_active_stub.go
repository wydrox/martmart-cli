//go:build !darwin

package login

import (
	"context"
	"fmt"
)

func runWithExistingBrowser(_ context.Context, _ Options) (*Result, error) {
	return nil, fmt.Errorf("existing-browser Delio login is currently supported on macOS only")
}

func ensurePreferredBrowserRemoteDebugOn9222(_ context.Context, _ Options) (string, *remoteDebugBrowserInfo, error) {
	return "", nil, fmt.Errorf("automatic browser restart on remote debug port 9222 is currently supported on macOS only")
}
