//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestMultiplayerTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MultiplayerTestSuite))
}

type MultiplayerTestSuite struct{ postgresTestSuite }

func (s *MultiplayerTestSuite) TestNewMultiplayerGame() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()
	userSecondPlayer := s.newTestUser()

	newGame, gameReq := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
			userSecondPlayer.PublicProfile,
		})

	s.Equal(gameReq.CreatorID, newGame.CreatorID)
	s.Equal(gameReq.Rounds, newGame.Rounds)
	s.Equal(0, newGame.RoundCurrent)
	s.Equal(gameReq.TimerSeconds, newGame.TimerSeconds)
	s.EqualValues(gameReq.Provider, newGame.Provider)
	s.Equal(gameReq.MovementAllowed, newGame.MovementAllowed)
	s.Equal(len(gameReq.ConnectedPlayers), newGame.Players)
	s.Equal(gameReq.RequestTime, newGame.CreatedAt)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameByID() {
	ctx := s.ctx

	_, err := s.postgresRepo.GetMultiplayerGame(ctx, uuid.New())
	s.Require().ErrorIs(err, multiplayer.ErrGameNotFound)

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()
	userSecondPlayer := s.newTestUser()

	newGame, gameReq := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
			userSecondPlayer.PublicProfile,
		})

	s.Equal(gameReq.CreatorID, newGame.CreatorID)
	s.Equal(gameReq.Rounds, newGame.Rounds)
	s.Equal(0, newGame.RoundCurrent)
	s.Equal(gameReq.TimerSeconds, newGame.TimerSeconds)
	s.EqualValues(gameReq.Provider, newGame.Provider)
	s.Equal(gameReq.MovementAllowed, newGame.MovementAllowed)
	s.Len(gameReq.ConnectedPlayers, newGame.Players)
	s.Equal(gameReq.RequestTime, newGame.CreatedAt)
}

func (s *MultiplayerTestSuite) TestNewMultiplayerRound() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	newRound, roundReq := s.newTestRound(ctx, newGame.ID, 1)

	s.Zero(newRound.Lat)
	s.Zero(newRound.Lng)
	s.Equal(0, newRound.GuessesCount)
	s.Equal(roundReq.GameID, newRound.GameID)
	s.Equal(roundReq.RoundNum, newRound.RoundNum)
	s.Equal(roundReq.CreatedAt, newRound.CreatedAt)
	s.Equal(roundReq.StartedAt, newRound.StartedAt)
}

func (s *MultiplayerTestSuite) TestDuplicateRoundPreservesOriginal() {
	creator, other := s.newTestUser(), s.newTestUser()
	game, _ := s.newTestGame(s.ctx, creator.ID, []user.PublicProfile{creator.PublicProfile, other.PublicProfile})
	original, req := s.newTestRound(s.ctx, game.ID, 1)
	req.CreatedAt = req.CreatedAt.Add(time.Minute)
	_, err := s.postgresRepo.NewMultiplayerRound(s.ctx, req)
	s.Require().Error(err)
	stored, err := s.postgresRepo.GetMultiplayerRound(s.ctx, game.ID, 1)
	s.Require().NoError(err)
	s.Equal(original, stored)
	storedGame, err := s.postgresRepo.GetMultiplayerGame(s.ctx, game.ID)
	s.Require().NoError(err)
	s.Equal(1, storedGame.RoundCurrent)
}

func (s *MultiplayerTestSuite) TestDuplicateGuessPreservesScoreAndCount() {
	creator, other := s.newTestUser(), s.newTestUser()
	game, _ := s.newTestGame(s.ctx, creator.ID, []user.PublicProfile{creator.PublicProfile, other.PublicProfile})
	round, _ := s.newTestRound(s.ctx, game.ID, 1)
	expected := s.newTestGuess(s.ctx, creator, round, 1000)
	s.Require().Error(s.postgresRepo.NewMultiplayerRoundGuess(s.ctx, dto.NewMultiplayerRoundGuessRequestDB{
		RoundID: round.ID, UserID: creator.ID, RequestTime: fixtureTime().Add(time.Minute), Score: 2000,
	}))

	stored, err := s.postgresRepo.GetMultiplayerRound(s.ctx, game.ID, 1)
	s.Require().NoError(err)
	s.Equal(1, stored.GuessesCount)
	guesses, err := s.postgresRepo.GetMultiplayerGameGuesses(s.ctx, game.ID)
	s.Require().NoError(err)
	s.Equal([]multiplayer.Guess{expected}, guesses)
	profile, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, creator.ID, game.ID)
	s.Require().NoError(err)
	s.Equal(1000, profile.Score)
}

func (s *MultiplayerTestSuite) TestFailedGameCreationDoesNotPersistPartialRows() {
	creator := s.newTestUser()
	for _, tc := range []struct {
		name    string
		players []user.PublicProfile
	}{
		{name: "duplicate participant", players: []user.PublicProfile{creator.PublicProfile, creator.PublicProfile}},
		{name: "missing participant", players: []user.PublicProfile{creator.PublicProfile, {ID: uuid.New()}}},
	} {
		s.Run(tc.name, func() {
			_, err := s.postgresRepo.NewMultiplayerGame(s.ctx, dto.NewMultiplayerGameRequest{
				CreatorID: creator.ID, ConnectedPlayers: tc.players, RequestTime: fixtureTime(),
				Provider: "google", Rounds: 3, TimerSeconds: 30,
			})
			s.Require().Error(err)

			var games, members int

			s.Require().NoError(s.pool.QueryRow(s.ctx, `SELECT COUNT(*) FROM multiplayer_game`).Scan(&games))
			s.Require().NoError(s.pool.QueryRow(s.ctx, `SELECT COUNT(*) FROM multiplayer_game_user`).Scan(&members))
			s.Zero(games)
			s.Zero(members)
		})
	}
}

func (s *MultiplayerTestSuite) TestIndividualUserScoreIsIsolatedByGame() {
	creator, other := s.newTestUser(), s.newTestUser()
	for _, score := range []int{1000, 2000} {
		game, _ := s.newTestGame(s.ctx, creator.ID, []user.PublicProfile{creator.PublicProfile, other.PublicProfile})
		round, _ := s.newTestRound(s.ctx, game.ID, 1)
		s.newTestGuess(s.ctx, creator, round, score)
		profile, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, creator.ID, game.ID)
		s.Require().NoError(err)
		s.Equal(score, profile.Score)
		s.True(profile.Guessed)
		_, err = s.postgresRepo.GetMultiplayerGameUser(s.ctx, s.newTestUser().ID, game.ID)
		s.ErrorIs(err, multiplayer.ErrGameWrongUserID)
	}
}

func (s *MultiplayerTestSuite) TestGetMultiplayerRound() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	roundNum := 1

	_, err := s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().ErrorIs(err, multiplayer.ErrRoundNotFound)

	_, _ = s.newTestRound(ctx, newGame.ID, roundNum)

	_, err = s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().NoError(err)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameUser() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	_, _ = s.newTestRound(ctx, newGame.ID, 1)

	creatorUserMultiplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userCreator.ID, newGame.ID)
	s.Require().NoError(err)
	s.Equal(userCreator.PublicProfile, creatorUserMultiplayer.PublicProfile)

	firstUserMultplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userFirstPlayer.ID, newGame.ID)
	s.Require().NoError(err)
	s.Equal(userFirstPlayer.PublicProfile, firstUserMultplayer.PublicProfile)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameUsers() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	gameUsers := []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	}

	for i := 1; i <= 3; i++ {
		testGame, _ := s.newTestGame(ctx, userCreator.ID, gameUsers)

		creatorScore := 0
		firstPlayerScore := 0

		for roundNum := 1; roundNum <= testGame.Rounds; roundNum++ {
			newRound, _ := s.newTestRound(ctx, testGame.ID, roundNum)

			creatorGuess := s.newTestGuess(ctx, userCreator, newRound, 1000)
			firstPlayerGuess := s.newTestGuess(ctx, userFirstPlayer, newRound, 2000)

			creatorScore += creatorGuess.Score
			firstPlayerScore += firstPlayerGuess.Score
		}

		users, err := s.postgresRepo.GetMultiplayerGameUsers(s.ctx, testGame.ID)
		s.Require().NoError(err)

		s.Require().Len(users, 2)

		byID := make(map[uuid.UUID]user.MultiplayerUser, len(users))
		for _, player := range users {
			byID[player.ID] = player
		}

		s.Equal(userCreator.PublicProfile, byID[userCreator.ID].PublicProfile)
		s.Equal(userFirstPlayer.PublicProfile, byID[userFirstPlayer.ID].PublicProfile)
		s.Equal(creatorScore, byID[userCreator.ID].Score)
		s.Equal(firstPlayerScore, byID[userFirstPlayer.ID].Score)
		s.True(byID[userCreator.ID].Guessed)
		s.True(byID[userFirstPlayer.ID].Guessed)
	}
}

func (s *MultiplayerTestSuite) TestSetMultiplayerUserGuess() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})
	newRound, _ := s.newTestRound(ctx, newGame.ID, 1)

	guessScore := 1500

	err := s.postgresRepo.NewMultiplayerRoundGuess(s.ctx, dto.NewMultiplayerRoundGuessRequestDB{
		RequestTime: fixtureTime(),
		UserID:      userFirstPlayer.ID,
		RoundID:     newRound.ID,
		Score:       guessScore,
		Lat:         12.5,
		Lng:         -33.75,
		Distance:    2500,
	})
	s.Require().NoError(err)

	firstPlayerMultiplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userFirstPlayer.ID, newGame.ID)
	s.Require().NoError(err)
	s.Equal(guessScore, firstPlayerMultiplayer.Score)
	s.True(firstPlayerMultiplayer.Guessed)

	// creator has not guessed the current round
	creatorMultiplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userCreator.ID, newGame.ID)
	s.Require().NoError(err)
	s.False(creatorMultiplayer.Guessed)

	// once a new round starts (with no guesses), the flag is derived against it and resets
	_, _ = s.newTestRound(ctx, newGame.ID, 2)

	firstPlayerMultiplayer, err = s.postgresRepo.GetMultiplayerGameUser(s.ctx, userFirstPlayer.ID, newGame.ID)
	s.Require().NoError(err)
	s.False(firstPlayerMultiplayer.Guessed)
}

func (s *MultiplayerTestSuite) TestMultiplayerRoundGuesses() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	for i := 1; i <= 2; i++ {
		newRound, _ := s.newTestRound(ctx, newGame.ID, i)

		creatorGuess := s.newTestGuess(ctx, userCreator, newRound, 1000)
		guesses, err := s.postgresRepo.GetMultiplayerRoundGuesses(s.ctx, newRound.ID)
		s.Require().NoError(err)
		s.ElementsMatch([]multiplayer.Guess{creatorGuess}, guesses)

		firstUserGuess := s.newTestGuess(ctx, userFirstPlayer, newRound, 2000)
		guesses, err = s.postgresRepo.GetMultiplayerRoundGuesses(s.ctx, newRound.ID)
		s.Require().NoError(err)
		s.ElementsMatch([]multiplayer.Guess{creatorGuess, firstUserGuess}, guesses)
	}

	updatedRound, err := s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, 2)
	s.Require().NoError(err)
	s.Equal(2, updatedRound.GuessesCount)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameGuesses() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	firstRound, _ := s.newTestRound(ctx, newGame.ID, 1)
	secondRound, _ := s.newTestRound(ctx, newGame.ID, 2)

	creatorFirstRoundGuess := s.newTestGuess(ctx, userCreator, firstRound, 1000)
	firstPlayerFirstRoundGuess := s.newTestGuess(ctx, userFirstPlayer, firstRound, 2000)

	creatorSecondRoundGuess := s.newTestGuess(ctx, userCreator, secondRound, 1000)
	firstPlayerSecondRoundGuess := s.newTestGuess(ctx, userFirstPlayer, secondRound, 2000)

	gameGuesses, err := s.postgresRepo.GetMultiplayerGameGuesses(s.ctx, newGame.ID)
	s.Require().NoError(err)

	s.ElementsMatch([]multiplayer.Guess{
		creatorFirstRoundGuess,
		firstPlayerFirstRoundGuess,
		creatorSecondRoundGuess,
		firstPlayerSecondRoundGuess,
	}, gameGuesses)
}

func (s *MultiplayerTestSuite) TestEndMultiplayerRound() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	roundNum := 1
	newRound, _ := s.newTestRound(ctx, newGame.ID, roundNum)

	endRoundReq := dto.EndMultiplayerRoundRequestDB{
		RequestTime: fixtureTime(),
		RoundID:     newRound.ID,
	}

	err := s.postgresRepo.EndMultiplayerRound(s.ctx, endRoundReq)
	s.Require().NoError(err)

	updatedRound, err := s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().NoError(err)

	s.True(updatedRound.Finished)
	s.Equal(endRoundReq.RequestTime, updatedRound.EndedAt)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameEnd() {
	ctx := s.ctx

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(
		ctx,
		userCreator.ID,
		[]user.PublicProfile{
			userCreator.PublicProfile,
			userFirstPlayer.PublicProfile,
		})

	endGameReq := dto.EndMultiplayerGameRequestDB{
		RequestTime: fixtureTime(),
		GameID:      newGame.ID,
	}

	err := s.postgresRepo.EndMultiplayerGame(s.ctx, endGameReq)
	s.Require().NoError(err)

	updatedGame, err := s.postgresRepo.GetMultiplayerGame(s.ctx, newGame.ID)
	s.Require().NoError(err)

	s.True(updatedGame.Finished)
	s.Equal(endGameReq.RequestTime, updatedGame.EndedAt)
}

func (s *MultiplayerTestSuite) newTestUser() user.PrivateProfile {
	s.T().Helper()

	newUserReq := s.newUserRequest()

	err := s.postgresRepo.NewUser(s.ctx, newUserReq)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, newUserReq.Username)
	s.Require().NoError(err)

	return newUser
}

func (s *MultiplayerTestSuite) newTestGame(
	ctx context.Context,
	userID uuid.UUID,
	connectedPlayers []user.PublicProfile,
) (multiplayer.Game, dto.NewMultiplayerGameRequest) {
	gameReq := dto.NewMultiplayerGameRequest{
		RequestTime:      fixtureTime(),
		CreatorID:        userID,
		ConnectedPlayers: connectedPlayers,
		Rounds:           3,
		TimerSeconds:     30,
		Provider:         "google",
		MovementAllowed:  true,
	}

	gameID, err := s.postgresRepo.NewMultiplayerGame(ctx, gameReq)
	s.Require().NoError(err)

	newGame, err := s.postgresRepo.GetMultiplayerGame(ctx, gameID)
	s.Require().NoError(err)

	return newGame, gameReq
}

func (s *MultiplayerTestSuite) newTestRound(
	ctx context.Context,
	gameID uuid.UUID,
	roundNum int,
) (multiplayer.Round, dto.NewMultiplayerRoundRequestDB) {
	stv, err := s.postgresRepo.RandomGoogleStreetview(ctx)
	s.Require().NoError(err)

	roundReq := dto.NewMultiplayerRoundRequestDB{
		CreatedAt:  fixtureTime(),
		StartedAt:  fixtureTime().Add(time.Second * 10),
		GameID:     gameID,
		LocationID: stv.ID,
		RoundNum:   roundNum,
	}

	round, err := s.postgresRepo.NewMultiplayerRound(ctx, roundReq)
	s.Require().NoError(err)

	return round, roundReq
}

func (s *MultiplayerTestSuite) newTestGuess(
	ctx context.Context,
	user user.PrivateProfile,
	round multiplayer.Round,
	score int,
) multiplayer.Guess {
	req := dto.NewMultiplayerRoundGuessRequestDB{
		RequestTime: fixtureTime(),
		UserID:      user.ID,
		RoundID:     round.ID,
		Lat:         12.5,
		Lng:         -33.75,
		Score:       score,
		Distance:    1500,
	}

	multiplayerGuess := multiplayer.Guess{
		Username:   user.Username,
		AvatarHash: user.AvatarHash,
		RoundNum:   round.RoundNum,
		RoundLat:   round.Lat,
		RoundLng:   round.Lng,
		Lat:        req.Lat,
		Lng:        req.Lng,
		Score:      req.Score,
	}

	err := s.postgresRepo.NewMultiplayerRoundGuess(ctx, req)
	s.Require().NoError(err)

	return multiplayerGuess
}
