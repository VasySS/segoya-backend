package containers

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go/modules/valkey"
)

// ValkeyContainer is a wrapper around the valkey test container.
type ValkeyContainer struct {
	*valkey.ValkeyContainer
	ConnectionString string
}

// NewValkeyContainer creates a new valkey test container.
func NewValkeyContainer(ctx context.Context) (*ValkeyContainer, error) {
	valkeyContainer, err := valkey.Run(ctx,
		"valkey/valkey:8.0.2",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to run valkey container: %w", err)
	}

	connString, err := valkeyContainer.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get valkey connection string: %w", err)
	}

	return &ValkeyContainer{
		ValkeyContainer:  valkeyContainer,
		ConnectionString: connString,
	}, nil
}
