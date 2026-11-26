package domain

// Preferences son los ajustes de avisos de un usuario. Sin fila guardada aplican los valores por defecto.
type Preferences struct {
	// InApp indica si se guardan avisos en la aplicacion. El correo no esta implementado (puerto Notifier).
	InApp bool
}

func DefaultPreferences() Preferences {
	return Preferences{InApp: true}
}
