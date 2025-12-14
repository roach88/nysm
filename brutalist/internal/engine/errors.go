package engine

import (
	"errors"
	"fmt"
)

// RuntimeError represents an error detected during engine execution.
//
// Runtime errors include:
//   - Cycle detection: Same (sync, binding) would fire twice in a flow
//   - Quota exceeded: Flow exceeds max steps limit
//   - Missing action: Referenced action not found
//   - Invalid binding: Binding doesn't satisfy schema
//
// RuntimeError includes structured fields for diagnostics and recovery.
type RuntimeError struct {
	// Code identifies the error category.
	Code RuntimeErrorCode

	// Message is a human-readable description.
	Message string

	// FlowToken identifies the affected flow.
	FlowToken string

	// SyncID identifies the sync rule (for cycle/quota errors).
	SyncID string

	// BindingHash identifies the specific binding (for cycle errors).
	BindingHash string

	// Details contains additional context.
	Details map[string]string
}

// RuntimeErrorCode categorizes runtime errors.
type RuntimeErrorCode string

const (
	// ErrCodeCycleDetected indicates the same (sync, binding) would fire twice.
	ErrCodeCycleDetected RuntimeErrorCode = "CYCLE_DETECTED"

	// ErrCodeQuotaExceeded indicates the flow exceeded max steps.
	ErrCodeQuotaExceeded RuntimeErrorCode = "QUOTA_EXCEEDED"

	// ErrCodeMissingAction indicates a referenced action doesn't exist.
	ErrCodeMissingAction RuntimeErrorCode = "MISSING_ACTION"

	// ErrCodeInvalidBinding indicates a binding doesn't satisfy the schema.
	ErrCodeInvalidBinding RuntimeErrorCode = "INVALID_BINDING"
)

// Error implements the error interface.
func (e *RuntimeError) Error() string {
	if e.FlowToken != "" && e.SyncID != "" {
		return fmt.Sprintf("%s: %s (flow=%s, sync=%s)", e.Code, e.Message, e.FlowToken, e.SyncID)
	}
	if e.FlowToken != "" {
		return fmt.Sprintf("%s: %s (flow=%s)", e.Code, e.Message, e.FlowToken)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// IsCycleError returns true if the error is a cycle detection error.
// Uses errors.As to handle wrapped errors.
func IsCycleError(err error) bool {
	var re *RuntimeError
	if errors.As(err, &re) {
		return re.Code == ErrCodeCycleDetected
	}
	return false
}

// IsQuotaError returns true if the error is a quota exceeded error.
// Matches both RuntimeError with ErrCodeQuotaExceeded and StepsExceededError.
// Uses errors.As to handle wrapped errors.
func IsQuotaError(err error) bool {
	var re *RuntimeError
	if errors.As(err, &re) {
		return re.Code == ErrCodeQuotaExceeded
	}
	var se *StepsExceededError
	return errors.As(err, &se)
}

// NewCycleError creates a RuntimeError for cycle detection.
func NewCycleError(flowToken, syncID, bindingHash string) *RuntimeError {
	return &RuntimeError{
		Code:        ErrCodeCycleDetected,
		Message:     "sync rule would fire same binding twice in flow",
		FlowToken:   flowToken,
		SyncID:      syncID,
		BindingHash: bindingHash,
	}
}

// NewQuotaError creates a RuntimeError for quota exceeded.
func NewQuotaError(flowToken string, steps, maxSteps int) *RuntimeError {
	return &RuntimeError{
		Code:      ErrCodeQuotaExceeded,
		Message:   fmt.Sprintf("flow exceeded max steps (%d >= %d)", steps, maxSteps),
		FlowToken: flowToken,
		Details: map[string]string{
			"steps":     fmt.Sprintf("%d", steps),
			"max_steps": fmt.Sprintf("%d", maxSteps),
		},
	}
}
