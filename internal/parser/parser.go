package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	oasyaml "github.com/oasdiff/yaml"
	yamlv3 "gopkg.in/yaml.v3"
)

type Parser struct{}

type ParsedSchema struct {
	OpenAPIVersion string
	SchemaHash     string
	Endpoints      []Endpoint
}

type Endpoint struct {
	Method             string
	Path               string
	EndpointHash       string
	AuthRequired       bool
	ParametersJSON     json.RawMessage
	RequestSchemaJSON  json.RawMessage
	ResponseSchemaJSON json.RawMessage
}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(ctx context.Context, raw []byte) (ParsedSchema, error) {
	if len(raw) == 0 {
		return ParsedSchema{}, fmt.Errorf("schema file is empty")
	}

	// 1. Check if it's a ZIP archive (Bruno Collection)
	if len(raw) > 4 && raw[0] == 0x50 && raw[1] == 0x4B && raw[2] == 0x03 && raw[3] == 0x04 {
		return p.parseZipArchive(raw)
	}

	// 2. Check if it's a single Bruno file (starts with "meta {")
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "meta {") {
		return p.parseSingleBruno(raw)
	}

	// 3. Fallback to OpenAPI/Swagger
	doc, err := loadDocument(ctx, raw)
	if err != nil {
		return ParsedSchema{}, err
	}
	if err := doc.Validate(ctx); err != nil {
		return ParsedSchema{}, fmt.Errorf("validate openapi schema: %w", err)
	}

	endpoints, err := normalizeEndpoints(doc)
	if err != nil {
		return ParsedSchema{}, err
	}

	return ParsedSchema{
		OpenAPIVersion: doc.OpenAPI,
		SchemaHash:     sha256Hex(raw),
		Endpoints:      endpoints,
	}, nil
}

func (p *Parser) parseSingleBruno(raw []byte) (ParsedSchema, error) {
	blocks, err := parseBruFile(string(raw))
	if err != nil {
		return ParsedSchema{}, err
	}
	ep, err := bruBlocksToEndpoint(blocks)
	if err != nil {
		return ParsedSchema{}, err
	}
	return ParsedSchema{
		OpenAPIVersion: "bruno",
		SchemaHash:     sha256Hex(raw),
		Endpoints:      []Endpoint{ep},
	}, nil
}

func (p *Parser) parseZipArchive(raw []byte) (ParsedSchema, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return ParsedSchema{}, fmt.Errorf("read zip archive: %w", err)
	}

	var endpoints []Endpoint
	for _, f := range reader.File {
		if !strings.HasSuffix(f.Name, ".bru") || f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return ParsedSchema{}, fmt.Errorf("open bru file %s: %w", f.Name, err)
		}
		
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return ParsedSchema{}, fmt.Errorf("read bru file %s: %w", f.Name, err)
		}

		blocks, err := parseBruFile(string(content))
		if err != nil {
			continue // Skip malformed files
		}
		
		ep, err := bruBlocksToEndpoint(blocks)
		if err == nil {
			endpoints = append(endpoints, ep)
		}
	}

	if len(endpoints) == 0 {
		return ParsedSchema{}, fmt.Errorf("zip archive contains no valid .bru files")
	}

	return ParsedSchema{
		OpenAPIVersion: "bruno",
		SchemaHash:     sha256Hex(raw),
		Endpoints:      endpoints,
	}, nil
}

func loadDocument(ctx context.Context, raw []byte) (*openapi3.T, error) {
	var probe struct {
		Swagger string `yaml:"swagger"`
	}
	if yamlErr := yamlv3.Unmarshal(raw, &probe); yamlErr == nil && probe.Swagger == "2.0" {
		return loadSwagger(raw)
	}

	loader := openapi3.NewLoader()
	loader.Context = ctx
	loader.IsExternalRefsAllowed = false

	doc, err := loader.LoadFromData(raw)
	if err == nil {
		return doc, nil
	}

	return nil, fmt.Errorf("parse openapi schema: %w", err)
}

func loadSwagger(raw []byte) (*openapi3.T, error) {
	var swagger openapi2.T
	if _, yamlErr := oasyaml.Unmarshal(raw, &swagger, oasyaml.DecodeOpts{DisableTimestamps: true}); yamlErr != nil {
		return nil, fmt.Errorf("parse swagger schema: %w", yamlErr)
	}

	doc, convErr := openapi2conv.ToV3(&swagger)
	if convErr != nil {
		return nil, fmt.Errorf("convert swagger schema to openapi3: %w", convErr)
	}
	return doc, nil
}

func normalizeEndpoints(doc *openapi3.T) ([]Endpoint, error) {
	if doc.Paths == nil {
		return nil, fmt.Errorf("schema has no paths")
	}

	paths := doc.Paths.InMatchingOrder()
	endpoints := make([]Endpoint, 0, len(paths))

	for _, path := range paths {
		pathItem := doc.Paths.Value(path)
		if pathItem == nil {
			continue
		}
		operations := pathItem.Operations()
		methods := make([]string, 0, len(operations))
		for method := range operations {
			methods = append(methods, method)
		}
		sort.Strings(methods)

		for _, method := range methods {
			operation := operations[method]
			if operation == nil {
				continue
			}

			parameters, err := marshalParameters(pathItem, operation)
			if err != nil {
				return nil, fmt.Errorf("%s %s parameters: %w", method, path, err)
			}
			requestSchema, err := marshalRequestSchema(operation)
			if err != nil {
				return nil, fmt.Errorf("%s %s request schema: %w", method, path, err)
			}
			responseSchema, err := marshalResponseSchema(operation)
			if err != nil {
				return nil, fmt.Errorf("%s %s response schema: %w", method, path, err)
			}

			hash, err := endpointHash(method, path, parameters, requestSchema)
			if err != nil {
				return nil, fmt.Errorf("%s %s endpoint hash: %w", method, path, err)
			}

			endpoints = append(endpoints, Endpoint{
				Method:             method,
				Path:               path,
				EndpointHash:       hash,
				AuthRequired:       authRequired(doc, operation),
				ParametersJSON:     parameters,
				RequestSchemaJSON:  requestSchema,
				ResponseSchemaJSON: responseSchema,
			})
		}
	}

	return endpoints, nil
}

func marshalParameters(pathItem *openapi3.PathItem, operation *openapi3.Operation) (json.RawMessage, error) {
	combined := make([]*openapi3.ParameterRef, 0, len(pathItem.Parameters)+len(operation.Parameters))
	combined = append(combined, pathItem.Parameters...)
	combined = append(combined, operation.Parameters...)
	return marshalOrEmptyArray(combined)
}

func marshalRequestSchema(operation *openapi3.Operation) (json.RawMessage, error) {
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return json.RawMessage(`{}`), nil
	}
	return marshalOrEmptyObject(operation.RequestBody.Value.Content)
}

func marshalResponseSchema(operation *openapi3.Operation) (json.RawMessage, error) {
	if operation.Responses == nil {
		return json.RawMessage(`{}`), nil
	}
	return marshalOrEmptyObject(operation.Responses)
}

func authRequired(doc *openapi3.T, operation *openapi3.Operation) bool {
	if operation.Security != nil {
		return len(*operation.Security) > 0
	}
	return len(doc.Security) > 0
}

func endpointHash(method, path string, parameters, requestSchema json.RawMessage) (string, error) {
	payload := struct {
		Method        string          `json:"method"`
		Path          string          `json:"path"`
		Parameters    json.RawMessage `json:"parameters"`
		RequestSchema json.RawMessage `json:"request_schema"`
	}{
		Method:        strings.ToUpper(method),
		Path:          path,
		Parameters:    parameters,
		RequestSchema: requestSchema,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal endpoint hash payload: %w", err)
	}
	return sha256Hex(b), nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func marshalOrEmptyArray(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return json.RawMessage(`[]`), nil
	}
	return b, nil
}

func marshalOrEmptyObject(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return json.RawMessage(`{}`), nil
	}
	return b, nil
}
