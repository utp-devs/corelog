package domain

// EmailMode says when a user receives email notices.
type EmailMode string

// Email modes: immediate sends each notice, daily bundles them into one digest, off sends none.
const (
	EmailImmediate EmailMode = "immediate"
	EmailDaily     EmailMode = "daily"
	EmailOff       EmailMode = "off"
)

// IsValid reports whether m is a known mode.
func (m EmailMode) IsValid() bool { return m == EmailImmediate || m == EmailDaily || m == EmailOff }

// Preferences son los ajustes de avisos de un usuario. Sin fila guardada aplican los valores por defecto.
type Preferences struct {
	// InApp indica si se guardan avisos en la aplicacion.
	InApp bool
	// Email is the delivery mode of the email channel. It only matters when the scheme asks for email.
	Email EmailMode
	// NotifyOwnActions lets a user receive notices about what they did themselves.
	NotifyOwnActions bool
}

func DefaultPreferences() Preferences {
	return Preferences{InApp: true, Email: EmailImmediate}
}
