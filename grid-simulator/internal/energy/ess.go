package energy

import (
	"sync"
)

// Battery implementuje interfejs core.EnergyStorage.
type Battery struct {
	capacity      float64 // Maksymalna pojemność w MWh
	currentCharge float64 // Aktualny poziom naładowania w MWh
	mu            sync.RWMutex
}

func NewBattery(capacityMWh float64, initialChargeMWh float64) *Battery {
	return &Battery{
		capacity:      capacityMWh,
		currentCharge: initialChargeMWh,
	}
}

// Charge ładuje baterię, o ile SoC < 100%. Zwraca ilość faktycznie zmagazynowanej energii.
func (b *Battery) Charge(amount float64) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	availableSpace := b.capacity - b.currentCharge
	if availableSpace <= 0 {
		return 0 // Bateria pełna, SoC = 100%
	}

	absorbed := amount
	if amount > availableSpace {
		absorbed = availableSpace
	}

	b.currentCharge += absorbed
	return absorbed
}

// Discharge rozładowuje baterię, o ile SoC > 0%. Zwraca ilość faktycznie oddanej energii.
func (b *Battery) Discharge(amount float64) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.currentCharge <= 0 {
		return 0 // Bateria pusta, SoC = 0%
	}

	provided := amount
	if amount > b.currentCharge {
		provided = b.currentCharge
	}

	b.currentCharge -= provided
	return provided
}

// GetSoC zwraca poziom naładowania baterii w przedziale 0.0 - 1.0 (State of Charge).
func (b *Battery) GetSoC() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.currentCharge / b.capacity
}
