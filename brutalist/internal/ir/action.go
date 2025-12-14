package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// ValidTypes defines the allowed type strings for action args and output fields.
// NO "float" - floats are forbidden per CP-5 (breaks determinism).
var ValidTypes = map[string]bool{
	"string": true,
	"int":    true,
	"bool":   true,
	"array":  true,
	"object": true,
}

// ValidationError represents a validation error with field path and message.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks ActionSig against schema rules.
// Returns all errors (not fail-fast) for better developer experience.
func (a *ActionSig) Validate() []ValidationError {
	var errs []ValidationError

	// Rule: At least one output case required
	if len(a.Outputs) == 0 {
		errs = append(errs, ValidationError{
			Field:   "outputs",
			Message: "at least one output case is required",
		})
	}

	// Rule: Unique output case names
	seenCases := make(map[string]bool)
	for i, out := range a.Outputs {
		if seenCases[out.Case] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("outputs[%d].case", i),
				Message: fmt.Sprintf("duplicate output case name: %q", out.Case),
			})
		}
		seenCases[out.Case] = true

		// Validate field types within OutputCase
		for fieldName, fieldType := range out.Fields {
			if !ValidTypes[fieldType] {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("outputs[%d].fields.%s", i, fieldName),
					Message: fmt.Sprintf("invalid type %q, must be one of: string, int, bool, array, object", fieldType),
				})
			}
		}
	}

	// Validate args types
	for i, arg := range a.Args {
		if !ValidTypes[arg.Type] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("args[%d].type", i),
				Message: fmt.Sprintf("invalid type %q for arg %q, must be one of: string, int, bool, array, object", arg.Type, arg.Name),
			})
		}
	}

	return errs
}

// MarshalJSON produces JSON with sorted keys for determinism.
// Fields are in alphabetical order: args, name, outputs, requires (if non-empty).
// NOTE: This is NOT canonical marshaling. Use MarshalCanonical (Story 1-4) for hashing.
func (a ActionSig) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	// Fixed field order: args, name, outputs, requires (alphabetical)
	buf.WriteString(`"args":`)
	argsBytes, err := json.Marshal(a.Args)
	if err != nil {
		return nil, err
	}
	buf.Write(argsBytes)

	buf.WriteString(`,"name":`)
	nameBytes, err := json.Marshal(a.Name)
	if err != nil {
		return nil, err
	}
	buf.Write(nameBytes)

	buf.WriteString(`,"outputs":`)
	outputsBytes, err := marshalOutputCases(a.Outputs)
	if err != nil {
		return nil, err
	}
	buf.Write(outputsBytes)

	// Only include requires if non-empty (omitempty behavior)
	if len(a.Requires) > 0 {
		buf.WriteString(`,"requires":`)
		requiresBytes, err := json.Marshal(a.Requires)
		if err != nil {
			return nil, err
		}
		buf.Write(requiresBytes)
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// marshalOutputCases marshals a slice of OutputCase with sorted keys.
func marshalOutputCases(outputs []OutputCase) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')

	for i, out := range outputs {
		if i > 0 {
			buf.WriteByte(',')
		}
		outBytes, err := out.MarshalJSON()
		if err != nil {
			return nil, err
		}
		buf.Write(outBytes)
	}

	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// MarshalJSON produces JSON with sorted field keys for determinism.
// Fields are in order: case, fields (with fields sorted by RFC 8785).
func (o OutputCase) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	buf.WriteString(`"case":`)
	caseBytes, err := json.Marshal(o.Case)
	if err != nil {
		return nil, err
	}
	buf.Write(caseBytes)

	buf.WriteString(`,"fields":{`)

	// Sort field keys using RFC 8785 ordering
	keys := make([]string, 0, len(o.Fields))
	for k := range o.Fields {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareKeysRFC8785)

	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		valBytes, err := json.Marshal(o.Fields[k])
		if err != nil {
			return nil, err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		buf.Write(valBytes)
	}
	buf.WriteString("}}")

	return buf.Bytes(), nil
}

// MarshalJSON produces JSON with sorted keys for NamedArg.
// Fields are in order: name, type (alphabetical).
func (n NamedArg) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	buf.WriteString(`"name":`)
	nameBytes, err := json.Marshal(n.Name)
	if err != nil {
		return nil, err
	}
	buf.Write(nameBytes)

	buf.WriteString(`,"type":`)
	typeBytes, err := json.Marshal(n.Type)
	if err != nil {
		return nil, err
	}
	buf.Write(typeBytes)

	buf.WriteByte('}')
	return buf.Bytes(), nil
}
