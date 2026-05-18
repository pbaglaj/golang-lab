package energy

import (
	"context"
	"time"

	"grid-simulator/internal/core"
)

// GenRequest to żądanie wyliczenia aktualnej produkcji w odpowiedzi na warunki pogodowe.
// Reply musi być buforowany (lub jednorazowy), aby aktor nigdy nie blokował się przy zwracaniu wyniku.
type GenRequest struct {
	Weather core.WeatherData
	Reply   chan float64
}

// StateRequest pozwala zewnętrznemu komponentowi zapytać o stan elektrowni węglowej.
type StateRequest struct {
	Reply chan PlantState
}

// WindFarm implementuje źródło OZE bazujące na wietrze.
// Stan (curtailmentLimit) jest dotykany wyłącznie z gorutyny Run obsługującej
// genChan i curtailChan, więc mutex nie jest potrzebny.
type WindFarm struct {
	baseEfficiency   float64
	curtailmentLimit float64 // Limit mocy przyciętej przez GridHub

	genChan     chan GenRequest
	curtailChan chan float64
}

func NewWindFarm() *WindFarm {
	return &WindFarm{
		baseEfficiency:   2.5, // Mnożnik MW na jednostkę wiatru
		curtailmentLimit: -1,  // -1 oznacza brak limitu
		genChan:          make(chan GenRequest),
		curtailChan:      make(chan float64, 1), // bufor 1 - sygnał typu fire-and-forget
	}
}

// GenChan udostępnia kanał, na który Hub wysyła zapytania o produkcję wiatru.
func (w *WindFarm) GenChan() chan<- GenRequest { return w.genChan }

// CurtailChan udostępnia kanał, którym Hub ogranicza moc OZE (lub zdejmuje limit przy -1).
func (w *WindFarm) CurtailChan() chan<- float64 { return w.curtailChan }

// Run uruchamia gorutynę aktora OZE - obsługuje żądania kanałowe aż do ctx.Done().
func (w *WindFarm) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-w.genChan:
			req.Reply <- w.Generate(req.Weather)
		case limit := <-w.curtailChan:
			w.SetCurtailment(limit)
		}
	}
}

// Generate wywoływane wyłącznie z gorutyny Run.
func (w *WindFarm) Generate(weather core.WeatherData) float64 {
	power := weather.WindSpeed * w.baseEfficiency

	// Obsługa sygnału Curtailment (ograniczenie mocy)
	if w.curtailmentLimit >= 0 && power > w.curtailmentLimit {
		return w.curtailmentLimit
	}
	return power
}

// SetCurtailment wywoływane wyłącznie z gorutyny Run (po odebraniu z curtailChan).
func (w *WindFarm) SetCurtailment(limitMW float64) {
	w.curtailmentLimit = limitMW
}

// Stan elektrowni węglowej
type PlantState int

const (
	StateOff PlantState = iota
	StateWarmingUp
	StateRunning
)

// CoalPlant implementuje konwencjonalne źródło o wysokiej bezwładności.
// Stan zmienia się wyłącznie w gorutynie Run - warmup wykonywany w osobnej
// gorutynie sygnalizuje koniec rozgrzewania przez warmupDoneChan (a nie przez
// współdzieloną zmienną), dzięki czemu mutex nie jest potrzebny.
type CoalPlant struct {
	maxPower float64
	state    PlantState

	genChan         chan GenRequest
	startChan       chan struct{}
	stateChan       chan StateRequest
	warmupDoneChan  chan struct{} // sygnał z gorutyny rozgrzewania
}

func NewCoalPlant(power float64) *CoalPlant {
	return &CoalPlant{
		maxPower:       power,
		state:          StateOff,
		genChan:        make(chan GenRequest),
		startChan:      make(chan struct{}, 1), // bufor 1 - sygnał startu może czekać
		stateChan:      make(chan StateRequest),
		warmupDoneChan: make(chan struct{}, 1),
	}
}

// GenChan udostępnia kanał, na który Hub wysyła zapytania o produkcję mocy.
func (c *CoalPlant) GenChan() chan<- GenRequest { return c.genChan }

// StartChan przyjmuje sygnał rozgrzewania (fire-and-forget).
func (c *CoalPlant) StartChan() chan<- struct{} { return c.startChan }

// StateChan przyjmuje zapytania o aktualny stan elektrowni.
func (c *CoalPlant) StateChan() chan<- StateRequest { return c.stateChan }

// Run uruchamia gorutynę aktora elektrowni węglowej.
// Wszystkie zmiany stanu (genChan, startChan, stateChan, warmupDoneChan)
// odbywają się w tej jednej gorutynie - brak współdzielenia pamięci między wątkami.
func (c *CoalPlant) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-c.genChan:
			req.Reply <- c.generate(req.Weather)
		case <-c.startChan:
			c.beginWarmup(ctx)
		case req := <-c.stateChan:
			req.Reply <- c.state
		case <-c.warmupDoneChan:
			// Sygnał od gorutyny rozgrzewania - przechodzimy do StateRunning,
			// chyba że ktoś już zatrzymał elektrownię.
			if c.state == StateWarmingUp {
				c.state = StateRunning
			}
		}
	}
}

// generate wywoływane wyłącznie z gorutyny Run.
func (c *CoalPlant) generate(_ core.WeatherData) float64 {
	if c.state == StateRunning {
		return c.maxPower
	}
	return 0 // Brak produkcji, jeśli wyłączona lub w trakcie rozruchu
}

// beginWarmup zmienia stan na WarmingUp i odpala asynchroniczny timer.
// Wywoływane wyłącznie z gorutyny Run.
func (c *CoalPlant) beginWarmup(ctx context.Context) {
	if c.state != StateOff {
		return
	}
	c.state = StateWarmingUp

	// Asynchroniczny proces rozgrzewania (np. zajmuje 2 kroki sieciowe).
	// Po upływie czasu gorutyna NIE modyfikuje stanu, tylko wysyła sygnał
	// na warmupDoneChan, który aktualizuje stan w gorutynie Run.
	go func() {
		warmupTime := core.GridStep * 2
		select {
		case <-ctx.Done():
			return
		case <-time.After(warmupTime):
			select {
			case c.warmupDoneChan <- struct{}{}:
			case <-ctx.Done():
			}
		}
	}()
}
