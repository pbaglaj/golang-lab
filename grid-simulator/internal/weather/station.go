package weather

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"grid-simulator/internal/core"
)

// Broadcaster zarządza subskrybentami i rozgłasza dane pogodowe (Pub/Sub).
type Broadcaster struct {
	subscribers []chan core.WeatherData
	mu          sync.RWMutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make([]chan core.WeatherData, 0),
	}
}

// Subscribe tworzy i rejestruje nowy kanał dla subskrybenta (np. OZE, Predictor).
func (b *Broadcaster) Subscribe() chan core.WeatherData {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Tworzymy kanał z buforem 1, co dodatkowo amortyzuje komunikację
	ch := make(chan core.WeatherData, 1)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Broadcast rozsyła dane do wszystkich zarejestrowanych subskrybentów.
func (b *Broadcaster) Broadcast(data core.WeatherData) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Rozgłaszanie danych z użyciem instrukcji select i default [cite: 51]
	for _, ch := range b.subscribers {
		select {
		case ch <- data:
			// Paczka wysłana pomyślnie [cite: 56]
		default:
			// Subskrybent zajęty - porzucamy paczkę, aby nie blokować sieci [cite: 52, 57, 59]
		}
	}
}

// Station symuluje środowisko zewnętrzne.
type Station struct {
	broadcaster *Broadcaster
	wind        float64 // Aktualna prędkość wiatru
	sun         float64 // Aktualne nasłonecznienie
}

func NewStation(b *Broadcaster) *Station {
	return &Station{
		broadcaster: b,
		wind:        20.0, // Wartość startowa dla wiatru
		sun:         50.0, // Wartość startowa dla słońca
	}
}

// Start uruchamia gorutynę stacji pogodowej na skali WeatherStep[cite: 45].
func (s *Station) Start(ctx context.Context, _ chan<- core.WeatherData) {
	ticker := time.NewTicker(core.WeatherStep)
	defer ticker.Stop()

	// Inicjalizacja generatora losowego
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-ctx.Done():
			return // Zakończenie pracy przy graceful shutdown
		case <-ticker.C:
			// Generowanie płynnych zmian wg wzoru: Vt+1 = Vt + random(-1, 1) [cite: 47, 48]
			windChange := (r.Float64() * 2) - 1 // Losuje z przedziału [-1.0, 1.0)
			sunChange := (r.Float64() * 2) - 1

			s.wind += windChange
			s.sun += sunChange

			// Zabezpieczenie przed nierealnymi wartościami fizycznymi
			if s.wind < 0 {
				s.wind = 0
			}
			if s.sun < 0 {
				s.sun = 0
			} else if s.sun > 100 {
				s.sun = 100
			}

			// Pakujemy dane [cite: 50]
			data := core.WeatherData{
				WindSpeed: s.wind,
				Sun:       s.sun,
			}

			// Wysyłamy do Broadcastera, który przekaże je dalej [cite: 50]
			s.broadcaster.Broadcast(data)
		}
	}
}
