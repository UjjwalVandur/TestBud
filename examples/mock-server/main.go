// Package main provides a lightweight mock API server that simulates
// the Petstore API for TestBud demo testing. Uses only the Go standard
// library — no external dependencies.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type pet struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Tag  string `json:"tag,omitempty"`
}

var samplePets = []pet{
	{ID: 1, Name: "Buddy", Tag: "dog"},
	{ID: 2, Name: "Whiskers", Tag: "cat"},
	{ID: 3, Name: "Goldie"},
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /pets", handleListPets)
	mux.HandleFunc("POST /pets", handleCreatePet)
	mux.HandleFunc("GET /pets/", handleGetPet)
	mux.HandleFunc("DELETE /pets/", handleDeletePet)
	mux.HandleFunc("PUT /pets/", handleUpdatePet)

	// Health check.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	addr := ":8090"
	log.Printf("Mock Petstore API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// GET /pets — returns all pets. Accepts optional ?limit= query param.
func handleListPets(w http.ResponseWriter, r *http.Request) {
	limit := len(samplePets)
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 100")
			return
		}
		if n < limit {
			limit = n
		}
	}

	writeJSON(w, http.StatusOK, samplePets[:limit])
}

// POST /pets — creates a pet. Requires {"name": "..."} in body.
func handleCreatePet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Tag  string `json:"tag"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	created := pet{
		ID:   len(samplePets) + 1,
		Name: body.Name,
		Tag:  body.Tag,
	}
	writeJSON(w, http.StatusCreated, created)
}

// GET /pets/{id} — returns a single pet by ID.
func handleGetPet(w http.ResponseWriter, r *http.Request) {
	id, err := parsePetID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "petId must be a valid integer")
		return
	}

	for _, p := range samplePets {
		if p.ID == id {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}

	writeError(w, http.StatusNotFound, "pet not found")
}

// DELETE /pets/{id} — deletes a pet. Requires Authorization header.
func handleDeletePet(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeError(w, http.StatusUnauthorized, "authorization header required")
		return
	}

	_, err := parsePetID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "petId must be a valid integer")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PUT /pets/{id} — updates a pet. Requires {"name": "...", "age": N} in body.
func handleUpdatePet(w http.ResponseWriter, r *http.Request) {
	id, err := parsePetID(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "petId must be a valid integer")
		return
	}

	var body struct {
		Name string `json:"name"`
		Age  *int   `json:"age"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	updated := pet{ID: id, Name: body.Name}
	writeJSON(w, http.StatusOK, updated)
}

// --- Helpers ---

func parsePetID(path string) (int, error) {
	// Path is "/pets/123" — extract the last segment.
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) < 2 {
		return 0, fmt.Errorf("no id in path")
	}
	return strconv.Atoi(parts[len(parts)-1])
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
