package recipes

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spare-run/spare/internal/recipe"
)

func TestBundledManifestsMatchImplementations(t *testing.T) {
	registry, err := Builtins()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"site", "drop", "hook"} {
		fromDisk, err := recipe.Load(filepath.Join("..", "..", "recipes", id))
		if err != nil {
			t.Fatalf("%s manifest: %v", id, err)
		}
		implementation, ok := registry.Get(id)
		if !ok {
			t.Fatalf("%s implementation is missing", id)
		}
		builtIn := implementation.Manifest()
		if !reflect.DeepEqual(fromDisk, builtIn) {
			t.Fatalf("%s manifest does not match implementation", id)
		}
	}
}
