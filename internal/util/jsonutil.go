package util

import (
	"encoding/json"
	"reflect"
)

// JSONBytesEqual compares two JSON byte slices for semantic equality.
func JSONBytesEqual(a, b []byte) bool {
	if string(a) == string(b) {
		return true
	}
	var va, vb interface{}
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}
