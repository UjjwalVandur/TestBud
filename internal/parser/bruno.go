package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	blockStartRegex = regexp.MustCompile(`^([a-zA-Z0-9_:-]+)\s*\{\s*$`)
	kvRegex         = regexp.MustCompile(`^\s*([^:]+):\s*(.*)$`)
)

type bruBlock struct {
	Name       string
	Properties map[string]string
	RawBody    string
}

// parseBruFile parses a .bru file into a list of blocks.
func parseBruFile(content string) ([]bruBlock, error) {
	var blocks []bruBlock
	lines := strings.Split(content, "\n")
	
	var currentBlock *bruBlock
	var blockBody strings.Builder
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if currentBlock == nil {
			if trimmed == "" {
				continue
			}
			matches := blockStartRegex.FindStringSubmatch(trimmed)
			if len(matches) == 2 {
				currentBlock = &bruBlock{
					Name:       matches[1],
					Properties: make(map[string]string),
				}
				blockBody.Reset()
			}
		} else {
			// A block ends when there is a '}' at the very beginning of the line
			if strings.TrimRight(line, "\r") == "}" {
				// Block ends
				if strings.HasPrefix(currentBlock.Name, "body:") {
					currentBlock.RawBody = strings.TrimSpace(blockBody.String())
				}
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
			} else {
				// We are inside a block
				blockBody.WriteString(line + "\n")
				
				// For non-body blocks, parse key-value pairs
				if !strings.HasPrefix(currentBlock.Name, "body:") {
					if kvMatch := kvRegex.FindStringSubmatch(trimmed); len(kvMatch) == 3 {
						key := strings.TrimSpace(kvMatch[1])
						val := strings.TrimSpace(kvMatch[2])
						currentBlock.Properties[key] = val
					}
				}
			}
		}
	}
	
	return blocks, nil
}

// bruBlocksToEndpoint converts a parsed Bruno file into an Endpoint with inferred schemas.
func bruBlocksToEndpoint(blocks []bruBlock) (Endpoint, error) {
	var ep Endpoint
	
	var method string
	var url string
	var rawBody string
	
	// Query params and path params will be collected here to build the ParametersJSON
	paramsMap := make(map[string]map[string]interface{})
	
	for _, b := range blocks {
		switch b.Name {
		case "get", "post", "put", "delete", "patch", "options", "head":
			method = strings.ToUpper(b.Name)
			url = b.Properties["url"]
			
			// Detect auth requirement at the request level
			if auth, ok := b.Properties["auth"]; ok && auth != "none" {
				ep.AuthRequired = true
			}
			
		case "params:query":
			for k, v := range b.Properties {
				paramsMap[k] = map[string]interface{}{
					"in": "query",
					"value": v,
				}
			}
			
		case "params:path":
			for k, v := range b.Properties {
				paramsMap[k] = map[string]interface{}{
					"in": "path",
					"value": v,
				}
			}
			
		case "body:json":
			rawBody = b.RawBody
			
		case "auth:bearer", "auth:basic":
			ep.AuthRequired = true
		}
	}
	
	if method == "" || url == "" {
		return Endpoint{}, fmt.Errorf("missing HTTP method or URL in .bru file")
	}
	
	// Strip query parameters from URL path
	pathOnly := url
	if idx := strings.Index(url, "?"); idx != -1 {
		pathOnly = url[:idx]
	}
	// Extract basic path, e.g. https://api.example.com/users/:id -> /users/{id}
	// Note: Bruno uses :id, OpenAPI uses {id}
	pathOnly = convertBrunoPathToOpenAPI(pathOnly)
	ep.Path = pathOnly
	ep.Method = method
	
	// 1. Synthesize ParametersJSON
	ep.ParametersJSON = synthesizeParameters(paramsMap)
	
	// 2. Synthesize RequestSchemaJSON
	ep.RequestSchemaJSON = synthesizeRequestBody(rawBody)
	
	// 3. Response Schema (empty for Bruno as it doesn't define responses upfront)
	ep.ResponseSchemaJSON = []byte(`{}`)
	
	// 4. Hash
	hash, err := endpointHash(ep.Method, ep.Path, ep.ParametersJSON, ep.RequestSchemaJSON)
	if err != nil {
		return Endpoint{}, err
	}
	ep.EndpointHash = hash
	
	return ep, nil
}

func convertBrunoPathToOpenAPI(u string) string {
	// Strip domain if present
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		parts := strings.SplitN(u, "/", 4)
		if len(parts) == 4 {
			u = "/" + parts[3]
		} else {
			u = "/"
		}
	}
	
	// Convert :id to {id}
	parts := strings.Split(u, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// synthesizeParameters converts a map of concrete parameters into OpenAPI parameter definitions.
func synthesizeParameters(params map[string]map[string]interface{}) []byte {
	if len(params) == 0 {
		return []byte(`[]`)
	}
	
	var out []map[string]interface{}
	for name, info := range params {
		in := info["in"].(string)
		valStr := info["value"].(string)
		
		paramDef := map[string]interface{}{
			"name": name,
			"in":   in,
			"required": in == "path", // path params are always required
			"schema": inferSchemaFromValue(valStr),
		}
		out = append(out, paramDef)
	}
	
	b, _ := json.Marshal(out)
	return b
}

// synthesizeRequestBody converts a raw JSON body into an OpenAPI RequestBody schema.
func synthesizeRequestBody(rawBody string) []byte {
	if rawBody == "" {
		return []byte(`{}`)
	}
	
	var parsed interface{}
	if err := json.Unmarshal([]byte(rawBody), &parsed); err != nil {
		return []byte(`{}`)
	}
	
	schema := inferSchemaFromInterface(parsed)
	
	// Wrap in OpenAPI 3 content structure
	content := map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": schema,
		},
	}
	
	b, _ := json.Marshal(content)
	return b
}

// inferSchemaFromValue guesses the OpenAPI type from a string representation.
func inferSchemaFromValue(val string) map[string]interface{} {
	if val == "true" || val == "false" {
		return map[string]interface{}{"type": "boolean"}
	}
	// Basic integer check
	isNum := true
	for _, c := range val {
		if c < '0' || c > '9' {
			isNum = false
			break
		}
	}
	if isNum && len(val) > 0 {
		return map[string]interface{}{"type": "integer"}
	}
	return map[string]interface{}{"type": "string"}
}

// inferSchemaFromInterface recursively builds an OpenAPI schema from a concrete JSON object.
func inferSchemaFromInterface(val interface{}) map[string]interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		props := make(map[string]interface{})
		for k, child := range v {
			props[k] = inferSchemaFromInterface(child)
		}
		return map[string]interface{}{
			"type":       "object",
			"properties": props,
		}
	case []interface{}:
		var itemSchema map[string]interface{}
		if len(v) > 0 {
			itemSchema = inferSchemaFromInterface(v[0])
		} else {
			itemSchema = map[string]interface{}{"type": "string"}
		}
		return map[string]interface{}{
			"type":  "array",
			"items": itemSchema,
		}
	case bool:
		return map[string]interface{}{"type": "boolean"}
	case float64:
		// JSON unmarshals all numbers as float64
		if v == float64(int64(v)) {
			return map[string]interface{}{"type": "integer"}
		}
		return map[string]interface{}{"type": "number"}
	case string:
		return map[string]interface{}{"type": "string"}
	default:
		return map[string]interface{}{"type": "string"}
	}
}
