package lobby_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	lobbyEntity "github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/playarea"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby/mocks"
)

type areaUsecaseStub struct {
	resolve func(
		context.Context, game.PanoramaProvider, playarea.Selection, uuid.UUID,
	) (playarea.Selection, error)
}

func (s areaUsecaseStub) Resolve(
	ctx context.Context, provider game.PanoramaProvider, area playarea.Selection, userID uuid.UUID,
) (playarea.Selection, error) {
	if s.resolve != nil {
		return s.resolve(ctx, provider, area, userID)
	}

	return area, nil
}

func TestLobbySettingsPersistResolvedSnapshot(t *testing.T) {
	t.Parallel()
	repo := mocks.NewRepository(t)
	creator := uuid.New()
	saved := playarea.Selection{Type: "saved", SavedID: uuid.New()}
	snapshot := playarea.Selection{Type: "polygon", Polygon: &playarea.Polygon{
		Type: "Polygon", Coordinates: [][][]float64{{{10, 20}, {11, 20}, {11, 21}, {10, 20}}},
	}}
	original := lobbyEntity.Lobby{ID: "test", CreatorID: creator, Provider: "google", Rounds: 3, MaxPlayers: 2}
	repo.On("GetLobby", mock.Anything, original.ID).Return(original, nil).Once()
	expected := original
	expected.PlayArea = snapshot
	repo.On("UpdateLobby", mock.Anything, expected).Return(nil).Once()

	areas := areaUsecaseStub{resolve: func(
		_ context.Context, provider game.PanoramaProvider, selection playarea.Selection, userID uuid.UUID,
	) (playarea.Selection, error) {
		require.Equal(t, saved, selection)
		require.Equal(t, creator, userID)
		require.Equal(t, game.GoogleProvider, provider)

		return snapshot, nil
	}}
	uc := lobby.NewUsecase(lobby.Config{}, nil, nil, repo, nil, areas)
	require.NoError(t, uc.UpdateLobbySettings(t.Context(), dto.UpdateLobbySettingsRequest{
		LobbyID: original.ID, Creator: user.PublicProfile{ID: creator},
		Settings: dto.LobbySettingsMessage{Provider: "google", Rounds: 3, PlayArea: saved},
	}))
}

func TestLobbySettingsRejectInaccessibleSavedArea(t *testing.T) {
	t.Parallel()
	repo := mocks.NewRepository(t)
	creator := uuid.New()
	original := lobbyEntity.Lobby{ID: "test", CreatorID: creator, Provider: "google", Rounds: 3, MaxPlayers: 2}
	repo.On("GetLobby", mock.Anything, original.ID).Return(original, nil).Once()

	areas := areaUsecaseStub{resolve: func(
		context.Context, game.PanoramaProvider, playarea.Selection, uuid.UUID,
	) (playarea.Selection, error) {
		return playarea.Selection{}, playarea.ErrNotFound
	}}
	uc := lobby.NewUsecase(lobby.Config{}, nil, nil, repo, nil, areas)
	err := uc.UpdateLobbySettings(t.Context(), dto.UpdateLobbySettingsRequest{
		LobbyID: original.ID, Creator: user.PublicProfile{ID: creator},
		Settings: dto.LobbySettingsMessage{
			Provider: "google", Rounds: 3, PlayArea: playarea.Selection{Type: "saved", SavedID: uuid.New()},
		},
	})
	require.ErrorIs(t, err, playarea.ErrNotFound)
}

func TestLobbyStartForwardsSnapshotAndRetainsLobbyWhenInsufficient(t *testing.T) {
	t.Parallel()
	repo := mocks.NewRepository(t)
	games := mocks.NewMultiplayerUsecase(t)
	creator := uuid.New()
	original := lobbyEntity.Lobby{
		ID: "test", CreatorID: creator, Provider: "google", Rounds: 3, MaxPlayers: 2, CurrentPlayers: 2,
		PlayArea: playarea.Selection{Type: "presets", PresetIDs: []uuid.UUID{uuid.New()}},
	}
	repo.On("GetLobby", mock.Anything, original.ID).Return(original, nil).Once()
	players := []user.PublicProfile{{ID: creator}, {ID: uuid.New()}}
	games.On("NewGame", mock.Anything, dto.NewMultiplayerGameRequest{
		CreatorID: creator, Provider: "google", Rounds: 3, PlayArea: original.PlayArea, ConnectedPlayers: players,
	}).Return(uuid.Nil, playarea.ErrInsufficientLocations).Once()

	uc := lobby.NewUsecase(lobby.Config{}, nil, nil, repo, games, areaUsecaseStub{})
	_, err := uc.StartLobbyGame(t.Context(), dto.StartLobbyGameRequest{
		LobbyID: original.ID, Creator: user.PublicProfile{ID: creator}, ConnectedPlayers: players,
	})
	require.ErrorIs(t, err, playarea.ErrInsufficientLocations)
}
