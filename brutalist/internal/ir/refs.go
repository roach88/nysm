package ir

// ActionRef is a typed reference to a concept action.
// Format: "Concept.action" (will evolve to nysm://... URI in future).
type ActionRef string

// ConceptRef is a typed reference to a concept.
type ConceptRef struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}
