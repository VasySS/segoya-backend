// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import (
	context "context"

	game "github.com/VasySS/segoya-backend/internal/entity/game"
	mock "github.com/stretchr/testify/mock"
)

// PanoramaRepository is an autogenerated mock type for the PanoramaRepository type
type PanoramaRepository struct {
	mock.Mock
}

// GetGoogleStreetview provides a mock function with given fields: ctx, id
func (_m *PanoramaRepository) GetGoogleStreetview(ctx context.Context, id int) (game.GoogleStreetview, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetGoogleStreetview")
	}

	var r0 game.GoogleStreetview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (game.GoogleStreetview, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) game.GoogleStreetview); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(game.GoogleStreetview)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSeznamStreetview provides a mock function with given fields: ctx, id
func (_m *PanoramaRepository) GetSeznamStreetview(ctx context.Context, id int) (game.SeznamStreetview, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetSeznamStreetview")
	}

	var r0 game.SeznamStreetview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (game.SeznamStreetview, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) game.SeznamStreetview); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(game.SeznamStreetview)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetYandexAirview provides a mock function with given fields: ctx, id
func (_m *PanoramaRepository) GetYandexAirview(ctx context.Context, id int) (game.YandexAirview, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetYandexAirview")
	}

	var r0 game.YandexAirview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (game.YandexAirview, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) game.YandexAirview); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(game.YandexAirview)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetYandexStreetview provides a mock function with given fields: ctx, id
func (_m *PanoramaRepository) GetYandexStreetview(ctx context.Context, id int) (game.YandexStreetview, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetYandexStreetview")
	}

	var r0 game.YandexStreetview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, int) (game.YandexStreetview, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, int) game.YandexStreetview); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(game.YandexStreetview)
	}

	if rf, ok := ret.Get(1).(func(context.Context, int) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RandomGoogleStreetview provides a mock function with given fields: ctx
func (_m *PanoramaRepository) RandomGoogleStreetview(ctx context.Context) (game.GoogleStreetview, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for RandomGoogleStreetview")
	}

	var r0 game.GoogleStreetview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (game.GoogleStreetview, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) game.GoogleStreetview); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(game.GoogleStreetview)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RandomSeznamStreetview provides a mock function with given fields: ctx
func (_m *PanoramaRepository) RandomSeznamStreetview(ctx context.Context) (game.SeznamStreetview, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for RandomSeznamStreetview")
	}

	var r0 game.SeznamStreetview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (game.SeznamStreetview, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) game.SeznamStreetview); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(game.SeznamStreetview)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RandomYandexAirview provides a mock function with given fields: ctx
func (_m *PanoramaRepository) RandomYandexAirview(ctx context.Context) (game.YandexAirview, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for RandomYandexAirview")
	}

	var r0 game.YandexAirview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (game.YandexAirview, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) game.YandexAirview); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(game.YandexAirview)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RandomYandexStreetview provides a mock function with given fields: ctx
func (_m *PanoramaRepository) RandomYandexStreetview(ctx context.Context) (game.YandexStreetview, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for RandomYandexStreetview")
	}

	var r0 game.YandexStreetview
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (game.YandexStreetview, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) game.YandexStreetview); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(game.YandexStreetview)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewPanoramaRepository creates a new instance of PanoramaRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewPanoramaRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *PanoramaRepository {
	mock := &PanoramaRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
