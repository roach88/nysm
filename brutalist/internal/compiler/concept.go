package compiler

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"

	"github.com/roach88/nysm/internal/ir"
)

// CompileConcept parses a CUE value into a ConceptSpec.
// Uses CUE SDK's Go API directly (not CLI subprocess).
//
// The CUE value should be the concept struct itself, e.g.:
//
//	ctx := cuecontext.New()
//	v := ctx.CompileString(`concept Cart { ... }`)
//	spec, err := CompileConcept(v.LookupPath(cue.ParsePath("concept.Cart")))
func CompileConcept(v cue.Value) (*ir.ConceptSpec, error) {
	if err := v.Err(); err != nil {
		return nil, formatCUEError(err)
	}

	spec := &ir.ConceptSpec{}

	// Parse concept name from struct label (the path selector)
	labels := v.Path().Selectors()
	if len(labels) > 0 {
		spec.Name = labels[len(labels)-1].String()
	}

	// Parse purpose (required)
	purposeVal := v.LookupPath(cue.ParsePath("purpose"))
	if !purposeVal.Exists() {
		return nil, &CompileError{
			Field:   "purpose",
			Message: "purpose is required",
			Pos:     v.Pos(),
		}
	}
	purpose, err := purposeVal.String()
	if err != nil {
		return nil, formatCUEError(err)
	}
	spec.Purpose = purpose

	// Parse states (optional, can be empty)
	spec.StateSchema, err = parseStates(v)
	if err != nil {
		return nil, err
	}

	// Parse actions (required, at least one)
	spec.Actions, err = parseActions(v)
	if err != nil {
		return nil, err
	}
	if len(spec.Actions) == 0 {
		return nil, &CompileError{
			Field:   "action",
			Message: "at least one action is required",
			Pos:     v.Pos(),
		}
	}

	// Parse operational_principle (optional) - can be a string, object, or array
	opVal := v.LookupPath(cue.ParsePath("operational_principle"))
	if opVal.Exists() {
		principles, err := parseOperationalPrinciples(opVal)
		if err != nil {
			return nil, err
		}
		spec.OperationalPrinciples = principles
	}

	// Also try plural form: operational_principles
	opVals := v.LookupPath(cue.ParsePath("operational_principles"))
	if opVals.Exists() {
		principles, err := parseOperationalPrinciples(opVals)
		if err != nil {
			return nil, err
		}
		spec.OperationalPrinciples = append(spec.OperationalPrinciples, principles...)
	}

	return spec, nil
}

// parseOperationalPrinciples parses operational principles from a CUE value.
// Supports:
// - Single string: "description text"
// - Single object: { description: "...", scenario: "..." }
// - Array of strings or objects
func parseOperationalPrinciples(v cue.Value) ([]ir.OperationalPrinciple, error) {
	var principles []ir.OperationalPrinciple

	// Try as string first (single principle)
	if op, err := v.String(); err == nil {
		return []ir.OperationalPrinciple{{Description: op}}, nil
	}

	// Try as structured object (single principle with description/scenario)
	descVal := v.LookupPath(cue.ParsePath("description"))
	if descVal.Exists() {
		principle, err := parseOperationalPrinciple(v)
		if err != nil {
			return nil, err
		}
		return []ir.OperationalPrinciple{principle}, nil
	}

	// Try as array
	opIter, err := v.List()
	if err != nil {
		return nil, formatCUEError(err)
	}

	for opIter.Next() {
		principle, err := parseOperationalPrinciple(opIter.Value())
		if err != nil {
			return nil, err
		}
		principles = append(principles, principle)
	}

	return principles, nil
}

// parseOperationalPrinciple parses a single operational principle.
// Supports string or structured object format.
func parseOperationalPrinciple(v cue.Value) (ir.OperationalPrinciple, error) {
	var principle ir.OperationalPrinciple

	// Try as string first
	if str, err := v.String(); err == nil {
		principle.Description = str
		return principle, nil
	}

	// Try as structured object
	descVal := v.LookupPath(cue.ParsePath("description"))
	if descVal.Exists() {
		desc, err := descVal.String()
		if err != nil {
			return principle, formatCUEError(err)
		}
		principle.Description = desc

		// Scenario is optional
		scenarioVal := v.LookupPath(cue.ParsePath("scenario"))
		if scenarioVal.Exists() {
			scenario, err := scenarioVal.String()
			if err != nil {
				return principle, formatCUEError(err)
			}
			principle.Scenario = scenario
		}

		return principle, nil
	}

	return principle, &CompileError{
		Field:   "operational_principle",
		Message: "must be a string or object with description field",
		Pos:     v.Pos(),
	}
}

// parseStates extracts state definitions from the concept.
func parseStates(v cue.Value) ([]ir.StateSchema, error) {
	var states []ir.StateSchema

	// Look for state definitions
	stateVal := v.LookupPath(cue.ParsePath("state"))
	if !stateVal.Exists() {
		return states, nil // state is optional
	}

	iter, err := stateVal.Fields()
	if err != nil {
		return nil, formatCUEError(err)
	}

	for iter.Next() {
		stateName := iter.Label()
		stateValue := iter.Value()

		state := ir.StateSchema{
			Name:   stateName,
			Fields: make(map[string]string),
		}

		// Parse fields
		fieldIter, err := stateValue.Fields()
		if err != nil {
			return nil, formatCUEError(err)
		}

		for fieldIter.Next() {
			fieldName := fieldIter.Label()
			fieldType, err := extractTypeName(fieldIter.Value())
			if err != nil {
				return nil, err
			}
			state.Fields[fieldName] = fieldType
		}

		states = append(states, state)
	}

	return states, nil
}

// parseActions extracts action definitions from the concept.
func parseActions(v cue.Value) ([]ir.ActionSig, error) {
	var actions []ir.ActionSig

	// Look for action definitions
	actionVal := v.LookupPath(cue.ParsePath("action"))
	if !actionVal.Exists() {
		return actions, nil
	}

	iter, err := actionVal.Fields()
	if err != nil {
		return nil, formatCUEError(err)
	}

	for iter.Next() {
		actionName := iter.Label()
		actionValue := iter.Value()

		action := ir.ActionSig{
			Name: actionName,
		}

		// Parse args
		argsVal := actionValue.LookupPath(cue.ParsePath("args"))
		if argsVal.Exists() {
			argsIter, err := argsVal.Fields()
			if err != nil {
				return nil, formatCUEError(err)
			}

			for argsIter.Next() {
				argName := argsIter.Label()
				argType, err := extractTypeName(argsIter.Value())
				if err != nil {
					return nil, err
				}
				action.Args = append(action.Args, ir.NamedArg{
					Name: argName,
					Type: argType,
				})
			}
		}

		// Parse requires (optional, for authz hooks)
		requiresVal := actionValue.LookupPath(cue.ParsePath("requires"))
		if requiresVal.Exists() {
			reqIter, err := requiresVal.List()
			if err != nil {
				return nil, formatCUEError(err)
			}
			for reqIter.Next() {
				reqStr, err := reqIter.Value().String()
				if err != nil {
					return nil, formatCUEError(err)
				}
				action.Requires = append(action.Requires, reqStr)
			}
		}

		// Parse outputs (required)
		outputsVal := actionValue.LookupPath(cue.ParsePath("outputs"))
		if !outputsVal.Exists() {
			return nil, &CompileError{
				Field:   fmt.Sprintf("action.%s.outputs", actionName),
				Message: "action outputs are required",
				Pos:     actionValue.Pos(),
			}
		}

		outputIter, err := outputsVal.List()
		if err != nil {
			return nil, formatCUEError(err)
		}

		for outputIter.Next() {
			outVal := outputIter.Value()

			caseName, err := outVal.LookupPath(cue.ParsePath("case")).String()
			if err != nil {
				return nil, formatCUEError(err)
			}

			output := ir.OutputCase{
				Case:   caseName,
				Fields: make(map[string]string),
			}

			fieldsVal := outVal.LookupPath(cue.ParsePath("fields"))
			if fieldsVal.Exists() {
				fieldsIter, err := fieldsVal.Fields()
				if err != nil {
					return nil, formatCUEError(err)
				}

				for fieldsIter.Next() {
					fieldName := fieldsIter.Label()
					fieldType, err := extractTypeName(fieldsIter.Value())
					if err != nil {
						return nil, err
					}
					output.Fields[fieldName] = fieldType
				}
			}

			action.Outputs = append(action.Outputs, output)
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// extractTypeName converts CUE type to IR type string.
// Floats are forbidden per CP-5.
func extractTypeName(v cue.Value) (string, error) {
	switch v.IncompleteKind() {
	case cue.StringKind:
		return "string", nil
	case cue.IntKind:
		return "int", nil
	case cue.BoolKind:
		return "bool", nil
	case cue.ListKind:
		return "array", nil
	case cue.StructKind:
		return "object", nil
	case cue.FloatKind, cue.NumberKind:
		return "", &CompileError{
			Field:   "type",
			Message: "float types are forbidden (CP-5) - use int instead",
			Pos:     v.Pos(),
		}
	default:
		return "", &CompileError{
			Field:   "type",
			Message: fmt.Sprintf("unsupported type kind: %v", v.IncompleteKind()),
			Pos:     v.Pos(),
		}
	}
}

// CompileError represents a compilation error with source position.
type CompileError struct {
	Field   string
	Message string
	Pos     token.Pos
}

func (e *CompileError) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("%s:%d:%d: %s: %s",
			e.Pos.Filename(), e.Pos.Line(), e.Pos.Column(),
			e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// formatCUEError extracts position info from CUE errors.
func formatCUEError(err error) error {
	if err == nil {
		return nil
	}

	// CUE errors may contain multiple errors
	errs := errors.Errors(err)
	if len(errs) == 0 {
		return err
	}

	// Return first error with position info
	firstErr := errs[0]
	positions := errors.Positions(firstErr)
	if len(positions) > 0 {
		return &CompileError{
			Field:   "cue",
			Message: firstErr.Error(),
			Pos:     positions[0],
		}
	}

	return err
}
