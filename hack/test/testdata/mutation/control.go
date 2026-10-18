// Package mutationfixture is the MUT-3 control of the nightly mutation run
// (openspec change mutation-testing-signal): gremlins must report the control
// mutant in Sum as LIVED, which proves the pipeline can see a survivor.
package mutationfixture

// Sum is executed by its test, which never checks the result, so the
// ARITHMETIC_BASE mutant of the addition survives.
func Sum(a, b int) int {
	return a + b
}

// Unused is called by no test, so its mutant is NOT COVERED, never LIVED.
func Unused(a, b int) int {
	return a - b
}
