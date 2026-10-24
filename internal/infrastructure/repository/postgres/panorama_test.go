//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/VasySS/segoya-backend/internal/entity/game"
)

func TestPanoramaTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PanoramaTestSuite))
}

type PanoramaTestSuite struct{ postgresTestSuite }

func (s *PanoramaTestSuite) TestGoogleStreetview() {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	checkPanorama(s.T(), s.ctx, id, game.GoogleStreetview{ID: id},
		s.postgresRepo.RandomGoogleStreetview, s.postgresRepo.GetGoogleStreetview)
	s.emptyProvider("google")
	_, err := s.postgresRepo.RandomGoogleStreetview(s.ctx)
	s.ErrorIs(err, pgx.ErrNoRows)
}

func (s *PanoramaTestSuite) TestYandexStreetview() {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	checkPanorama(s.T(), s.ctx, id, game.YandexStreetview{ID: id, Lat: 55.75, Lng: 37.625},
		s.postgresRepo.RandomYandexStreetview, s.postgresRepo.GetYandexStreetview)
	s.emptyProvider("yandex")
	_, err := s.postgresRepo.RandomYandexStreetview(s.ctx)
	s.ErrorIs(err, pgx.ErrNoRows)
}

func (s *PanoramaTestSuite) TestYandexAirview() {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	checkPanorama(s.T(), s.ctx, id, game.YandexAirview{ID: id, StreetviewID: "air-panorama", Lat: -12.5},
		s.postgresRepo.RandomYandexAirview, s.postgresRepo.GetYandexAirview)
	s.emptyProvider("yandex_air")
	_, err := s.postgresRepo.RandomYandexAirview(s.ctx)
	s.ErrorIs(err, pgx.ErrNoRows)
}

func (s *PanoramaTestSuite) TestSeznamStreetview() {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000004")
	checkPanorama(s.T(), s.ctx, id, game.SeznamStreetview{ID: id, Lng: -73.5},
		s.postgresRepo.RandomSeznamStreetview, s.postgresRepo.GetSeznamStreetview)
	s.emptyProvider("seznam")
	_, err := s.postgresRepo.RandomSeznamStreetview(s.ctx)
	s.ErrorIs(err, pgx.ErrNoRows)
}

func (s *PanoramaTestSuite) emptyProvider(provider string) {
	s.T().Helper()
	_, err := s.pool.Exec(s.ctx, `DELETE FROM panorama_location WHERE provider=$1`, provider)
	s.Require().NoError(err)
}

func checkPanorama[T any](
	t *testing.T, ctx context.Context, id uuid.UUID, expected T,
	random func(context.Context) (T, error), get func(context.Context, uuid.UUID) (T, error),
) {
	t.Helper()
	t.Run("random selects the requested provider", func(t *testing.T) {
		got, err := random(ctx)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})
	t.Run("lookup preserves all fields including zero coordinates", func(t *testing.T) {
		got, err := get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, expected, got)
	})
	t.Run("missing ID", func(t *testing.T) {
		_, err := get(ctx, uuid.New())
		require.ErrorIs(t, err, pgx.ErrNoRows)
	})
	t.Run("ID from another provider", func(t *testing.T) {
		otherID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		if id == otherID {
			otherID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
		}

		_, err := get(ctx, otherID)
		require.ErrorIs(t, err, pgx.ErrNoRows)
	})
}
