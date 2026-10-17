package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/usecase/lobby"
	"github.com/VasySS/segoya-backend/internal/usecase/multiplayer"
	"github.com/VasySS/segoya-backend/internal/usecase/singleplayer"
)

func TestInvalidCreationRequestsDoNotReachPersistence(t *testing.T) {
	t.Parallel()

	sp := singleplayer.NewUsecase(singleplayer.Config{}, nil, nil)
	mp := multiplayer.NewUsecase(multiplayer.Config{}, nil, nil)
	lobbies := lobby.NewUsecase(lobby.Config{}, nil, nil, nil, nil)

	for _, input := range []struct {
		provider string
		rounds   int
		timer    int
		want     error
	}{
		{provider: "unknown", rounds: 0, timer: -1, want: game.ErrInvalidProvider},
		{provider: "google", rounds: 0, want: game.ErrInvalidRounds},
		{provider: "google", rounds: 11, want: game.ErrInvalidRounds},
		{provider: "google", rounds: 1, timer: 9, want: game.ErrInvalidTimerSeconds},
		{provider: "google", rounds: 1, timer: 601, want: game.ErrInvalidTimerSeconds},
	} {
		_, err := sp.NewGame(t.Context(), dto.NewSingleplayerGameRequest{
			Provider: input.provider, Rounds: input.rounds, TimerSeconds: input.timer,
		})
		require.ErrorIs(t, err, input.want)

		_, err = mp.NewGame(t.Context(), dto.NewMultiplayerGameRequest{
			Provider: input.provider, Rounds: input.rounds, TimerSeconds: input.timer,
		})
		require.ErrorIs(t, err, input.want)

		_, err = lobbies.NewLobby(t.Context(), dto.NewLobbyRequest{
			Provider: input.provider, Rounds: input.rounds, TimerSeconds: input.timer, MaxPlayers: 2,
		})
		require.ErrorIs(t, err, input.want)
	}

	_, err := mp.NewGame(t.Context(), dto.NewMultiplayerGameRequest{Provider: "google", Rounds: 1})
	require.ErrorIs(t, err, game.ErrInvalidPlayers)

	_, err = lobbies.NewLobby(t.Context(), dto.NewLobbyRequest{Provider: "google", Rounds: 1, MaxPlayers: 1})
	require.ErrorIs(t, err, game.ErrInvalidPlayers)
}
