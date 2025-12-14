package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"cuelang.org/go/cue"

	"github.com/roach88/nysm/internal/ir"
)

// keyedPattern matches keyed("field_name")
var keyedPattern = regexp.MustCompile(`^keyed\("([^"]+)"\)$`)

// CompileSync parses a CUE value into a SyncRule.
// Uses CUE SDK's Go API directly (not CLI subprocess).
//
// The CUE value should be the sync struct itself, e.g.:
//
//	ctx := cuecontext.New()
//	v := ctx.CompileString(`sync "my-sync" { ... }`)
//	rule, err := CompileSync(v.LookupPath(cue.ParsePath(`sync."my-sync"`)))
func CompileSync(v cue.Value) (*ir.SyncRule, error) {
	if err := v.Err(); err != nil {
		return nil, formatCUEError(err)
	}

	rule := &ir.SyncRule{}

	// Parse sync ID from struct label
	// e.g., `sync "cart-inventory" { ... }` → id is "cart-inventory"
	labels := v.Path().Selectors()
	if len(labels) > 0 {
		// The ID may be quoted in CUE, extract it
		rule.ID = strings.Trim(labels[len(labels)-1].String(), `"`)
	}

	// Parse scope (required)
	var err error
	rule.Scope, err = parseScope(v)
	if err != nil {
		return nil, err
	}

	// Parse when clause (required)
	rule.When, err = parseWhenClause(v)
	if err != nil {
		return nil, err
	}

	// Parse where clause (optional)
	whereVal := v.LookupPath(cue.ParsePath("where"))
	if whereVal.Exists() {
		where, err := parseWhereClause(whereVal)
		if err != nil {
			return nil, err
		}
		rule.Where = where
	}

	// Parse then clause (required)
	rule.Then, err = parseThenClause(v)
	if err != nil {
		return nil, err
	}

	return rule, nil
}

// parseScope extracts and validates the scope specification.
func parseScope(v cue.Value) (ir.ScopeSpec, error) {
	scopeVal := v.LookupPath(cue.ParsePath("scope"))
	if !scopeVal.Exists() {
		return ir.ScopeSpec{}, &CompileError{
			Field:   "scope",
			Message: "scope is required",
			Pos:     v.Pos(),
		}
	}

	scopeStr, err := scopeVal.String()
	if err != nil {
		return ir.ScopeSpec{}, formatCUEError(err)
	}

	// Check for keyed("field") pattern
	if matches := keyedPattern.FindStringSubmatch(scopeStr); matches != nil {
		return ir.ScopeSpec{
			Mode: "keyed",
			Key:  matches[1],
		}, nil
	}

	// Check for simple modes: flow, global
	if !ir.ValidScopeModes[scopeStr] {
		return ir.ScopeSpec{}, &CompileError{
			Field:   "scope",
			Message: fmt.Sprintf("invalid scope %q, must be \"flow\", \"global\", or keyed(\"field\")", scopeStr),
			Pos:     scopeVal.Pos(),
		}
	}

	return ir.ScopeSpec{Mode: scopeStr}, nil
}

// parseWhenClause extracts the when clause from a sync rule.
func parseWhenClause(v cue.Value) (ir.WhenClause, error) {
	whenVal := v.LookupPath(cue.ParsePath("when"))
	if !whenVal.Exists() {
		return ir.WhenClause{}, &CompileError{
			Field:   "when",
			Message: "when clause is required",
			Pos:     v.Pos(),
		}
	}

	when := ir.WhenClause{
		Bindings: make(map[string]string),
	}

	// Parse action reference (required string field)
	actionRefVal := whenVal.LookupPath(cue.ParsePath("action"))
	if !actionRefVal.Exists() {
		return when, &CompileError{
			Field:   "when.action",
			Message: "when clause requires 'action' field",
			Pos:     whenVal.Pos(),
		}
	}
	actionRef, err := actionRefVal.String()
	if err != nil {
		return when, formatCUEError(err)
	}
	when.ActionRef = actionRef

	// Parse event type (required string field)
	eventVal := whenVal.LookupPath(cue.ParsePath("event"))
	if !eventVal.Exists() {
		return when, &CompileError{
			Field:   "when.event",
			Message: "when clause requires 'event' field (\"completed\" or \"invoked\")",
			Pos:     whenVal.Pos(),
		}
	}
	event, err := eventVal.String()
	if err != nil {
		return when, formatCUEError(err)
	}
	if event != "completed" && event != "invoked" {
		return when, &CompileError{
			Field:   "when.event",
			Message: fmt.Sprintf("invalid event type %q, must be \"completed\" or \"invoked\"", event),
			Pos:     eventVal.Pos(),
		}
	}
	when.EventType = event

	// Parse output case (optional, for completed events)
	caseVal := whenVal.LookupPath(cue.ParsePath("case"))
	if caseVal.Exists() {
		caseName, err := caseVal.String()
		if err != nil {
			return when, formatCUEError(err)
		}
		when.OutputCase = caseName
	}

	// Parse bindings (string values only)
	bindVal := whenVal.LookupPath(cue.ParsePath("bind"))
	if bindVal.Exists() {
		iter, err := bindVal.Fields()
		if err != nil {
			return when, formatCUEError(err)
		}

		for iter.Next() {
			varName := iter.Label()
			pathExpr, err := iter.Value().String()
			if err != nil {
				return when, &CompileError{
					Field:   fmt.Sprintf("when.bind.%s", varName),
					Message: "binding value must be a string path expression",
					Pos:     iter.Value().Pos(),
				}
			}
			when.Bindings[varName] = pathExpr
		}
	}

	return when, nil
}

// parseWhereClause extracts the where clause from a sync rule.
func parseWhereClause(v cue.Value) (*ir.WhereClause, error) {
	where := &ir.WhereClause{
		Bindings: make(map[string]string),
	}

	// Parse source (from field - required string)
	fromVal := v.LookupPath(cue.ParsePath("from"))
	if !fromVal.Exists() {
		return nil, &CompileError{
			Field:   "where.from",
			Message: "where clause requires 'from' field",
			Pos:     v.Pos(),
		}
	}
	from, err := fromVal.String()
	if err != nil {
		return nil, &CompileError{
			Field:   "where.from",
			Message: "from field must be a string state reference",
			Pos:     fromVal.Pos(),
		}
	}
	where.Source = from

	// Parse filter expression (optional string)
	filterVal := v.LookupPath(cue.ParsePath("filter"))
	if filterVal.Exists() {
		filter, err := filterVal.String()
		if err != nil {
			return nil, &CompileError{
				Field:   "where.filter",
				Message: "filter must be a string expression",
				Pos:     filterVal.Pos(),
			}
		}
		where.Filter = filter
	}

	// Parse bindings (all string values)
	bindVal := v.LookupPath(cue.ParsePath("bind"))
	if bindVal.Exists() {
		iter, err := bindVal.Fields()
		if err != nil {
			return nil, formatCUEError(err)
		}

		for iter.Next() {
			varName := iter.Label()
			pathExpr, err := iter.Value().String()
			if err != nil {
				return nil, &CompileError{
					Field:   fmt.Sprintf("where.bind.%s", varName),
					Message: "binding value must be a string path expression",
					Pos:     iter.Value().Pos(),
				}
			}
			where.Bindings[varName] = pathExpr
		}
	}

	return where, nil
}

// parseThenClause extracts the then clause from a sync rule.
func parseThenClause(v cue.Value) (ir.ThenClause, error) {
	thenVal := v.LookupPath(cue.ParsePath("then"))
	if !thenVal.Exists() {
		return ir.ThenClause{}, &CompileError{
			Field:   "then",
			Message: "then clause is required",
			Pos:     v.Pos(),
		}
	}

	then := ir.ThenClause{
		Args: make(map[string]string),
	}

	// Parse action reference (required string field)
	actionVal := thenVal.LookupPath(cue.ParsePath("action"))
	if !actionVal.Exists() {
		return then, &CompileError{
			Field:   "then.action",
			Message: "then clause requires 'action' field",
			Pos:     thenVal.Pos(),
		}
	}
	action, err := actionVal.String()
	if err != nil {
		return then, &CompileError{
			Field:   "then.action",
			Message: "action must be a string action reference",
			Pos:     actionVal.Pos(),
		}
	}
	then.ActionRef = action

	// Parse args (all string values)
	argsVal := thenVal.LookupPath(cue.ParsePath("args"))
	if argsVal.Exists() {
		iter, err := argsVal.Fields()
		if err != nil {
			return then, formatCUEError(err)
		}

		for iter.Next() {
			argName := iter.Label()
			argExpr, err := iter.Value().String()
			if err != nil {
				return then, &CompileError{
					Field:   fmt.Sprintf("then.args.%s", argName),
					Message: "arg value must be a string expression",
					Pos:     iter.Value().Pos(),
				}
			}
			then.Args[argName] = argExpr
		}
	}

	return then, nil
}
