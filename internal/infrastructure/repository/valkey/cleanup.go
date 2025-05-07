package valkey

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"
)

// StartPeriodicCleanup removes keys from sorted set when lobbies are expired/deleted.
//
// Since there is no expiration for sorted sets in valkey/redis, this workaround is needed.
// Not an ideal solution, but it should work fine on thousands of entries, if not triggered too often.
func (r *Repository) StartPeriodicCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				slog.Debug("cleaning up orphaned lobbies...")

				if err := r.cleanupLobbiesSortedSet(ctx); err != nil {
					slog.Error("failed to cleanup orphaned lobbies", "error", err)
				}

				slog.Debug("done cleaning up orphaned lobbies")
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *Repository) cleanupLobbiesSortedSet(ctx context.Context) error {
	// 1. Fetch all lobby IDs
	lobbiesCmd := r.valkey.B().Zrevrange().Key(lobbiesPrefix).Start(0).Stop(-1)

	lobbyIDs, err := r.valkey.Do(ctx, lobbiesCmd.Build()).AsStrSlice()
	if err != nil {
		return fmt.Errorf("failed to get lobby IDs: %w", err)
	}

	// 2. Batch EXISTS checks
	cmds := make([]valkey.Completed, 0, len(lobbyIDs))
	for _, id := range lobbyIDs {
		cmds = append(cmds, r.valkey.B().Exists().Key(lobbyPrefix+id).Build())
	}

	existsResults := r.valkey.DoMulti(ctx, cmds...)

	// 3. Collect IDs to remove
	toRemove := make([]string, 0)

	for i, res := range existsResults {
		exists, err := res.AsBool()
		if err != nil {
			slog.Debug("failed to check lobby existence", slog.Any("error", err))
			continue
		}

		if !exists {
			toRemove = append(toRemove, lobbyIDs[i])
		}
	}

	// 4. Batch remove from sorted set (if any orphans are found)
	if len(toRemove) > 0 {
		slog.Debug("found orphaned lobbies, deleting", "count", len(toRemove))

		cmd := r.valkey.B().Zrem().Key(lobbiesPrefix).Member(toRemove...).Build()

		if err := r.valkey.Do(ctx, cmd).Error(); err != nil {
			return fmt.Errorf("failed to remove orphaned lobbies: %w", err)
		}
	}

	return nil
}
