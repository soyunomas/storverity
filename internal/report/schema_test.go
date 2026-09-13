package report

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCommittedSchemaIsValidJSONAndMatchesContract(t *testing.T) {
	payload, err := os.ReadFile("schema-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(payload) {
		t.Fatal("schema-v1.json is not valid JSON")
	}
	var schema struct {
		Schema string `json:"$schema"`
		ID     string `json:"$id"`
		Properties struct {
			SchemaVersion struct {
				Const string `json:"const"`
			} `json:"schemaVersion"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(payload, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("$schema=%q", schema.Schema)
	}
	if schema.ID == "" {
		t.Fatal("$id must not be empty")
	}
	if schema.Properties.SchemaVersion.Const != SchemaVersion {
		t.Fatalf("schema version const=%q want=%q", schema.Properties.SchemaVersion.Const, SchemaVersion)
	}
}
