package config

import "testing"

func TestResolveAppliesDefaultsAndParsesSizes(t *testing.T) {
	fields := map[string]Field{
		"path": {Type: TypeDirectory, Label: "Folder", Required: true},
		"max":  {Type: TypeSize, Label: "Maximum size", Default: "2GB"},
	}
	values, err := Resolve(fields, map[string]any{"path": " ./files "})
	if err != nil {
		t.Fatal(err)
	}
	if values["path"] != "./files" {
		t.Fatalf("path = %#v", values["path"])
	}
	if values["max"] != int64(2_000_000_000) {
		t.Fatalf("max = %#v", values["max"])
	}
}

func TestResolveRejectsUnknownAndMissingValues(t *testing.T) {
	fields := map[string]Field{
		"path": {Type: TypeDirectory, Label: "Folder", Required: true},
	}
	if _, err := Resolve(fields, nil); err == nil {
		t.Fatal("expected missing field error")
	}
	if _, err := Resolve(fields, map[string]any{"path": ".", "other": true}); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestParseSize(t *testing.T) {
	tests := map[string]int64{
		"512B":  512,
		"2KB":   2_000,
		"1.5MB": 1_500_000,
		"2GiB":  2 << 30,
		"3 GB":  3_000_000_000,
	}
	for input, expected := range tests {
		actual, err := ParseSize(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("%s = %d, want %d", input, actual, expected)
		}
	}
}
