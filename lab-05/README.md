# Lab-05 — Klient HTTP w Go

Mały klient HTTP komunikujący się z fikcyjną usługą JSON API. Klient obsługuje dwa
endpointy:

| Metoda | Endpoint  | Oczekiwany status |
|--------|-----------|-------------------|
| GET    | `/items`  | `200 OK`          |
| POST   | `/items`  | `201 Created`     |

## Założenie zadania

Celem zadania (`zadanie_5.pdf`) jest przećwiczenie poprawnego użycia pakietów
`net/http`, `context`, `encoding/json` oraz `net/http/httptest`. Należało
zaimplementować klienta API, który:

- korzysta z **jednego, wielokrotnie używanego** `http.Client`,
- tworzy **żądania świadome kontekstu** (`context.Context`),
- **koduje** dane do JSON (POST) i **dekoduje** odpowiedzi JSON (GET/POST),
- **jawnie obsługuje statusy HTTP** (czytelny błąd przy nieoczekiwanym statusie),
- używa **własnego transportu** dodającego nagłówek `User-Agent`,
- ma **testy** napisane z użyciem `httptest.Server`.

## Jak działa program

`main.go` uruchamia krótkie demo bez potrzeby prawdziwego API. Startuje lokalny
`httptest.Server`, który symuluje usługę (`GET /items` i `POST /items`), a następnie:

1. wywołuje `GetItems` (z 2-sekundowym limitem czasu z kontekstu) i wypisuje listę,
2. wywołuje `CreateItem`, tworząc nowy element, i wypisuje wynik.

```bash
go run .      # uruchomienie demo
go test ./... # uruchomienie testów
```

## Co, gdzie i jak zostało spełnione

| Wymaganie zadania | Gdzie (plik) | Jak zostało zrealizowane |
|-------------------|--------------|--------------------------|
| Poprawna struktura klienta API | `client.go` — `Client`, `NewClient` | Struktura `Client{ baseURL, httpClient }`; `NewClient` przycina końcowy `/` z adresu bazowego. |
| Wielokrotnie używany `http.Client` | `client.go` — `NewClient` | Jeden `*http.Client` (z `Timeout` 10 s) tworzony raz i współdzielony przez wszystkie metody — nie tworzymy nowego klienta na każde żądanie. |
| Obsługa `GET /items` | `client.go` — `GetItems` | Buduje żądanie GET, ustawia `Accept: application/json`, sprawdza status `200 OK`, dekoduje JSON do `[]Item`. |
| Obsługa `POST /items` | `client.go` — `CreateItem` | Koduje ciało, ustawia `Content-Type: application/json`, wysyła POST, sprawdza status `201 Created`, dekoduje odpowiedź do `*Item`. |
| Kodowanie i dekodowanie JSON | `client.go` — `CreateItem`, `GetItems` | `json.NewEncoder(...).Encode` do zakodowania ciała POST; `json.NewDecoder(...).Decode` do dekodowania odpowiedzi GET i POST. |
| Użycie `context.Context` | `client.go` — `GetItems`, `CreateItem` | Każde żądanie tworzone przez `http.NewRequestWithContext(ctx, ...)`, dzięki czemu honoruje limit czasu / anulowanie. |
| Jawna obsługa statusów HTTP | `client.go` — `unexpectedStatusError` | Po każdym żądaniu jawne porównanie `resp.StatusCode`; przy nieoczekiwanym statusie zwracany czytelny błąd z fragmentem ciała odpowiedzi (ograniczony do 512 bajtów). |
| Własny transport z `User-Agent` | `client.go` — `UserAgentTransport`, `RoundTrip` | Własny `http.RoundTripper` opakowujący `Base` (domyślnie `http.DefaultTransport`); klonuje żądanie i ustawia nagłówek `User-Agent: go-http-clientPV` (stała `defaultUserAgent`), nie modyfikując oryginału. |
| Struktury danych | `client.go` — `Item`, `CreateItemRequest` | Struktury z tagami `json` zgodnymi z treścią zadania. |
| Testy z `httptest.Server` | `client_test.go` | Komplet testów pokrywających wszystkie wymagane przypadki (poniżej). |

## Pokrycie testami (`client_test.go`)

| Test | Co sprawdza |
|------|-------------|
| `TestGetItems` | GET `/items`, obecność nagłówka `User-Agent`, poprawne dekodowanie JSON. |
| `TestCreateItem` | POST `/items`, nagłówek `Content-Type`, poprawne **kodowanie** ciała i dekodowanie odpowiedzi `201`. |
| `TestGetItemsUnexpectedStatus` | Błąd przy statusie `500` zamiast `200`. |
| `TestCreateItemUnexpectedStatus` | Błąd przy statusie `200` zamiast `201`. |
| `TestGetItemsInvalidJSON` | Błąd dekodowania przy niepoprawnym JSON-ie. |
| `TestUserAgentTransport` | Własny transport dodaje `User-Agent` i deleguje do transportu bazowego. |
| `TestContextCancellation` | Żądanie honoruje kontekst — wygaśnięcie limitu czasu przerywa żądanie. |

## Struktura projektu

```
lab-05/
├── client.go        # klient API: Client, GetItems, CreateItem, UserAgentTransport
├── client_test.go   # testy z użyciem httptest.Server
├── main.go          # demo na lokalnym serwerze testowym
├── go.mod           # moduł lab-05 (Go 1.25)
└── zadanie_5.pdf    # treść zadania
```
