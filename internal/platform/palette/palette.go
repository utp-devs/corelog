// Package palette holds the color keys shared by labels and other colored catalog items. Clients map each key
// to a theme color; the server only stores and validates the key.
package palette

// keys lists the accepted color keys in display order.
var keys = []string{"gray", "red", "orange", "amber", "green", "teal", "blue", "indigo", "purple", "pink"}

// Default is the color used when the caller does not choose one.
const Default = "blue"

// IsKey reports whether key is one of the palette keys.
func IsKey(key string) bool {
	for _, candidate := range keys {
		if candidate == key {
			return true
		}
	}
	return false
}

// Keys returns a copy of the palette keys in display order.
func Keys() []string {
	out := make([]string, len(keys))
	copy(out, keys)
	return out
}
