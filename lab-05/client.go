// Package main implementuje małego klienta HTTP dla fikcyjnej usługi JSON API.
//
// Klient obsługuje dwa endpointy:
//
//	GET  /items  – pobranie listy elementów (oczekiwany status 200 OK)
//	POST /items  – utworzenie nowego elementu (oczekiwany status 201 Created)
//
// Realizuje wymagania zadania 5: wielokrotnie używany http.Client, żądania
// świadome kontekstu, kodowanie/dekodowanie JSON, jawną obsługę statusów HTTP
// oraz własny transport dodający nagłówek User-Agent.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultUserAgent to wartość nagłówka User-Agent dodawana do każdego żądania.
const defaultUserAgent = "go-http-clientPV"

// Item reprezentuje pojedynczy element zwracany przez usługę.
type Item struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateItemRequest to ciało żądania POST /items.
type CreateItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UserAgentTransport to własny http.RoundTripper, który dodaje nagłówek
// User-Agent do każdego żądania, a następnie deleguje wykonanie do Base.
// Jeśli Base jest nil, używany jest http.DefaultTransport.
type UserAgentTransport struct {
	Base      http.RoundTripper
	UserAgent string
}

// RoundTrip implementuje interfejs http.RoundTripper.
//
// Zgodnie z kontraktem RoundTripper nie wolno modyfikować przekazanego żądania,
// dlatego tworzymy płytką kopię i ustawiamy nagłówek na kopii.
func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	ua := t.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}

	// Klonujemy żądanie razem z jego kontekstem, aby nie modyfikować oryginału.
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", ua)

	return base.RoundTrip(clone)
}

// Client to klient fikcyjnej usługi JSON API. Korzysta z jednego, wielokrotnie
// używanego http.Client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient tworzy klienta dla podanego adresu bazowego usługi.
//
// Klient używa pojedynczego http.Client z własnym transportem dodającym
// nagłówek User-Agent oraz rozsądnym limitem czasu.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &UserAgentTransport{
				Base:      http.DefaultTransport,
				UserAgent: defaultUserAgent,
			},
		},
	}
}

// GetItems wysyła żądanie GET /items i zwraca listę elementów.
//
// Metoda używa przekazanego kontekstu, sprawdza czy status to 200 OK i dekoduje
// odpowiedź JSON. W razie błędnego statusu lub niepoprawnego JSON-a zwraca błąd.
func (c *Client) GetItems(ctx context.Context) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/items", nil)
	if err != nil {
		return nil, fmt.Errorf("GetItems: tworzenie żądania: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetItems: wykonanie żądania: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, unexpectedStatusError("GET /items", http.StatusOK, resp)
	}

	var items []Item
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("GetItems: dekodowanie odpowiedzi JSON: %w", err)
	}

	return items, nil
}

// CreateItem wysyła żądanie POST /items z zakodowanym ciałem JSON i zwraca
// utworzony element.
//
// Metoda ustawia nagłówek Content-Type: application/json, używa przekazanego
// kontekstu, sprawdza czy status to 201 Created i dekoduje odpowiedź JSON.
func (c *Client) CreateItem(ctx context.Context, input CreateItemRequest) (*Item, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(input); err != nil {
		return nil, fmt.Errorf("CreateItem: kodowanie ciała żądania: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/items", &body)
	if err != nil {
		return nil, fmt.Errorf("CreateItem: tworzenie żądania: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CreateItem: wykonanie żądania: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, unexpectedStatusError("POST /items", http.StatusCreated, resp)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("CreateItem: dekodowanie odpowiedzi JSON: %w", err)
	}

	return &item, nil
}

// unexpectedStatusError buduje czytelny błąd dla nieoczekiwanego statusu HTTP,
// dołączając fragment ciała odpowiedzi (o ile istnieje), co ułatwia diagnozę.
func unexpectedStatusError(op string, want int, resp *http.Response) error {
	// Ograniczamy ilość czytanego ciała, aby nie wczytywać dużych odpowiedzi.
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if len(snippet) > 0 {
		return fmt.Errorf("%s: nieoczekiwany status %d (oczekiwano %d): %s",
			op, resp.StatusCode, want, strings.TrimSpace(string(snippet)))
	}
	return fmt.Errorf("%s: nieoczekiwany status %d (oczekiwano %d)", op, resp.StatusCode, want)
}
