package issue

import (
	"github.com/jhonsferg/corelog/internal/issue/application/port"
)

// WithOperations wires the bulk, clone, move, convert and export operations. repo implements their out-ports on
// one database; attachments and files copy the attachments of a cloned issue.
func (m *Module) WithOperations(repo port.OperationsRepository, attachments port.AttachmentRepository, files port.FileStorage) *Module {
	m.service.WithOperations(repo, attachments, files)
	m.Handler.WithOperations(m.service)
	return m
}
