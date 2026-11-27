package port

import "github.com/google/uuid"

// ConvertInput is the request of a type conversion. ParentID is the parent to set: an unset field keeps the
// current parent and a null field removes it.
type ConvertInput struct {
	IssueTypeID uuid.UUID
	ParentID    Field[uuid.UUID]
}
