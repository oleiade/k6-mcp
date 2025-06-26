// Package docs provides k6 documentation access functionality for MCP servers.
package docs

import "fmt"

// ErrorType represents the type of error that occurred.
type ErrorType string

const (
	// ErrorTypeValidation indicates a validation error.
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeNotFound indicates a resource was not found.
	ErrorTypeNotFound ErrorType = "not_found"
	// ErrorTypeIO indicates an input/output error.
	ErrorTypeIO ErrorType = "io"
	// ErrorTypeInternal indicates an internal error.
	ErrorTypeInternal ErrorType = "internal"
)

// Error represents a documentation-related error with type information.
type Error struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}