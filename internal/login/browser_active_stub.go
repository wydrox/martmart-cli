//go:build !darwin

package login

import (
	"context"
	"fmt"
)

func runWithExistingBrowser(_ context.Context, _ Options) (*Result, error) {
	return nil, fmt.Errorf("existing-browser Delio login is currently supported on macOS only")
}
