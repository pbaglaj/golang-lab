package energy

import (
	"context"
	"sync"
	"time"

	"grid-simulator/internal/core"
)

// WindFarm implementuje źródło OZE bazujące na wietrze.
type WindFarm struct {
	baseEfficiency   float64
	curtailmentLimit float64 // Limit mocy przyciętej przez GridHub
	mu               sync.RWMutex
}

func NewWindFarm() *WindFarm {
	return &WindFarm{
		baseEfficiency:   2.5, // Mnożnik MW na jednostkę wiatru
		curtailmentLimit: -1,  // -1 oznacza brak limitu
	}
}

func (w *WindFarm) Generate(weather core.WeatherData) float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	power := weather.WindSpeed * w.baseEfficiency

	// Obsługa sygnału Curtailment (ograniczenie mocy)
	if w.curtailmentLimit >= 0 && power > w.curtailmentLimit {
		return w.curtailmentLimit
	}
	return power
}

// SetCurtailment pozwala GridHubowi ograniczyć produkcję, gdy jest nadwyżka i bateria jest w 100% pełna.
func (w *WindFarm) SetCurtailment(limitMW float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
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
type CoalPlant struct {
	maxPower float64
	state    PlantState
	mu       sync.RWMutex
}

func NewCoalPlant(power float64) *CoalPlant {
	return &CoalPlant{
		maxPower: power,
		state:    StateOff,
	}
}

func (c *CoalPlant) Generate(weather core.WeatherData) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.state == StateRunning {
		return c.maxPower
	}
	return 0 // Brak produkcji, jeśli wyłączona lub w trakcie rozruchu
}

// Start uruchamia procedurę rozgrzewania elektrowni, o ile była wyłączona.
func (c *CoalPlant) Start(ctx context.Context) {
	c.mu.Lock()
	if c.state != StateOff {
		c.mu.Unlock()
		return
	}
	c.state = StateWarmingUp
	c.mu.Unlock()

	// Asynchroniczny proces rozgrzewania (np. zajmuje 2 kroki sieciowe)
	go func() {
		warmupTime := core.GridStep * 2

		select {
		case <-ctx.Done():
			return
		case <-time.After(warmupTime):
			c.mu.Lock()
			c.state = StateRunning
			c.mu.Unlock()
		}
	}()
}

// Stop wyłącza elektrownię.
func (c *CoalPlant) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = StateOff
}

func (c *CoalPlant) GetState() PlantState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}
