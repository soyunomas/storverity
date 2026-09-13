package appmeta

import "testing"

func TestMetadataIsSet(t *testing.T) {
	if Name == "" {
		t.Fatal("Name must not be empty")
	}
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}
