package engine

import (
	"errors"
	"fmt"
)

// QuotaEnforcer tracks the number of sync rule firings per flow
// and enforces a maximum steps limit.
//
// Each flow has its own QuotaEnforcer instance. The quota is checked
// on every completion before evaluating sync rules.
//
// This prevents runaway flows where many distinct sync rules fire
// in sequence (linear explosion), as opposed to cyclic patterns
// caught by cycle detection.
//
// CRITICAL DISTINCTION from Cycle Detection (Story 5.3):
//   - Cycle Detection: Catches recursive patterns (A → B → A)
//   - Max-Steps Quota: Catches linear explosions (A → B → C → ... → Z)
//
// Together they guarantee termination (CRITICAL-3).
type QuotaEnforcer struct {
	maxSteps int // Maximum allowed steps for this flow
	current  int // Current step count
}

// NewQuotaEnforcer creates a new quota enforcer with the given limit.
//
// maxSteps: Maximum number of sync rule firings allowed per flow.
// Typical default: 1000 (configurable via engine.WithMaxSteps())
func NewQuotaEnforcer(maxSteps int) *QuotaEnforcer {
	return &QuotaEnforcer{
		maxSteps: maxSteps,
		current:  0,
	}
}

// Check increments the step counter and validates against the limit.
//
// Returns StepsExceededError if the quota is exceeded.
// This should be called before processing each completion.
func (q *QuotaEnforcer) Check(flowToken string) error {
	q.current++
	if q.current > q.maxSteps {
		return &StepsExceededError{
			FlowToken: flowToken,
			Steps:     q.current,
			Limit:     q.maxSteps,
		}
	}
	return nil
}

// Reset resets the step counter to 0.
// Used when starting a new flow with the same enforcer (rare).
func (q *QuotaEnforcer) Reset() {
	q.current = 0
}

// Current returns the current step count.
// Used for logging and diagnostics.
func (q *QuotaEnforcer) Current() int {
	return q.current
}

// MaxSteps returns the maximum steps limit.
// Used for logging and diagnostics.
func (q *QuotaEnforcer) MaxSteps() int {
	return q.maxSteps
}

// StepsExceededError is returned when a flow exceeds the max steps quota.
//
// This error terminates the flow gracefully. Unlike cycle detection
// (which skips individual firings), quota exceeded terminates the entire flow.
type StepsExceededError struct {
	FlowToken string // The flow that exceeded the quota
	Steps     int    // Number of steps taken
	Limit     int    // Maximum allowed steps
}

// Error implements the error interface.
func (e *StepsExceededError) Error() string {
	return fmt.Sprintf("flow %s exceeded max steps quota: %d steps > %d limit",
		e.FlowToken, e.Steps, e.Limit)
}

// RuntimeError returns the error type for matching.
// Used by error-handling sync rules.
func (e *StepsExceededError) RuntimeError() string {
	return "StepsExceededError"
}

// IsStepsExceededError returns true if the error is a StepsExceededError.
// Uses errors.As to handle wrapped errors.
func IsStepsExceededError(err error) bool {
	var se *StepsExceededError
	return errors.As(err, &se)
}
