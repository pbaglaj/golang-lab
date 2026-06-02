package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGetItems sprawdza, że GetItems wysyła GET /items, dodaje nagłówek
// User-Agent oraz poprawnie dekoduje odpowiedź JSON.
func TestGetItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("oczekiwano GET, otrzymano %s", r.Method)
		}
		if r.URL.Path != "/items" {
			t.Errorf("oczekiwano /items, otrzymano %s", r.URL.Path)
		}
		if got := r.Header.Get("User-Agent"); got != defaultUserAgent {
			t.Errorf("oczekiwano User-Agent %q, otrzymano %q", defaultUserAgent, got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"First item","description":"Example description"},
			{"id":2,"name":"Second item","description":"Another example description"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	items, err := client.GetItems(context.Background())
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("oczekiwano 2 elementów, otrzymano %d", len(items))
	}
	want := Item{ID: 1, Name: "First item", Description: "Example description"}
	if items[0] != want {
		t.Errorf("items[0] = %+v, oczekiwano %+v", items[0], want)
	}
}

// TestCreateItem sprawdza, że CreateItem wysyła POST /items, ustawia
// Content-Type, poprawnie koduje ciało żądania i dekoduje odpowiedź 201.
func TestCreateItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("oczekiwano POST, otrzymano %s", r.Method)
		}
		if r.URL.Path != "/items" {
			t.Errorf("oczekiwano /items, otrzymano %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("oczekiwano Content-Type application/json, otrzymano %q", ct)
		}

		// Sprawdzamy, że ciało żądania zostało poprawnie zakodowane do JSON.
		var got CreateItemRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("nie udało się zdekodować ciała żądania: %v", err)
		}
		want := CreateItemRequest{Name: "New item", Description: "Description of the new item"}
		if got != want {
			t.Errorf("ciało żądania = %+v, oczekiwano %+v", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Item{
			ID:          3,
			Name:        got.Name,
			Description: got.Description,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	created, err := client.CreateItem(context.Background(), CreateItemRequest{
		Name:        "New item",
		Description: "Description of the new item",
	})
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}

	want := &Item{ID: 3, Name: "New item", Description: "Description of the new item"}
	if *created != *want {
		t.Errorf("utworzony element = %+v, oczekiwano %+v", *created, *want)
	}
}

// TestGetItemsUnexpectedStatus sprawdza jawną obsługę nieoczekiwanego statusu.
func TestGetItemsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.GetItems(context.Background()); err == nil {
		t.Fatal("oczekiwano błędu dla statusu 500, otrzymano nil")
	}
}

// TestCreateItemUnexpectedStatus sprawdza, że status inny niż 201 daje błąd.
func TestCreateItemUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Zwracamy 200 zamiast oczekiwanego 201 Created.
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(Item{ID: 3, Name: "x"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.CreateItem(context.Background(), CreateItemRequest{Name: "x"})
	if err == nil {
		t.Fatal("oczekiwano błędu dla statusu 200, otrzymano nil")
	}
}

// TestGetItemsInvalidJSON sprawdza obsługę niepoprawnego JSON-a w odpowiedzi.
func TestGetItemsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{ to nie jest poprawny json `))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.GetItems(context.Background()); err == nil {
		t.Fatal("oczekiwano błędu dekodowania JSON, otrzymano nil")
	}
}

// TestUserAgentTransport sprawdza, że własny transport dodaje User-Agent
// i deleguje wykonanie do transportu bazowego.
func TestUserAgentTransport(t *testing.T) {
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	httpClient := &http.Client{
		Transport: &UserAgentTransport{Base: http.DefaultTransport, UserAgent: "custom-agent/1.0"},
	}
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("nie udało się utworzyć żądania: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("nieoczekiwany błąd: %v", err)
	}
	defer resp.Body.Close()

	if gotUA != "custom-agent/1.0" {
		t.Errorf("oczekiwano User-Agent custom-agent/1.0, otrzymano %q", gotUA)
	}
}

// TestContextCancellation sprawdza, że żądania honorują przekazany kontekst —
// anulowanie kontekstu przerywa żądanie i zwraca błąd.
func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serwer celowo zwleka, aby kontekst zdążył wygasnąć.
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	client := NewClient(server.URL)
	if _, err := client.GetItems(ctx); err == nil {
		t.Fatal("oczekiwano błędu z powodu wygaśnięcia kontekstu, otrzymano nil")
	}
}
