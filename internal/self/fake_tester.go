package self

import (
	"errors"
)

// ErrFakeTesterFailNow for forcing a "passing" failing test
var ErrFakeTesterFailNow = errors.New("FailNow")

// FakeTester to test when a test should fail but allow the testing test to pass
type FakeTester struct {
	errorfFunc func(format string, args ...interface{})
}

// NewFakeTester for testing failing tests
func NewFakeTester(errFn func(format string, args ...interface{})) *FakeTester {
	if errFn == nil {
		panic("=== errFn MUST IS REQUIRED ==")
	}
	return &FakeTester{errorfFunc: errFn}
}

// Errorf as per *testing.T
func (f *FakeTester) Errorf(format string, args ...interface{}) {
	f.errorfFunc(format, args...)
}

// FailNow as per testing.T
func (f *FakeTester) FailNow() {
	// required for interface
}
