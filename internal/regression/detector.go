package regression

import (
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/util"
)

// EndpointChange describes a single endpoint difference between two schema versions.
type EndpointChange struct {
	Method                string `json:"method"`
	Path                  string `json:"path"`
	ParametersChanged     bool   `json:"parameters_changed,omitempty"`
	RequestSchemaChanged  bool   `json:"request_schema_changed,omitempty"`
	ResponseSchemaChanged bool   `json:"response_schema_changed,omitempty"`
	AuthChanged           bool   `json:"auth_changed,omitempty"`
}

// RegressionReport is the result of diffing two schema versions.
type RegressionReport struct {
	BaseSchemaID   uuid.UUID        `json:"base_schema_id"`
	TargetSchemaID uuid.UUID        `json:"target_schema_id"`
	Added          []EndpointChange `json:"added"`
	Removed        []EndpointChange `json:"removed"`
	Modified       []EndpointChange `json:"modified"`
}

// Detect computes the diff between base and target endpoint sets.
// Base is the previous schema version; target is the new one.
func Detect(baseEndpoints, targetEndpoints []models.Endpoint, baseSchemaID, targetSchemaID uuid.UUID) RegressionReport {
	report := RegressionReport{
		BaseSchemaID:   baseSchemaID,
		TargetSchemaID: targetSchemaID,
		Added:          []EndpointChange{},
		Removed:        []EndpointChange{},
		Modified:       []EndpointChange{},
	}

	baseByRoute := indexByRoute(baseEndpoints)
	targetByRoute := indexByRoute(targetEndpoints)

	// Collect all route keys for deterministic ordering.
	allRoutes := make(map[string]bool)
	for k := range baseByRoute {
		allRoutes[k] = true
	}
	for k := range targetByRoute {
		allRoutes[k] = true
	}
	sortedRoutes := make([]string, 0, len(allRoutes))
	for k := range allRoutes {
		sortedRoutes = append(sortedRoutes, k)
	}
	sort.Strings(sortedRoutes)

	for _, route := range sortedRoutes {
		baseEp, inBase := baseByRoute[route]
		targetEp, inTarget := targetByRoute[route]

		switch {
		case inTarget && !inBase:
			report.Added = append(report.Added, EndpointChange{
				Method: targetEp.Method,
				Path:   targetEp.Path,
			})
		case inBase && !inTarget:
			report.Removed = append(report.Removed, EndpointChange{
				Method: baseEp.Method,
				Path:   baseEp.Path,
			})
		default:
			// Both exist — compare fields.
			change := compareEndpoints(baseEp, targetEp)
			if change != nil {
				report.Modified = append(report.Modified, *change)
			}
		}
	}

	return report
}

// compareEndpoints checks if two endpoints with the same route differ in any
// tracked field. Returns nil if they are identical.
func compareEndpoints(base, target models.Endpoint) *EndpointChange {
	paramsChanged := !util.JSONBytesEqual(base.ParametersJSON, target.ParametersJSON)
	reqChanged := !util.JSONBytesEqual(base.RequestSchemaJSON, target.RequestSchemaJSON)
	respChanged := !util.JSONBytesEqual(base.ResponseSchemaJSON, target.ResponseSchemaJSON)
	authChanged := base.AuthRequired != target.AuthRequired

	if !paramsChanged && !reqChanged && !respChanged && !authChanged {
		return nil
	}

	return &EndpointChange{
		Method:                target.Method,
		Path:                  target.Path,
		ParametersChanged:     paramsChanged,
		RequestSchemaChanged:  reqChanged,
		ResponseSchemaChanged: respChanged,
		AuthChanged:           authChanged,
	}
}

// indexByRoute builds a lookup map keyed on "METHOD /path".
func indexByRoute(endpoints []models.Endpoint) map[string]models.Endpoint {
	m := make(map[string]models.Endpoint, len(endpoints))
	for _, ep := range endpoints {
		key := routeKey(ep.Method, ep.Path)
		m[key] = ep
	}
	return m
}

// routeKey returns the canonical "METHOD /path" key for an endpoint.
func routeKey(method, path string) string {
	return fmt.Sprintf("%s %s", method, path)
}


