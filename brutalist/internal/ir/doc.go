// Package ir provides canonical intermediate representation types for NYSM.
//
// This package contains type definitions only. All other internal packages
// import ir; ir imports nothing internal. This ensures IR remains the
// foundational layer with no circular dependencies.
//
// Key design constraints:
//   - NO float types anywhere (CP-5) - use int64 for numbers
//   - SecurityContext always non-pointer on Invocation and Completion (CP-6)
//   - All JSON tags use snake_case
//   - Logical clocks (seq) only, never wall-clock timestamps (CP-2)
package ir
