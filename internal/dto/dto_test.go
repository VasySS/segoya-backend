package dto_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/lobby"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestEmptyCollectionsSerializeAsArrays(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		value any
		json  string
	}{
		{"nil sessions", dto.SessionsToAPI(nil), `[]`},
		{"empty sessions", dto.SessionsToAPI([]user.Session{}), `[]`},
		{"nil oauth providers", dto.OAuthToAPI(nil), `[]`},
		{"empty oauth providers", dto.OAuthToAPI([]user.OAuth{}), `[]`},
		{"nil singleplayer guesses", dto.SingleplayerRoundsToAPI(nil), `[]`},
		{"nil multiplayer guesses", dto.MultiplayerGameGuessesToAPI(nil), `[]`},
		{"nil game page", dto.SingleplyerGamesToAPI(nil, 0), `{"total":0,"games":[]}`},
		{"empty game page", dto.SingleplyerGamesToAPI([]singleplayer.Game{}, 0), `{"total":0,"games":[]}`},
		{"nil lobby page", dto.LobbiesToAPI(nil, 0), `{"total":0,"lobbies":[]}`},
		{"empty lobby page", dto.LobbiesToAPI([]lobby.Lobby{}, 0), `{"total":0,"lobbies":[]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(tt.value)
			require.NoError(t, err)
			assert.JSONEq(t, tt.json, string(encoded))
		})
	}
}

func TestProfileResponsesExcludePrivateFields(t *testing.T) {
	t.Parallel()

	profile := user.PrivateProfile{
		PublicProfile: user.PublicProfile{
			ID:       uuid.MustParse("019a553a-fc00-7000-8000-000000000001"),
			Username: "alice", Name: "Alice", AvatarHash: "avatar",
			RegisterDate: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC),
		},
		Password: "secret-password-hash", AvatarLastUpdate: time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC),
	}
	for _, tt := range []struct {
		name  string
		value any
	}{
		{"public profile", dto.UserToAPIPublicUser(profile.ToPublicProfile())},
		{"private profile", dto.UserToAPIPrivateUser(profile)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(tt.value)
			require.NoError(t, err)

			var response map[string]any

			require.NoError(t, json.Unmarshal(encoded, &response))
			assert.Equal(t, profile.ID.String(), response["id"])
			assert.Equal(t, profile.Username, response["username"])
			assert.Equal(t, profile.Name, response["name"])
			assert.Equal(t, profile.AvatarHash, response["avatarHash"])
			assert.Equal(t, "2026-10-11T12:00:00Z", response["registerDate"])
			assert.NotContains(t, response, "password")
			assert.NotContains(t, response, "avatarLastUpdate")
			assert.NotContains(t, string(encoded), profile.Password)
		})
	}
}

func TestPageConversionsPreserveTotalAndOrder(t *testing.T) {
	t.Parallel()

	first := uuid.MustParse("019a553a-fc00-7000-8000-000000000001")
	second := uuid.MustParse("019a553a-fc00-7000-8000-000000000002")
	games := dto.SingleplyerGamesToAPI([]singleplayer.Game{{ID: second}, {ID: first}}, 42)
	assert.Equal(t, 42, games.Total)
	require.Len(t, games.Games, 2)
	assert.Equal(t, second, games.Games[0].ID)
	assert.Equal(t, first, games.Games[1].ID)
	lobbies := dto.LobbiesToAPI([]lobby.Lobby{{ID: "second"}, {ID: "first"}}, 73)
	assert.Equal(t, 73, lobbies.Total)
	require.Len(t, lobbies.Lobbies, 2)
	assert.Equal(t, "second", lobbies.Lobbies[0].ID)
	assert.Equal(t, "first", lobbies.Lobbies[1].ID)
}
