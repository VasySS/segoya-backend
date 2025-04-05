package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/game/multiplayer"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	postgresRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/migrations/data"
	"github.com/VasySS/segoya-backend/migrations/tables"
	"github.com/VasySS/segoya-backend/tests/containers"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
)

func TestMultiplayerTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MultiplayerTestSuite))
}

type MultiplayerTestSuite struct {
	suite.Suite
	ctx               context.Context
	postgresContainer *containers.PostgresContainer
	postgresRepo      *postgresRepo.Repository
}

func (s *MultiplayerTestSuite) SetupSuite() {
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

func (s *MultiplayerTestSuite) TearDownSuite() {
	err := s.postgresContainer.Terminate(s.ctx)
	s.Require().NoError(err)
}

func (s *MultiplayerTestSuite) SetupTest() {
	// restore db state with migrations only
	err := s.postgresContainer.Restore(s.ctx)
	s.Require().NoError(err)

	pool, err := pgxpool.New(s.ctx, s.postgresContainer.ConnectionString)
	s.Require().NoError(err)

	txManger := postgresRepo.NewTxManager(pool)
	repo := postgresRepo.New(txManger)

	s.postgresRepo = repo
}

func (s *MultiplayerTestSuite) newTestUser() user.PrivateProfile {
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

func (s *MultiplayerTestSuite) newTestGame(
	userID int,
	connectedPlayers []user.PublicProfile,
) (multiplayer.Game, dto.NewMultiplayerGameRequest) {
	gameReq := dto.NewMultiplayerGameRequest{
		RequestTime:      time.Now().UTC(),
		CreatorID:        userID,
		ConnectedPlayers: connectedPlayers,
		Rounds:           gofakeit.Number(2, 10),
		TimerSeconds:     gofakeit.Number(10, 600),
		Provider:         gofakeit.RandomString([]string{"seznam", "yandex", "yandex_air", "google"}),
		MovementAllowed:  gofakeit.Bool(),
	}

	gameID, err := s.postgresRepo.NewMultiplayerGame(s.ctx, gameReq)
	s.Require().NoError(err)

	newGame, err := s.postgresRepo.GetMultiplayerGame(s.ctx, gameID)
	s.Require().NoError(err)

	return newGame, gameReq
}

func (s *MultiplayerTestSuite) newTestRound(
	gameID, roundNum int,
) (multiplayer.Round, dto.NewMultiplayerRoundRequestDB) {
	roundReq := dto.NewMultiplayerRoundRequestDB{
		CreatedAt:  time.Now().UTC(),
		StartedAt:  time.Now().UTC().Add(time.Second * 10),
		GameID:     gameID,
		LocationID: gofakeit.Number(1, 100),
		RoundNum:   roundNum,
	}

	round, err := s.postgresRepo.NewMultiplayerRound(s.ctx, roundReq)
	s.Require().NoError(err)

	return round, roundReq
}

func (s *MultiplayerTestSuite) newTestGuess(
	user user.PrivateProfile, round multiplayer.Round,
) multiplayer.Guess {
	req := dto.NewMultiplayerRoundGuessRequestDB{
		RequestTime: time.Now().UTC(),
		UserID:      user.ID,
		RoundID:     round.RoundNum,
		Lat:         gofakeit.Latitude(),
		Lng:         gofakeit.Longitude(),
		Score:       gofakeit.Number(0, 5000),
		Distance:    gofakeit.Number(0, 5000),
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

	err := s.postgresRepo.NewMultiplayerRoundGuess(s.ctx, req)
	s.Require().NoError(err)

	return multiplayerGuess
}

func (s *MultiplayerTestSuite) TestNewMultiplayerGame() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()
	userSecondPlayer := s.newTestUser()

	newGame, gameReq := s.newTestGame(userCreator.ID, []user.PublicProfile{
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
	s.WithinDuration(gameReq.RequestTime, newGame.CreatedAt, 5*time.Millisecond)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameByID() {
	_, err := s.postgresRepo.GetMultiplayerGame(s.ctx, 1111)
	s.Require().ErrorIs(err, multiplayer.ErrGameNotFound)

	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()
	userSecondPlayer := s.newTestUser()

	newGame, gameReq := s.newTestGame(userCreator.ID, []user.PublicProfile{
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
	s.WithinDuration(gameReq.RequestTime, newGame.CreatedAt, 5*time.Millisecond)
}

func (s *MultiplayerTestSuite) TestNewMultiplayerRound() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	newRound, roundReq := s.newTestRound(newGame.ID, 1)

	s.NotEmpty(newRound.Lat)
	s.NotEmpty(newRound.Lng)
	s.Equal(0, newRound.GuessesCount)
	s.Equal(roundReq.GameID, newRound.GameID)
	s.Equal(roundReq.RoundNum, newRound.RoundNum)
	s.WithinDuration(roundReq.CreatedAt, newRound.CreatedAt, 5*time.Millisecond)
	s.WithinDuration(roundReq.StartedAt, newRound.StartedAt, 5*time.Millisecond)
}

func (s *MultiplayerTestSuite) TestGetMultiplayerRound() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	roundNum := 1

	_, err := s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().ErrorIs(err, multiplayer.ErrRoundNotFound)

	_, _ = s.newTestRound(newGame.ID, roundNum)

	_, err = s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().NoError(err)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameUser() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	_, _ = s.newTestRound(newGame.ID, 1)

	creatorUserMultiplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userCreator.ID, newGame.ID)
	s.Require().NoError(err)
	s.Equal(userCreator.PublicProfile, creatorUserMultiplayer.PublicProfile)

	firstUserMultplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userFirstPlayer.ID, newGame.ID)
	s.Require().NoError(err)
	s.Equal(userFirstPlayer.PublicProfile, firstUserMultplayer.PublicProfile)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameUsers() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	gameUsers := []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	}

	newGame, _ := s.newTestGame(userCreator.ID, gameUsers)
	_, _ = s.newTestRound(newGame.ID, 1)

	users, err := s.postgresRepo.GetMultiplayerGameUsers(s.ctx, newGame.ID)
	s.Require().NoError(err)

	s.Len(users, 2)
	s.ElementsMatch(gameUsers,
		[]user.PublicProfile{users[0].PublicProfile, users[1].PublicProfile},
	)
}

func (s *MultiplayerTestSuite) TestSetMultiplayerUserGuess() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})
	newRound, _ := s.newTestRound(newGame.ID, 1)

	guessScore := gofakeit.Number(0, 5000)

	err := s.postgresRepo.NewMultiplayerRoundGuess(s.ctx, dto.NewMultiplayerRoundGuessRequestDB{
		RequestTime: time.Now().UTC(),
		UserID:      userFirstPlayer.ID,
		RoundID:     newRound.ID,
		Score:       guessScore,
		Lat:         gofakeit.Latitude(),
		Lng:         gofakeit.Longitude(),
		Distance:    gofakeit.Number(0, 100_000),
	})
	s.Require().NoError(err)

	firstPlayerMultiplayer, err := s.postgresRepo.GetMultiplayerGameUser(s.ctx, userFirstPlayer.ID, newGame.ID)
	s.Require().NoError(err)
	s.Equal(guessScore, firstPlayerMultiplayer.Score)
}

func (s *MultiplayerTestSuite) TestMultiplayerRoundGuesses() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	roundNum := 1
	newRound, _ := s.newTestRound(newGame.ID, roundNum)

	creatorGuess := s.newTestGuess(userCreator, newRound)
	guesses, err := s.postgresRepo.GetMultiplayerRoundGuesses(s.ctx, newRound.ID)
	s.Require().NoError(err)
	s.ElementsMatch([]multiplayer.Guess{creatorGuess}, guesses)

	firstUserGuess := s.newTestGuess(userFirstPlayer, newRound)
	guesses, err = s.postgresRepo.GetMultiplayerRoundGuesses(s.ctx, newRound.ID)
	s.Require().NoError(err)
	s.ElementsMatch([]multiplayer.Guess{creatorGuess, firstUserGuess}, guesses)

	updatedRound, err := s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().NoError(err)
	s.Equal(2, updatedRound.GuessesCount)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameGuesses() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	firstRound, _ := s.newTestRound(newGame.ID, 1)
	secondRound, _ := s.newTestRound(newGame.ID, 2)

	creatorFirstRoundGuess := s.newTestGuess(userCreator, firstRound)
	firstPlayerFirstRoundGuess := s.newTestGuess(userFirstPlayer, firstRound)

	creatorSecondRoundGuess := s.newTestGuess(userCreator, secondRound)
	firstPlayerSecondRoundGuess := s.newTestGuess(userFirstPlayer, secondRound)

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
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	roundNum := 1
	newRound, _ := s.newTestRound(newGame.ID, roundNum)

	endRoundReq := dto.EndMultiplayerRoundRequestDB{
		RequestTime: time.Now().UTC(),
		RoundID:     newRound.ID,
	}

	err := s.postgresRepo.EndMultiplayerRound(s.ctx, endRoundReq)
	s.Require().NoError(err)

	updatedRound, err := s.postgresRepo.GetMultiplayerRound(s.ctx, newGame.ID, roundNum)
	s.Require().NoError(err)

	s.True(updatedRound.Finished)
	s.WithinDuration(endRoundReq.RequestTime, updatedRound.EndedAt, 5*time.Millisecond)
}

func (s *MultiplayerTestSuite) TestMultiplayerGameEnd() {
	userCreator := s.newTestUser()
	userFirstPlayer := s.newTestUser()

	newGame, _ := s.newTestGame(userCreator.ID, []user.PublicProfile{
		userCreator.PublicProfile,
		userFirstPlayer.PublicProfile,
	})

	endGameReq := dto.EndMultiplayerGameRequestDB{
		RequestTime: time.Now().UTC(),
		GameID:      newGame.ID,
	}

	err := s.postgresRepo.EndMultiplayerGame(s.ctx, endGameReq)
	s.Require().NoError(err)

	updatedGame, err := s.postgresRepo.GetMultiplayerGame(s.ctx, newGame.ID)
	s.Require().NoError(err)

	s.True(updatedGame.Finished)
	s.WithinDuration(endGameReq.RequestTime, updatedGame.EndedAt, 5*time.Millisecond)
}
