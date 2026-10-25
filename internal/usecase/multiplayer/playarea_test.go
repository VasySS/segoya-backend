package multiplayer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer/mocks"
)

type areaUsecaseStub struct {
	prepare func(context.Context, game.PanoramaProvider, playarea.Selection, uuid.UUID, int) (playarea.Selection, error)
}

func (s areaUsecaseStub) Prepare(
	ctx context.Context, provider game.PanoramaProvider, area playarea.Selection,
	userID uuid.UUID, rounds int,
) (playarea.Selection, error) {
	if s.prepare != nil {
		return s.prepare(ctx, provider, area, userID, rounds)
	}

	return area, nil
}

func TestNewGamePreparesPlayAreaInTransaction(t *testing.T) {
	t.Parallel()

	for _, insufficient := range []bool{false, true} {
		name := map[bool]string{false: "resolved snapshot persisted", true: "insufficient locations reject creation"}
		t.Run(name[insufficient], func(t *testing.T) {
			t.Parallel()
			repo := mocks.NewRepository(t)
			pano := mocks.NewPanoramaUsecase(t)
			userID := uuid.New()
			saved := playarea.Selection{Type: "saved", SavedID: uuid.New()}
			polygon := playarea.Selection{Type: "polygon", Polygon: &playarea.Polygon{
				Type: "Polygon", Coordinates: [][][]float64{{{10, 20}, {11, 20}, {11, 21}, {10, 20}}},
			}}

			type txKey struct{}

			repo.On("RunTx", mock.Anything, mock.AnythingOfType("repository.TxFunc")).
				Return(func(ctx context.Context, fn repository.TxFunc) error {
					return fn(context.WithValue(ctx, txKey{}, true))
				}).Once()

			area := areaUsecaseStub{prepare: func(
				ctx context.Context, provider game.PanoramaProvider, selection playarea.Selection,
				actor uuid.UUID, rounds int,
			) (playarea.Selection, error) {
				require.Equal(t, true, ctx.Value(txKey{}))
				require.Equal(t, game.GoogleProvider, provider)
				require.Equal(t, userID, actor)
				require.Equal(t, 3, rounds)
				require.Equal(t, saved, selection)

				if insufficient {
					return playarea.Selection{}, playarea.ErrInsufficientLocations
				}

				return polygon, nil
			}}
			req := dto.NewMultiplayerGameRequest{
				Provider: "google", Rounds: 3, PlayArea: saved, RequestTime: time.Now().UTC(), CreatorID: userID,
				ConnectedPlayers: []user.PublicProfile{
					{ID: userID},
					{ID: uuid.New()},
				},
			}
			cause := errors.New("stop after persistence")

			if !insufficient {
				expected := req
				expected.PlayArea = polygon
				repo.On("NewMultiplayerGame", mock.Anything, expected).Return(uuid.Nil, cause).Once()
			}

			uc := multiplayer.NewUsecase(multiplayer.Config{}, repo, pano, area)

			_, err := uc.NewGame(t.Context(), req)
			if insufficient {
				require.ErrorIs(t, err, playarea.ErrInsufficientLocations)
			} else {
				require.ErrorIs(t, err, cause)
			}
		})
	}
}

func TestNewRoundUsesStoredPlayArea(t *testing.T) {
	t.Parallel()
	h := newMultiplayerHarness(t)
	h.game.PlayArea = playarea.Selection{Type: "presets", PresetIDs: []uuid.UUID{uuid.New()}}
	h.expectTransaction()
	lock := h.repo.On("LockMultiplayerGame", mock.Anything, h.game.ID).Return(nil).Once()
	participants := h.repo.On("GetMultiplayerGameUsers", mock.Anything, h.game.ID).
		Return(h.participants(), nil).Once().NotBefore(lock)
	load := h.repo.On("GetMultiplayerGame", mock.Anything, h.game.ID).Return(h.game, nil).Once().NotBefore(participants)
	h.round.Finished = true
	current := h.repo.On("GetMultiplayerRound", mock.Anything, h.game.ID, h.game.RoundCurrent).
		Return(h.round, nil).Once().NotBefore(load)
	cause := playarea.ErrInsufficientLocations
	h.pano.On("NewStreetviewInArea",
		mock.Anything, h.game.Provider, h.game.PlayArea, h.game.ID, true).
		Return(game.PanoramaMetadata{}, cause).Once().NotBefore(current)

	_, err := h.uc.NewRound(t.Context(), dto.NewMultiplayerRoundRequest{
		GameID: h.game.ID, UserID: h.userID, RequestTime: h.now,
	})
	require.ErrorIs(t, err, cause)
}
