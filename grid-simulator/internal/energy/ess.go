package energy

import (
	"context"
)

// BatteryOp wyznacza typ operacji żądanej przez Hub na magazynie energii.
type BatteryOp int

const (
	OpCharge    BatteryOp = iota // Ładowanie nadwyżką MW
	OpDischarge                  // Rozładowanie żądaną liczbą MW
	OpGetSoC                     // Zapytanie o State of Charge (0.0 - 1.0)
)

// BatteryRequest to pojedyncze żądanie wysyłane na kanał aktora baterii.
// Reply musi być buforowany (lub jednorazowy), aby aktor nigdy nie blokował się na odpowiedzi.
type BatteryRequest struct {
	Op     BatteryOp
	Amount float64
	Reply  chan float64
}

// Battery implementuje interfejs core.EnergyStorage.
// Stan baterii jest serializowany przez gorutynę Run obsługującą reqChan -
// dostęp pochodzi z dokładnie jednej gorutyny, więc mutex nie jest potrzebny.
type Battery struct {
	capacity      float64 // Maksymalna pojemność w MWh
	currentCharge float64 // Aktualny poziom naładowania w MWh

	reqChan chan BatteryRequest
}

func NewBattery(capacityMWh float64, initialChargeMWh float64) *Battery {
	return &Battery{
		capacity:      capacityMWh,
		currentCharge: initialChargeMWh,
		reqChan:       make(chan BatteryRequest),
	}
}

// RequestChan udostępnia kanał wejściowy aktora baterii.
func (b *Battery) RequestChan() chan<- BatteryRequest { return b.reqChan }

// Run uruchamia gorutynę aktora baterii - serializuje żądania Charge/Discharge/SoC.
func (b *Battery) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-b.reqChan:
			switch req.Op {
			case OpCharge:
				req.Reply <- b.Charge(req.Amount)
			case OpDischarge:
				req.Reply <- b.Discharge(req.Amount)
			case OpGetSoC:
				req.Reply <- b.GetSoC()
			}
		}
	}
}

// Charge ładuje baterię, o ile SoC < 100%. Zwraca ilość faktycznie zmagazynowanej energii.
// Wywoływane wyłącznie z gorutyny Run.
func (b *Battery) Charge(amount float64) float64 {
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
// Wywoływane wyłącznie z gorutyny Run.
func (b *Battery) Discharge(amount float64) float64 {
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
// Wywoływane wyłącznie z gorutyny Run.
func (b *Battery) GetSoC() float64 {
	return b.currentCharge / b.capacity
}
