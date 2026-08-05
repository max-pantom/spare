package apischema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedSchemaMatchesCheckedInReference(t *testing.T) {
	generated, err := json.MarshalIndent(Document(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	generated = append(generated, '\n')
	checkedIn, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "api-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(generated) != string(checkedIn) {
		t.Fatal("API schema is stale; run `go run ./cmd/spare-schema`")
	}
}

func TestEndpointCatalogHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, endpoint := range Endpoints() {
		key := endpoint.Method + " " + endpoint.Path
		if seen[key] {
			t.Fatalf("duplicate endpoint %s", key)
		}
		seen[key] = true
	}
}
