package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/roach88/nysm/internal/ir"
)

// Validation error codes (E100-E199)
const (
	// General validation errors (E100)
	ErrUnsupportedIRType = "E100" // unsupported IR type for validation

	// ConceptSpec errors (E101-E109)
	ErrConceptPurposeEmpty = "E101" // purpose is required
	ErrConceptNoActions    = "E102" // at least one action required
	ErrActionNoOutputs     = "E103" // action must have outputs
	ErrInvalidFieldType    = "E104" // invalid type string
	ErrDuplicateName       = "E105" // duplicate action/state name
	ErrFloatTypeForbidden  = "E106" // float types not allowed

	// SyncRule errors (E110-E119)
	ErrInvalidActionRef       = "E110" // invalid action reference format
	ErrInvalidScopeMode       = "E111" // invalid scope mode or missing keyed key
	ErrInvalidWhereClause     = "E112" // invalid where clause
	ErrInvalidThenClause      = "E113" // invalid then clause
	ErrUndefinedBoundVariable = "E114" // bound variable not defined
	ErrMissingSyncClause      = "E115" // missing required clause
	ErrInvalidEventType       = "E116" // invalid event type
)

// ValidationError represents a schema validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Line    int    `json:"line,omitempty"`
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("[%s] line %d: %s: %s", e.Code, e.Line, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Field, e.Message)
}

// Validate validates compiled IR against schema rules.
// Returns all errors found (does not fail-fast).
// Supports ConceptSpec and SyncRule types.
func Validate(v any) []ValidationError {
	switch ir := v.(type) {
	case *ir.ConceptSpec:
		return validateConceptSpec(ir)
	case ir.ConceptSpec:
		return validateConceptSpec(&ir)
	case *ir.SyncRule:
		return validateSyncRule(ir)
	case ir.SyncRule:
		return validateSyncRule(&ir)
	default:
		return []ValidationError{{
			Field:   "type",
			Message: fmt.Sprintf("unsupported IR type: %T", v),
			Code:    ErrUnsupportedIRType,
		}}
	}
}

// validateConceptSpec validates a concept specification.
func validateConceptSpec(spec *ir.ConceptSpec) []ValidationError {
	var errs []ValidationError

	// E101: purpose is required
	if strings.TrimSpace(spec.Purpose) == "" {
		errs = append(errs, ValidationError{
			Field:   "purpose",
			Message: "purpose is required and must be non-empty",
			Code:    ErrConceptPurposeEmpty,
		})
	}

	// E102: at least one action required
	if len(spec.Actions) == 0 {
		errs = append(errs, ValidationError{
			Field:   "actions",
			Message: "at least one action is required",
			Code:    ErrConceptNoActions,
		})
	}

	// Track names for duplicate detection
	actionNames := make(map[string]bool)
	stateNames := make(map[string]bool)

	// Validate actions
	for i, action := range spec.Actions {
		// E105: duplicate action name
		if actionNames[action.Name] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("actions[%d].name", i),
				Message: fmt.Sprintf("duplicate action name: %q", action.Name),
				Code:    ErrDuplicateName,
			})
		}
		actionNames[action.Name] = true

		// E103: action must have outputs
		if len(action.Outputs) == 0 {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("actions[%d].outputs", i),
				Message: fmt.Sprintf("action %q must have at least one output case", action.Name),
				Code:    ErrActionNoOutputs,
			})
		}

		// Validate arg types
		for j, arg := range action.Args {
			typeErrs := validateFieldType(arg.Type, fmt.Sprintf("actions[%d].args[%d].type", i, j), arg.Name)
			errs = append(errs, typeErrs...)
		}

		// Validate output field types
		for j, out := range action.Outputs {
			for fieldName, fieldType := range out.Fields {
				typeErrs := validateFieldType(fieldType, fmt.Sprintf("actions[%d].outputs[%d].fields.%s", i, j, fieldName), fieldName)
				errs = append(errs, typeErrs...)
			}
		}
	}

	// Validate states (StateSchema in our IR)
	for i, state := range spec.StateSchema {
		// E105: duplicate state name
		if stateNames[state.Name] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("state_schema[%d].name", i),
				Message: fmt.Sprintf("duplicate state name: %q", state.Name),
				Code:    ErrDuplicateName,
			})
		}
		stateNames[state.Name] = true

		// Validate field types
		for fieldName, fieldType := range state.Fields {
			typeErrs := validateFieldType(fieldType, fmt.Sprintf("state_schema[%d].fields.%s", i, fieldName), fieldName)
			errs = append(errs, typeErrs...)
		}
	}

	return errs
}

// validateFieldType validates a type string, returning errors for invalid types and floats.
func validateFieldType(fieldType, fieldPath, fieldName string) []ValidationError {
	var errs []ValidationError

	// E104: check for valid type
	if !isValidType(fieldType) {
		errs = append(errs, ValidationError{
			Field:   fieldPath,
			Message: fmt.Sprintf("invalid type %q for field %q", fieldType, fieldName),
			Code:    ErrInvalidFieldType,
		})
	}

	// E106: float forbidden (explicit check even if not in valid types)
	if isFloatType(fieldType) {
		errs = append(errs, ValidationError{
			Field:   fieldPath,
			Message: fmt.Sprintf("float type forbidden for field %q, use int instead", fieldName),
			Code:    ErrFloatTypeForbidden,
		})
	}

	return errs
}

// validateSyncRule validates a sync rule specification.
func validateSyncRule(rule *ir.SyncRule) []ValidationError {
	var errs []ValidationError

	// E111: validate scope mode
	if !isValidScopeMode(rule.Scope.Mode) {
		errs = append(errs, ValidationError{
			Field:   "scope.mode",
			Message: fmt.Sprintf("invalid scope mode %q, must be \"flow\", \"global\", or \"keyed\"", rule.Scope.Mode),
			Code:    ErrInvalidScopeMode,
		})
	}

	// E111: keyed scope must have non-empty key
	if rule.Scope.Mode == "keyed" && strings.TrimSpace(rule.Scope.Key) == "" {
		errs = append(errs, ValidationError{
			Field:   "scope.key",
			Message: "keyed scope requires a non-empty key field",
			Code:    ErrInvalidScopeMode,
		})
	}

	// E110: validate when action reference
	if !isValidActionRef(rule.When.ActionRef) {
		errs = append(errs, ValidationError{
			Field:   "when.action_ref",
			Message: fmt.Sprintf("invalid action reference %q, expected format \"Concept.action\"", rule.When.ActionRef),
			Code:    ErrInvalidActionRef,
		})
	}

	// E116: validate event type
	if rule.When.EventType != "completed" && rule.When.EventType != "invoked" {
		errs = append(errs, ValidationError{
			Field:   "when.event_type",
			Message: fmt.Sprintf("invalid event type %q, must be \"completed\" or \"invoked\"", rule.When.EventType),
			Code:    ErrInvalidEventType,
		})
	}

	// E113: validate then action reference
	if !isValidActionRef(rule.Then.ActionRef) {
		errs = append(errs, ValidationError{
			Field:   "then.action_ref",
			Message: fmt.Sprintf("invalid action reference %q, expected format \"Concept.action\"", rule.Then.ActionRef),
			Code:    ErrInvalidActionRef,
		})
	}

	// E112: validate where clause if present
	if rule.Where != nil {
		if strings.TrimSpace(rule.Where.Source) == "" {
			errs = append(errs, ValidationError{
				Field:   "where.source",
				Message: "where clause requires non-empty \"from\" source",
				Code:    ErrInvalidWhereClause,
			})
		}
	}

	// E114: validate bound variables in then.args are defined
	definedVars := collectBoundVariables(rule)
	for argName, argExpr := range rule.Then.Args {
		usedVars := extractBoundVariableRefs(argExpr)
		for _, usedVar := range usedVars {
			if !definedVars[usedVar] {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("then.args.%s", argName),
					Message: fmt.Sprintf("undefined bound variable %q in expression %q", usedVar, argExpr),
					Code:    ErrUndefinedBoundVariable,
				})
			}
		}
	}

	return errs
}

// isValidType checks if a type string is valid for IR.
func isValidType(t string) bool {
	validTypes := map[string]bool{
		"string": true,
		"int":    true,
		"bool":   true,
		"array":  true,
		"object": true,
	}
	return validTypes[t]
}

// isFloatType checks if a type string represents a float type.
func isFloatType(t string) bool {
	floatTypes := map[string]bool{
		"float":   true,
		"float32": true,
		"float64": true,
		"number":  true,
		"double":  true,
	}
	return floatTypes[t]
}

// isValidScopeMode checks if a scope mode is valid.
func isValidScopeMode(mode string) bool {
	return mode == "flow" || mode == "global" || mode == "keyed"
}

// actionRefPattern matches "Concept.action" format.
// Concept starts with uppercase letter, action starts with lowercase letter.
var actionRefPattern = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*\.[a-z][a-zA-Z0-9]*$`)

// isValidActionRef checks if an action reference has valid format.
func isValidActionRef(ref string) bool {
	return actionRefPattern.MatchString(ref)
}

// collectBoundVariables returns all variable names defined in when and where bindings.
func collectBoundVariables(rule *ir.SyncRule) map[string]bool {
	vars := make(map[string]bool)

	for varName := range rule.When.Bindings {
		vars[varName] = true
	}

	if rule.Where != nil {
		for varName := range rule.Where.Bindings {
			vars[varName] = true
		}
	}

	return vars
}

// boundVarPattern matches "bound.variable_name" references in expressions.
var boundVarPattern = regexp.MustCompile(`bound\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// extractBoundVariableRefs extracts bound variable names from an expression string.
func extractBoundVariableRefs(expr string) []string {
	matches := boundVarPattern.FindAllStringSubmatch(expr, -1)
	vars := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			vars = append(vars, match[1])
		}
	}
	return vars
}
