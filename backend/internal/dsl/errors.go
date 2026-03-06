package dsl

import "fmt"

// DSLError is a position-aware error for user-friendly messages.
type DSLError struct {
	Pos     int    // byte offset in the original input
	Message string // human-readable description
}

func (e *DSLError) Error() string {
	return fmt.Sprintf("at position %d: %s", e.Pos, e.Message)
}
