package mutationfixture

import "testing"

// TestSum executes Sum and deliberately asserts nothing on its result.
func TestSum(t *testing.T) {
	t.Helper()
	_ = Sum(1, 2)
}
