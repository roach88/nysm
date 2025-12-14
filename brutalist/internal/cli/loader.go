package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/token"

	"github.com/roach88/nysm/internal/compiler"
	"github.com/roach88/nysm/internal/ir"
)

// LoadMode controls how errors are handled during spec loading.
type LoadMode int

const (
	// LoadModeFailFast stops on the first error encountered.
	LoadModeFailFast LoadMode = iota
	// LoadModeCollectAll collects all errors before returning.
	LoadModeCollectAll
)

// LoadResult contains the results of loading specs from a directory.
type LoadResult struct {
	Concepts []ir.ConceptSpec
	Syncs    []ir.SyncRule
	CUEValue cue.Value // The raw CUE value for additional processing
	FileCount int       // Number of CUE files found
}

// LoadError represents an error that occurred during spec loading.
type LoadError struct {
	Code    string
	Message string
	Pos     token.Pos // CUE position if available
}

func (e *LoadError) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.Pos.Filename(), e.Pos.Line(), e.Pos.Column(), e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// LoadSpecs loads and optionally compiles CUE specs from a directory.
// If mode is LoadModeFailFast, returns on first error.
// If mode is LoadModeCollectAll, collects all errors.
func LoadSpecs(dir string, mode LoadMode) (*LoadResult, []error) {
	var errs []error

	// Verify directory exists
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, []error{&LoadError{Code: ErrCodeNotFound, Message: fmt.Sprintf("specs directory not found: %s", dir)}}
	}
	if err != nil {
		return nil, []error{&LoadError{Code: ErrCodeNotFound, Message: fmt.Sprintf("error accessing specs directory: %v", err)}}
	}
	if !info.IsDir() {
		return nil, []error{&LoadError{Code: ErrCodeNotFound, Message: fmt.Sprintf("not a directory: %s", dir)}}
	}

	// Find CUE files
	cueFiles, err := FindCUEFiles(dir)
	if err != nil {
		return nil, []error{&LoadError{Code: ErrCodeScanError, Message: fmt.Sprintf("error scanning directory: %v", err)}}
	}
	if len(cueFiles) == 0 {
		return nil, []error{&LoadError{Code: ErrCodeNoFiles, Message: fmt.Sprintf("no CUE files found in %s", dir)}}
	}

	// Load CUE instances
	ctx := cuecontext.New()
	cfg := &load.Config{Dir: dir}
	instances := load.Instances([]string{"."}, cfg)
	if len(instances) == 0 {
		return nil, []error{&LoadError{Code: ErrCodeLoadFailed, Message: "no CUE instances loaded"}}
	}

	// Check for load errors
	inst := instances[0]
	if inst.Err != nil {
		return nil, []error{&LoadError{Code: ErrCodeLoadFailed, Message: fmt.Sprintf("loading CUE files: %v", inst.Err)}}
	}

	// Build value from instance
	value := ctx.BuildInstance(inst)
	if err := value.Err(); err != nil {
		return nil, []error{&LoadError{Code: ErrCodeBuildFailed, Message: fmt.Sprintf("building CUE value: %v", err)}}
	}

	result := &LoadResult{
		CUEValue:  value,
		FileCount: len(cueFiles),
	}

	// Extract concepts
	conceptsVal := value.LookupPath(cue.ParsePath("concept"))
	if conceptsVal.Exists() {
		iter, iterErr := conceptsVal.Fields()
		if iterErr != nil {
			errs = append(errs, &LoadError{Code: ErrCodeGeneric, Message: fmt.Sprintf("iterating concepts: %v", iterErr)})
			if mode == LoadModeFailFast {
				return result, errs
			}
		} else {
			for iter.Next() {
				spec, compileErr := compiler.CompileConcept(iter.Value())
				if compileErr != nil {
					loadErr := convertCompileError(compileErr, "concept."+iter.Label())
					errs = append(errs, loadErr)
					if mode == LoadModeFailFast {
						return result, errs
					}
					continue
				}
				result.Concepts = append(result.Concepts, *spec)
			}
		}
	}

	// Extract syncs
	syncsVal := value.LookupPath(cue.ParsePath("sync"))
	if syncsVal.Exists() {
		iter, iterErr := syncsVal.Fields()
		if iterErr != nil {
			errs = append(errs, &LoadError{Code: ErrCodeGeneric, Message: fmt.Sprintf("iterating syncs: %v", iterErr)})
			if mode == LoadModeFailFast {
				return result, errs
			}
		} else {
			for iter.Next() {
				rule, compileErr := compiler.CompileSync(iter.Value())
				if compileErr != nil {
					loadErr := convertCompileError(compileErr, "sync."+iter.Label())
					errs = append(errs, loadErr)
					if mode == LoadModeFailFast {
						return result, errs
					}
					continue
				}
				result.Syncs = append(result.Syncs, *rule)
			}
		}
	}

	// Check if we found anything
	if len(result.Concepts) == 0 && len(result.Syncs) == 0 && len(errs) == 0 {
		errs = append(errs, &LoadError{Code: ErrCodeGeneric, Message: "no concepts or syncs found in specs"})
	}

	return result, errs
}

// FindCUEFiles walks the directory and returns all .cue file paths.
func FindCUEFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".cue" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// convertCompileError converts a compiler error to a LoadError with position info.
func convertCompileError(err error, context string) *LoadError {
	var compileErr *compiler.CompileError
	if errors.As(err, &compileErr) {
		return &LoadError{
			Code:    MapFieldToErrorCode(compileErr.Field),
			Message: compileErr.Message,
			Pos:     compileErr.Pos,
		}
	}
	return &LoadError{
		Code:    ErrCodeGeneric,
		Message: fmt.Sprintf("%s: %v", context, err),
	}
}

// Error code constants - unified across all CLI commands.
const (
	ErrCodeGeneric     = "E001" // Generic/unknown error
	ErrCodeScanError   = "E002" // Directory scan error
	ErrCodeNoFiles     = "E003" // No CUE files found
	ErrCodeLoadFailed  = "E004" // CUE load failed
	ErrCodeNotFound    = "E005" // Path not found
	ErrCodeBuildFailed = "E006" // CUE build failed
	ErrCodeWriteFailed = "E007" // File write error

	// Concept validation errors
	ErrCodeConceptPurpose = "E101" // Missing purpose
	ErrCodeConceptActions = "E102" // No actions defined
	ErrCodeActionOutputs  = "E103" // No outputs defined
	ErrCodeInvalidType    = "E104" // Invalid field type (e.g., float)

	// Sync validation errors
	ErrCodeInvalidScope  = "E111" // Invalid scope mode
	ErrCodeInvalidWhen   = "E110" // Invalid when clause
	ErrCodeInvalidWhere  = "E112" // Invalid where clause
	ErrCodeInvalidThen   = "E113" // Invalid then clause
)

// MapFieldToErrorCode maps a compiler error field to an error code.
func MapFieldToErrorCode(field string) string {
	switch field {
	case "purpose":
		return ErrCodeConceptPurpose
	case "action", "action.*":
		return ErrCodeConceptActions
	case "outputs":
		return ErrCodeActionOutputs
	case "type":
		return ErrCodeInvalidType
	case "scope":
		return ErrCodeInvalidScope
	case "when", "when.action", "when.event":
		return ErrCodeInvalidWhen
	case "where", "where.from":
		return ErrCodeInvalidWhere
	case "then", "then.action":
		return ErrCodeInvalidThen
	default:
		return ErrCodeGeneric
	}
}
