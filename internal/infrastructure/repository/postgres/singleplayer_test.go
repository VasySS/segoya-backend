//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
)

func TestSingleplayerTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SingleplayerTestSuite))
}

type SingleplayerTestSuite struct{ postgresTestSuite }

func (s *SingleplayerTestSuite) TestNewSingleplayerGame() {
	newUser := s.newTestUser()
	newTestGame, gameReq := s.newTestGame(newUser.ID)

	game, err := s.postgresRepo.GetSingleplayerGame(s.ctx, newTestGame.ID)
	s.Require().NoError(err)
	s.Equal(gameReq.Rounds, game.Rounds)
	s.Equal(0, game.RoundCurrent)
	s.Equal(gameReq.TimerSeconds, game.TimerSeconds)
	s.EqualValues(gameReq.Provider, game.Provider)
	s.Equal(gameReq.MovementAllowed, game.MovementAllowed)
	s.Equal(gameReq.RequestTime, game.CreatedAt)
}

func (s *SingleplayerTestSuite) TestSingleplayerGameByID() {
	newUser := s.newTestUser()

	_, err := s.postgresRepo.GetSingleplayerGame(s.ctx, uuid.New())
	s.Require().ErrorIs(err, singleplayer.ErrGameNotFound)

	newTestGame, gameReq := s.newTestGame(newUser.ID)

	s.Equal(gameReq.Rounds, newTestGame.Rounds)
	s.Equal(0, newTestGame.RoundCurrent)
	s.Equal(gameReq.TimerSeconds, newTestGame.TimerSeconds)
	s.EqualValues(gameReq.Provider, newTestGame.Provider)
	s.Equal(gameReq.MovementAllowed, newTestGame.MovementAllowed)
	s.Equal(gameReq.RequestTime, newTestGame.CreatedAt)
}

func (s *SingleplayerTestSuite) TestSingleplayerGamesByUserID() {
	newUser := s.newTestUser()
	createdGames := make([]singleplayer.Game, 0, 3)

	for range 3 {
		newGame, _ := s.newTestGame(newUser.ID)
		createdGames = append(createdGames, newGame)
	}

	games, gamesAmount, err := s.postgresRepo.GetSingleplayerGames(s.ctx, dto.GetSingleplayerGamesRequest{
		UserID:   newUser.ID,
		Page:     1,
		PageSize: 3,
	})
	s.Require().NoError(err)

	s.ElementsMatch(createdGames, games)
	s.Equal(3, gamesAmount)
}

func (s *SingleplayerTestSuite) TestEndSingleplayerGame() {
	newUser := s.newTestUser()
	newTestGame, _ := s.newTestGame(newUser.ID)

	s.False(newTestGame.Finished)

	endGameReq := dto.EndSingleplayerGameRequestDB{
		RequestTime: fixtureTime(),
		GameID:      newTestGame.ID,
	}

	err := s.postgresRepo.EndSingleplayerGame(s.ctx, endGameReq)
	s.Require().NoError(err)

	updatedGame, err := s.postgresRepo.GetSingleplayerGame(s.ctx, newTestGame.ID)
	s.Require().NoError(err)

	s.True(updatedGame.Finished)
	s.Equal(endGameReq.RequestTime, updatedGame.EndedAt)
}

func (s *SingleplayerTestSuite) TestNewSingleplayerRound() {
	ctx := s.ctx

	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)

	newRound, newRoundReq := s.newTestRound(ctx, newGame.ID, 1)

	s.Equal(newRoundReq.GameID, newRound.GameID)
	s.Equal(newRoundReq.RoundNum, newRound.RoundNum)
	s.Equal(newRoundReq.CreatedAt, newRound.CreatedAt)
	s.Equal(newRoundReq.StartedAt, newRound.StartedAt)
}

func (s *SingleplayerTestSuite) TestGetSingleplayerRound() {
	ctx := s.ctx

	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)
	_, newRoundReq := s.newTestRound(ctx, newGame.ID, 1)

	getRoundResponse, err := s.postgresRepo.GetSingleplayerRound(s.ctx, newGame.ID, 1)
	s.Require().NoError(err)

	s.Equal(newRoundReq.GameID, getRoundResponse.GameID)
	s.Equal(newRoundReq.RoundNum, getRoundResponse.RoundNum)
	s.Equal(newGame.ID, getRoundResponse.GameID)
	s.Equal(newRoundReq.CreatedAt, getRoundResponse.CreatedAt)
	s.Equal(newRoundReq.StartedAt, getRoundResponse.StartedAt)
}

func (s *SingleplayerTestSuite) TestSingleplayerGamesPaginationAndUserIsolation() {
	owner, other := s.newTestUser(), s.newTestUser()
	ids := make([]uuid.UUID, 0, 3)

	for i := range 3 {
		id, err := s.postgresRepo.NewSingleplayerGame(s.ctx, dto.NewSingleplayerGameRequest{
			UserID: owner.ID, RequestTime: fixtureTime().Add(time.Duration(i) * time.Second),
			Rounds: 3, TimerSeconds: 30, Provider: "google",
		})
		s.Require().NoError(err)

		ids = append(ids, id)
	}

	s.newTestGame(other.ID)

	for _, tc := range []struct {
		name string
		page int
		ids  []uuid.UUID
	}{
		{name: "first page", page: 1, ids: []uuid.UUID{ids[2], ids[1]}},
		{name: "partial last page", page: 2, ids: []uuid.UUID{ids[0]}},
		{name: "out of range", page: 3, ids: []uuid.UUID{}},
	} {
		s.Run(tc.name, func() {
			games, total, err := s.postgresRepo.GetSingleplayerGames(s.ctx, dto.GetSingleplayerGamesRequest{
				UserID: owner.ID, Page: tc.page, PageSize: 2,
			})
			s.Require().NoError(err)
			s.Equal(3, total)

			actual := make([]uuid.UUID, 0, len(games))
			for _, game := range games {
				actual = append(actual, game.ID)
				s.Equal(owner.ID, game.UserID)
			}

			s.Equal(tc.ids, actual)
		})
	}

	games, total, err := s.postgresRepo.GetSingleplayerGames(s.ctx, dto.GetSingleplayerGamesRequest{
		UserID: uuid.New(), Page: 1, PageSize: 2,
	})
	s.Require().NoError(err)
	s.Empty(games)
	s.Zero(total)
}

func (s *SingleplayerTestSuite) TestDuplicateRoundPreservesOriginal() {
	owner := s.newTestUser()
	game, _ := s.newTestGame(owner.ID)
	original, req := s.newTestRound(s.ctx, game.ID, 1)
	req.CreatedAt = req.CreatedAt.Add(time.Minute)
	_, err := s.postgresRepo.NewSingleplayerRound(s.ctx, req)
	s.Require().Error(err)
	stored, err := s.postgresRepo.GetSingleplayerRound(s.ctx, game.ID, 1)
	s.Require().NoError(err)
	s.Equal(original, stored)
	storedGame, err := s.postgresRepo.GetSingleplayerGame(s.ctx, game.ID)
	s.Require().NoError(err)
	s.Equal(1, storedGame.RoundCurrent)
}

func (s *SingleplayerTestSuite) TestDuplicateGuessPreservesOriginalResult() {
	owner := s.newTestUser()
	gameValue, _ := s.newTestGame(owner.ID)
	round, _ := s.newTestRound(s.ctx, gameValue.ID, 1)
	req := dto.NewSingleplayerRoundGuessRequest{
		GameID: gameValue.ID, RoundID: round.ID, RequestTime: fixtureTime().Add(time.Minute),
		Guess: &game.LatLng{Lat: 12.5, Lng: -33.75}, Score: 1000, Distance: 2500,
	}
	s.Require().NoError(s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, req))
	retry := req
	retry.Score, retry.Distance, retry.RequestTime = 2000, 0, req.RequestTime.Add(time.Minute)
	s.Require().Error(s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, retry))
	stored, err := s.postgresRepo.GetSingleplayerRound(s.ctx, gameValue.ID, 1)
	s.Require().NoError(err)
	s.True(stored.Finished)
	s.Equal(req.RequestTime, stored.EndedAt)
	guesses, err := s.postgresRepo.GetSingleplayerGameGuesses(s.ctx, gameValue.ID)
	s.Require().NoError(err)
	s.Require().Len(guesses, 1)
	s.Equal(req.Score, guesses[0].Score)
	s.Equal(req.Distance, guesses[0].MissDistance)
	s.InDelta(req.Guess.Lat, guesses[0].GuessLat, 1e-10)
	s.InDelta(req.Guess.Lng, guesses[0].GuessLng, 1e-10)
}

func (s *SingleplayerTestSuite) TestFailedGuessLeavesRoundUnfinished() {
	owner := s.newTestUser()
	game, _ := s.newTestGame(owner.ID)
	original, _ := s.newTestRound(s.ctx, game.ID, 1)
	s.Require().Error(s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, dto.NewSingleplayerRoundGuessRequest{
		GameID: game.ID, RoundID: uuid.New(), RequestTime: fixtureTime(),
	}))

	stored, err := s.postgresRepo.GetSingleplayerRound(s.ctx, game.ID, 1)
	s.Require().NoError(err)
	s.Equal(original, stored)
	guesses, err := s.postgresRepo.GetSingleplayerGameGuesses(s.ctx, game.ID)
	s.Require().NoError(err)
	s.Empty(guesses)
}

func (s *SingleplayerTestSuite) TestSetSingleplayerGuess() {
	ctx := s.ctx

	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)
	firstRound, _ := s.newTestRound(ctx, newGame.ID, 1)

	setGuessReq := dto.NewSingleplayerRoundGuessRequest{
		RequestTime: fixtureTime(),
		RoundID:     firstRound.ID,
		GameID:      newGame.ID,
		Guess: &game.LatLng{
			Lat: 12.5,
			Lng: -33.75,
		},
		Score:    1500,
		Distance: 2500,
	}

	err := s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, setGuessReq)
	s.Require().NoError(err)

	updatedRound, err := s.postgresRepo.GetSingleplayerRound(s.ctx, newGame.ID, 1)
	s.Require().NoError(err)

	s.True(updatedRound.Finished)
	s.Equal(setGuessReq.RequestTime, updatedRound.EndedAt)
}

func (s *SingleplayerTestSuite) TestSingleplayerRoundsWithGuesses() {
	ctx := s.ctx

	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)

	guesses := make([]singleplayer.Guess, 0, newGame.Rounds)

	for i := 1; i <= newGame.Rounds; i++ {
		newRound, _ := s.newTestRound(ctx, newGame.ID, i)

		newRoundGuessReq := dto.NewSingleplayerRoundGuessRequest{
			RequestTime: fixtureTime(),
			RoundID:     newRound.ID,
			GameID:      newGame.ID,
			Guess: &game.LatLng{
				Lat: 12.5,
				Lng: -33.75,
			},
			Score:    1500,
			Distance: 2500,
		}

		err := s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, newRoundGuessReq)
		s.Require().NoError(err)

		updatedRound, err := s.postgresRepo.GetSingleplayerRound(s.ctx, newGame.ID, i)
		s.Require().NoError(err)
		s.True(updatedRound.Finished)
		s.Equal(newRoundGuessReq.RequestTime, updatedRound.EndedAt)

		guesses = append(guesses, singleplayer.Guess{
			RoundNum:     i,
			RoundLat:     updatedRound.Lat,
			RoundLng:     updatedRound.Lng,
			GuessLat:     newRoundGuessReq.Guess.Lat,
			GuessLng:     newRoundGuessReq.Guess.Lng,
			Score:        newRoundGuessReq.Score,
			MissDistance: newRoundGuessReq.Distance,
		})
	}

	roundGuesses, err := s.postgresRepo.GetSingleplayerGameGuesses(s.ctx, newGame.ID)
	s.Require().NoError(err)

	s.ElementsMatch(guesses, roundGuesses)

	gameScore := 0
	for _, roundWithGuess := range roundGuesses {
		gameScore += roundWithGuess.Score
	}

	updatedGame, err := s.postgresRepo.GetSingleplayerGame(s.ctx, newGame.ID)
	s.Require().NoError(err)

	s.Equal(gameScore, updatedGame.Score)
	s.Equal(updatedGame.Rounds, updatedGame.RoundCurrent)
}

func (s *SingleplayerTestSuite) newTestUser() user.PrivateProfile {
	s.T().Helper()

	newUserReq := s.newUserRequest()

	err := s.postgresRepo.NewUser(s.ctx, newUserReq)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, newUserReq.Username)
	s.Require().NoError(err)

	return newUser
}

func (s *SingleplayerTestSuite) newTestGame(userID uuid.UUID) (singleplayer.Game, dto.NewSingleplayerGameRequest) {
	gameReq := dto.NewSingleplayerGameRequest{
		RequestTime:     fixtureTime(),
		UserID:          userID,
		Rounds:          3,
		TimerSeconds:    30,
		Provider:        "google",
		MovementAllowed: true,
	}

	gameID, err := s.postgresRepo.NewSingleplayerGame(s.ctx, gameReq)
	s.Require().NoError(err)

	newGame, err := s.postgresRepo.GetSingleplayerGame(s.ctx, gameID)
	s.Require().NoError(err)

	return newGame, gameReq
}

func (s *SingleplayerTestSuite) newTestRound(
	ctx context.Context,
	gameID uuid.UUID,
	roundNum int,
) (singleplayer.Round, dto.NewSingleplayerRoundDBRequest) {
	stv, err := s.postgresRepo.RandomGoogleStreetview(ctx)
	s.Require().NoError(err)

	roundReq := dto.NewSingleplayerRoundDBRequest{
		CreatedAt:  fixtureTime(),
		StartedAt:  fixtureTime().Add(time.Second * 10),
		GameID:     gameID,
		LocationID: stv.ID,
		RoundNum:   roundNum,
	}

	round, err := s.postgresRepo.NewSingleplayerRound(ctx, roundReq)
	s.Require().NoError(err)

	return round, roundReq
}
