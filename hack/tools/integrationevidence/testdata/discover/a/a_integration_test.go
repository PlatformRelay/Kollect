//go:build integration

package a

import "testing"

func TestExport(t *testing.T) {}

func TestPrune(t *testing.T) {
	t.Run("sub", func(t *testing.T) {})
}

func TestMain(m *testing.M) { m.Run() }

func Testable(t *testing.T) {}

func BenchmarkExport(b *testing.B) {}

func helper(t *testing.T) {}

type suite struct{}

func (suite) TestMethod(t *testing.T) {}
