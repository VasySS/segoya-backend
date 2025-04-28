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
		"valkey/valkey:8.1.1-alpine3.21@sha256:f3959d30d4aa6df4fe7468c6b17d103e56f0fc7a4246f32d8106991b3665cdb9",
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
