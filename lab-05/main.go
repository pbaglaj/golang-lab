package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
)

// main uruchamia krótką demonstrację klienta na lokalnym serwerze testowym.
//
// Serwer naśladuje fikcyjną usługę z zadania (GET /items oraz POST /items),
// dzięki czemu program można uruchomić bez dostępu do prawdziwego API:
//
//	go run .
func main() {
	server := httptest.NewServer(http.HandlerFunc(demoHandler))
	defer server.Close()

	client := NewClient(server.URL)

	// GET /items — pobranie listy elementów z 2-sekundowym limitem czasu.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	items, err := client.GetItems(ctx)
	if err != nil {
		log.Fatalf("GetItems: %v", err)
	}
	fmt.Println("GET /items zwróciło:")
	for _, it := range items {
		fmt.Printf("  #%d %s — %s\n", it.ID, it.Name, it.Description)
	}

	// POST /items — utworzenie nowego elementu.
	created, err := client.CreateItem(ctx, CreateItemRequest{
		Name:        "New item",
		Description: "Description of the new item",
	})
	if err != nil {
		log.Fatalf("CreateItem: %v", err)
	}
	fmt.Printf("POST /items utworzyło: #%d %s — %s\n",
		created.ID, created.Name, created.Description)
}

// demoHandler obsługuje endpointy fikcyjnej usługi na potrzeby demonstracji.
func demoHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/items" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]Item{
			{ID: 1, Name: "First item", Description: "Example description"},
			{ID: 2, Name: "Second item", Description: "Another example description"},
		})

	case http.MethodPost:
		var req CreateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Item{
			ID:          3,
			Name:        req.Name,
			Description: req.Description,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
