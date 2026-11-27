package palette

import "testing"

func TestIsKey(t *testing.T) {
	for _, key := range []string{"gray", "red", "orange", "amber", "green", "teal", "blue", "indigo", "purple", "pink"} {
		if !IsKey(key) {
			t.Errorf("IsKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"", "Blue", "black", " red", "#ff0000"} {
		if IsKey(key) {
			t.Errorf("IsKey(%q) = true, want false", key)
		}
	}
	if !IsKey(Default) {
		t.Errorf("Default %q is not a palette key", Default)
	}
}

func TestKeysReturnsCopy(t *testing.T) {
	keys := Keys()
	keys[0] = "changed"
	if Keys()[0] != "gray" {
		t.Error("Keys must return a copy")
	}
}
