package parser

import (
	"encoding/json"
	"testing"
)

func TestParseBruFile(t *testing.T) {
	bruContent := `meta {
  name: Create User
  type: http
  seq: 1
}

post {
  url: https://api.example.com/users/:id?active=true
  body: json
  auth: bearer
}

params:query {
  active: true
  role: admin
}

params:path {
  id: 123
}

body:json {
  {
    "username": "john",
    "age": 30,
    "is_active": true
  }
}
`

	blocks, err := parseBruFile(bruContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(blocks) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(blocks))
	}

	ep, err := bruBlocksToEndpoint(blocks)
	if err != nil {
		t.Fatalf("failed to convert to endpoint: %v", err)
	}

	if ep.Method != "POST" {
		t.Errorf("expected POST, got %s", ep.Method)
	}
	if ep.Path != "/users/{id}" {
		t.Errorf("expected /users/{id}, got %s", ep.Path)
	}
	if ep.AuthRequired != true {
		t.Errorf("expected AuthRequired to be true")
	}

	// Verify synthesized request schema
	var reqSchema map[string]interface{}
	if err := json.Unmarshal(ep.RequestSchemaJSON, &reqSchema); err != nil {
		t.Fatalf("failed to parse synthesized request schema: %v", err)
	}

	content := reqSchema["application/json"].(map[string]interface{})
	schema := content["schema"].(map[string]interface{})
	props := schema["properties"].(map[string]interface{})

	if props["username"].(map[string]interface{})["type"] != "string" {
		t.Errorf("username type should be string")
	}
	if props["age"].(map[string]interface{})["type"] != "integer" {
		t.Errorf("age type should be integer")
	}
	if props["is_active"].(map[string]interface{})["type"] != "boolean" {
		t.Errorf("is_active type should be boolean")
	}

	// Verify synthesized parameters
	var params []map[string]interface{}
	if err := json.Unmarshal(ep.ParametersJSON, &params); err != nil {
		t.Fatalf("failed to parse synthesized params: %v", err)
	}

	if len(params) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(params))
	}

	foundID := false
	for _, p := range params {
		if p["name"] == "id" {
			foundID = true
			if p["in"] != "path" {
				t.Errorf("id should be in path")
			}
			if p["required"] != true {
				t.Errorf("id should be required")
			}
			schema := p["schema"].(map[string]interface{})
			if schema["type"] != "integer" {
				t.Errorf("id type should be integer")
			}
		}
	}
	if !foundID {
		t.Errorf("missing path parameter 'id'")
	}
}
