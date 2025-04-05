package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game"
	"github.com/VasySS/segoya-backend/internal/entity/game/singleplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	postgresRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/migrations/data"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
)

func TestSingleplayerTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SingleplayerTestSuite))
}

type SingleplayerTestSuite struct {
	suite.Suite
	ctx               context.Context
	postgresContainer *containers.PostgresContainer
	postgresRepo      *postgresRepo.Repository
}

func (s *SingleplayerTestSuite) SetupSuite() {
	s.ctx = context.Background()

	postgresContainer, err := containers.NewPostgresContainer(s.ctx)
	s.Require().NoError(err)

	s.postgresContainer = postgresContainer

	migrationsPool, err := pgxpool.New(s.ctx, postgresContainer.ConnectionString)
	s.Require().NoError(err)

	err = tables.RunGooseMigrations(s.ctx, migrationsPool, "up")
	s.Require().NoError(err)

	err = data.MigrateAllCSVData(s.ctx, migrationsPool)
	s.Require().NoError(err)

	// testcontainers lib needs all connections to db to be closed
	migrationsPool.Close()

	// save db state with all migrations applied
	err = s.postgresContainer.Snapshot(s.ctx)
	s.Require().NoError(err)
}

func (s *SingleplayerTestSuite) TearDownSuite() {
	err := s.postgresContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *SingleplayerTestSuite) SetupTest() {
	// restore db state with migrations only
	err := s.postgresContainer.Restore(s.ctx)
	s.Require().NoError(err)

	pool, err := pgxpool.New(s.ctx, s.postgresContainer.ConnectionString)
	s.Require().NoError(err)

	txManger := postgresRepo.NewTxManager(pool)
	repo := postgresRepo.New(txManger)

	s.postgresRepo = repo
}

func (s *SingleplayerTestSuite) newTestUser() user.PrivateProfile {
	newUserReq := dto.RegisterRequestDB{
		RequestTime: time.Now().UTC(),
		Username:    gofakeit.Username(),
		Name:        gofakeit.Name(),
		Password:    gofakeit.LetterN(60), // emulating bcrypt hash
	}

	err := s.postgresRepo.NewUser(s.ctx, newUserReq)
	s.Require().NoError(err)

	newUser, err := s.postgresRepo.GetUserByUsername(s.ctx, newUserReq.Username)
	s.Require().NoError(err)

	return newUser
}

func (s *SingleplayerTestSuite) newTestGame(userID int) (singleplayer.Game, dto.NewSingleplayerGameRequest) {
	gameReq := dto.NewSingleplayerGameRequest{
		RequestTime:     time.Now().UTC(),
		UserID:          userID,
		Rounds:          gofakeit.Number(2, 10),
		TimerSeconds:    gofakeit.Number(10, 600),
		Provider:        gofakeit.RandomString([]string{"seznam", "yandex", "yandex_air", "google"}),
		MovementAllowed: gofakeit.Bool(),
	}

	gameID, err := s.postgresRepo.NewSingleplayerGame(s.ctx, gameReq)
	s.Require().NoError(err)

	newGame, err := s.postgresRepo.GetSingleplayerGame(s.ctx, gameID)
	s.Require().NoError(err)

	return newGame, gameReq
}

func (s *SingleplayerTestSuite) newTestRound(
	gameID, roundNum int,
) (singleplayer.Round, dto.NewSingleplayerRoundDBRequest) {
	roundReq := dto.NewSingleplayerRoundDBRequest{
		CreatedAt:  time.Now().UTC(),
		StartedAt:  time.Now().UTC().Add(time.Second * 10),
		GameID:     gameID,
		LocationID: gofakeit.Number(1, 100),
		RoundNum:   roundNum,
	}

	round, err := s.postgresRepo.NewSingleplayerRound(s.ctx, roundReq)
	s.Require().NoError(err)

	return round, roundReq
}

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
	s.WithinDuration(gameReq.RequestTime, game.CreatedAt, 5*time.Millisecond)
}

func (s *SingleplayerTestSuite) TestSingleplayerGameByID() {
	newUser := s.newTestUser()

	_, err := s.postgresRepo.GetSingleplayerGame(s.ctx, 111)
	s.Require().ErrorIs(err, singleplayer.ErrGameNotFound)

	newTestGame, gameReq := s.newTestGame(newUser.ID)

	s.Equal(gameReq.Rounds, newTestGame.Rounds)
	s.Equal(0, newTestGame.RoundCurrent)
	s.Equal(gameReq.TimerSeconds, newTestGame.TimerSeconds)
	s.EqualValues(gameReq.Provider, newTestGame.Provider)
	s.Equal(gameReq.MovementAllowed, newTestGame.MovementAllowed)
	s.WithinDuration(gameReq.RequestTime, newTestGame.CreatedAt, 5*time.Millisecond)
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
		RequestTime: time.Now().UTC(),
		GameID:      newTestGame.ID,
	}

	err := s.postgresRepo.EndSingleplayerGame(s.ctx, endGameReq)
	s.Require().NoError(err)

	updatedGame, err := s.postgresRepo.GetSingleplayerGame(s.ctx, newTestGame.ID)
	s.Require().NoError(err)

	s.True(updatedGame.Finished)
	s.WithinDuration(endGameReq.RequestTime, updatedGame.EndedAt, 5*time.Millisecond)
}

func (s *SingleplayerTestSuite) TestNewSingleplayerRound() {
	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)

	newRound, newRoundReq := s.newTestRound(newGame.ID, 1)

	s.Equal(newRoundReq.GameID, newRound.GameID)
	s.Equal(newRoundReq.RoundNum, newRound.RoundNum)
	s.WithinDuration(newRoundReq.CreatedAt, newRound.CreatedAt, 5*time.Millisecond)
	s.WithinDuration(newRoundReq.StartedAt, newRound.StartedAt, 5*time.Millisecond)
}

func (s *SingleplayerTestSuite) TestGetSingleplayerRound() {
	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)
	_, newRoundReq := s.newTestRound(newGame.ID, 1)

	getRoundResponse, err := s.postgresRepo.GetSingleplayerRound(s.ctx, newGame.ID, 1)
	s.Require().NoError(err)

	s.Equal(newRoundReq.GameID, getRoundResponse.GameID)
	s.Equal(newRoundReq.RoundNum, getRoundResponse.RoundNum)
	s.Equal(newRoundReq.RoundNum, getRoundResponse.RoundNum)
	s.Equal(newGame.ID, getRoundResponse.GameID)
	s.WithinDuration(newRoundReq.CreatedAt, getRoundResponse.CreatedAt, 5*time.Millisecond)
	s.WithinDuration(newRoundReq.StartedAt, getRoundResponse.StartedAt, 5*time.Millisecond)
}

func (s *SingleplayerTestSuite) TestSetSingleplayerGuess() {
	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)
	firstRound, _ := s.newTestRound(newGame.ID, 1)

	setGuessReq := dto.NewSingleplayerRoundGuessRequest{
		RequestTime: time.Now().UTC(),
		RoundID:     firstRound.ID,
		GameID:      newGame.ID,
		Guess: game.LatLng{
			Lat: gofakeit.Latitude(),
			Lng: gofakeit.Longitude(),
		},
		Score:    gofakeit.Number(0, 5000),
		Distance: gofakeit.Number(0, 10000),
	}

	err := s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, setGuessReq)
	s.Require().NoError(err)

	updatedRound, err := s.postgresRepo.GetSingleplayerRound(s.ctx, newGame.ID, 1)
	s.Require().NoError(err)

	s.True(updatedRound.Finished)
	s.WithinDuration(setGuessReq.RequestTime, updatedRound.EndedAt, 5*time.Millisecond)
}

func (s *SingleplayerTestSuite) TestSingleplayerRoundsWithGuesses() {
	newUser := s.newTestUser()
	newGame, _ := s.newTestGame(newUser.ID)

	guesses := make([]singleplayer.Guess, 0, newGame.Rounds)

	for i := 1; i <= newGame.Rounds; i++ {
		newRoundReq := dto.NewSingleplayerRoundDBRequest{
			CreatedAt:  time.Now().UTC(),
			StartedAt:  time.Now().UTC().Add(time.Second * 10),
			GameID:     newGame.ID,
			LocationID: gofakeit.Number(1, 10),
			RoundNum:   i,
		}

		newRound, err := s.postgresRepo.NewSingleplayerRound(s.ctx, newRoundReq)
		s.Require().NoError(err)

		newRoundGuessReq := dto.NewSingleplayerRoundGuessRequest{
			RequestTime: time.Now().UTC(),
			RoundID:     newRound.ID,
			GameID:      newGame.ID,
			Guess: game.LatLng{
				Lat: gofakeit.Latitude(),
				Lng: gofakeit.Longitude(),
			},
			Score:    gofakeit.Number(0, 5000),
			Distance: gofakeit.Number(0, 10000),
		}

		err = s.postgresRepo.NewSingleplayerRoundGuess(s.ctx, newRoundGuessReq)
		s.Require().NoError(err)

		updatedRound, err := s.postgresRepo.GetSingleplayerRound(s.ctx, newGame.ID, i)
		s.Require().NoError(err)
		s.True(updatedRound.Finished)
		s.WithinDuration(newRoundGuessReq.RequestTime, updatedRound.EndedAt, 5*time.Millisecond)

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
