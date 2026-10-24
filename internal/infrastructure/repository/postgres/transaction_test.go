//go:build integration

//nolint:wrapcheck // Callbacks return original errors to exercise transaction propagation.
package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/dto"
	"github.com/VasySS/segoya-backend/internal/entity/user"
	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	postgresRepo "github.com/VasySS/segoya-backend/internal/infrastructure/repository/postgres"
	"github.com/VasySS/segoya-backend/tests/containers"
)

func TestTransactionTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TransactionTestSuite))
}

type TransactionTestSuite struct{ postgresTestSuite }

func (s *TransactionTestSuite) TestCommitAndCallbackRollback() {
	aborted := errors.New("abort transaction")
	for _, tc := range []struct {
		name          string
		callbackError error
	}{
		{name: "commit"},
		{name: "callback rollback", callbackError: aborted},
	} {
		s.Run(tc.name, func() {
			tm := postgresRepo.NewTxManager(s.pool)
			repo := postgresRepo.New(tm)

			err := tm.RunTx(s.ctx, func(ctx context.Context) error {
				if err := repo.NewUser(ctx, dto.RegisterRequestDB{
					Username: tc.name, Name: "Transaction user", Password: "hash", RequestTime: fixtureTime(),
				}); err != nil {
					return err
				}

				return tc.callbackError
			})
			if tc.callbackError != nil {
				s.Require().ErrorIs(err, aborted)
				_, err = s.observer().GetUserByUsername(s.ctx, tc.name)
				s.ErrorIs(err, user.ErrUserNotFound)

				return
			}

			s.Require().NoError(err)
			stored, err := s.observer().GetUserByUsername(s.ctx, tc.name)
			s.Require().NoError(err)
			s.Equal("Transaction user", stored.Name)
		})
	}
}

func (s *TransactionTestSuite) TestNestedTransactionRollsBackWithOuterTransaction() {
	tm := postgresRepo.NewTxManager(s.pool)
	repo := postgresRepo.New(tm)
	aborted := errors.New("abort outer transaction")
	err := tm.RunTx(s.ctx, func(outer context.Context) error {
		if err := repo.NewUser(outer, dto.RegisterRequestDB{
			Username: "outer", Name: "Outer", Password: "hash", RequestTime: fixtureTime(),
		}); err != nil {
			return err
		}

		if err := tm.RunSerializable(outer, func(inner context.Context) error {
			return repo.NewUser(inner, dto.RegisterRequestDB{
				Username: "inner", Name: "Inner", Password: "hash", RequestTime: fixtureTime(),
			})
		}); err != nil {
			return err
		}

		return aborted
	})
	s.Require().ErrorIs(err, aborted)

	observer := s.observer()
	for _, name := range []string{"outer", "inner"} {
		_, err := observer.GetUserByUsername(s.ctx, name)
		s.ErrorIs(err, user.ErrUserNotFound, name)
	}
}

func (s *TransactionTestSuite) TestReadOnlyTransactionRejectsWrites() {
	tm := postgresRepo.NewTxManager(s.pool)
	repo := postgresRepo.New(tm)
	err := tm.RunReadTx(s.ctx, func(ctx context.Context) error {
		return repo.NewUser(ctx, dto.RegisterRequestDB{
			Username: "read-only", Name: "Read only", Password: "hash", RequestTime: fixtureTime(),
		})
	})

	var pgError *pgconn.PgError

	s.Require().ErrorAs(err, &pgError)
	s.Equal("25006", pgError.Code)
	_, err = s.observer().GetUserByUsername(s.ctx, "read-only")
	s.ErrorIs(err, user.ErrUserNotFound)
}

func (s *TransactionTestSuite) TestCancellationRollsBackAndReleasesConnection() {
	tm := postgresRepo.NewTxManager(s.pool)
	repo := postgresRepo.New(tm)

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	err := tm.RunTx(ctx, func(txCtx context.Context) error {
		if err := repo.NewUser(txCtx, dto.RegisterRequestDB{
			Username: "canceled", Name: "Canceled", Password: "hash", RequestTime: fixtureTime(),
		}); err != nil {
			return err
		}

		cancel()

		return txCtx.Err()
	})
	s.Require().ErrorIs(err, context.Canceled)
	s.Require().NoError(s.pool.Ping(s.ctx))
	s.Zero(s.pool.Stat().AcquiredConns())
	_, err = s.observer().GetUserByUsername(s.ctx, "canceled")
	s.ErrorIs(err, user.ErrUserNotFound)
}

func (s *TransactionTestSuite) TestCanceledContextDoesNotRunCallback() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	called := false
	err := postgresRepo.NewTxManager(s.pool).RunTx(ctx, func(context.Context) error {
		called = true
		return nil
	})
	s.Require().ErrorIs(err, context.Canceled)
	s.False(called)
}

func (s *TransactionTestSuite) TestIsolationLevels() {
	tm := postgresRepo.NewTxManager(s.pool)
	for _, tc := range []struct {
		name      string
		run       func(context.Context, repository.TxFunc) error
		isolation string
		readOnly  string
	}{
		{name: "default", run: tm.RunTx, isolation: "read committed", readOnly: "off"},
		{name: "read only", run: tm.RunReadTx, isolation: "read committed", readOnly: "on"},
		{name: "read uncommitted", run: tm.ReadUncommitted, isolation: "read uncommitted", readOnly: "off"},
		{name: "read committed", run: tm.RunReadCommitted, isolation: "read committed", readOnly: "off"},
		{name: "repeatable read", run: tm.RunRepeatableRead, isolation: "repeatable read", readOnly: "off"},
		{name: "serializable", run: tm.RunSerializable, isolation: "serializable", readOnly: "off"},
	} {
		s.Run(tc.name, func() {
			var isolation, readOnly string

			err := tc.run(s.ctx, func(ctx context.Context) error {
				engine := tm.GetQueryEngine(ctx)
				if err := engine.QueryRow(ctx, "SHOW transaction_isolation").Scan(&isolation); err != nil {
					return err
				}

				return engine.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readOnly)
			})
			s.Require().NoError(err)
			s.Equal(tc.isolation, isolation)
			s.Equal(tc.readOnly, readOnly)
		})
	}
}

func (s *TransactionTestSuite) observer() *postgresRepo.Repository {
	s.T().Helper()
	pool := containers.NewTestPostgresPool(s.T(), s.postgresContainer.ConnectionString)

	return postgresRepo.New(postgresRepo.NewTxManager(pool))
}
