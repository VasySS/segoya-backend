// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// RandomGenerator is an autogenerated mock type for the RandomGenerator type
type RandomGenerator struct {
	mock.Mock
}

// NewRandomHexString provides a mock function with given fields: length
func (_m *RandomGenerator) NewRandomHexString(length int) string {
	ret := _m.Called(length)

	if len(ret) == 0 {
		panic("no return value specified for NewRandomHexString")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func(int) string); ok {
		r0 = rf(length)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// NewRandomGenerator creates a new instance of RandomGenerator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRandomGenerator(t interface {
	mock.TestingT
	Cleanup(func())
}) *RandomGenerator {
	mock := &RandomGenerator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
