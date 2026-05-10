package consumer

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"grid-simulator/internal/core"
)

// DemandFunc to funkcja definiująca zapotrzebowanie odbiorcy w zależności od godziny (0-23).
type DemandFunc func(hour int) float64

// Node implementuje interfejs core.Consumer.
type Node struct {
	id         string
	priority   int
	demandFunc DemandFunc
	replyChan  chan core.SupplyStatus
}

// New stworzenie nowego węzła odbiorcy.
func New(id string, priority int, demandFunc DemandFunc) *Node {
	return &Node{
		id:         id,
		priority:   priority,
		demandFunc: demandFunc,
		// Buforowany kanał zwrotny (rozmiar 1) zabezpiecza przed blokowaniem GridHub-a
		replyChan: make(chan core.SupplyStatus, 1),
	}
}

// Run to cykl życia konsumenta. Działa w skali GridStep (co 1h symulacji).
func (c *Node) Run(ctx context.Context, gridChan chan<- core.DemandReport) {
	ticker := time.NewTicker(core.GridStep)
	defer ticker.Stop()

	hourCounter := 0

	for {
		select {
		case <-ctx.Done():
			return // Graceful shutdown
		case <-ticker.C:
			// 1. Ustalenie aktualnego czasu symulacji (godzina w dobie 0-23)
			currentHour := hourCounter % 24
			hourCounter++

			// 2. Oblicz aktualne zapotrzebowanie
			pDemand := c.demandFunc(currentHour)

			// 3. Wyślij Demand Report do wspólnego kanału GridHub (Fan-In)
			report := core.DemandReport{
				ID:        c.id,
				PDemand:   pDemand,
				Priority:  c.priority,
				ReplyChan: c.replyChan, // Przekazujemy adres zwrotny!
			}
			gridChan <- report

			// 4. Czekaj na SupplyStatus (przydział) z kanału zwrotnego
			select {
			case <-ctx.Done():
				return
			case status := <-c.replyChan:
				// 5. Sprawdź, czy wystąpił Load Shedding
				if status.Reason == "LoadShed" {
					fmt.Printf("[Load Shedding] Odbiorca %s (Priorytet %d) dostał %.2f MW z %.2f MW. Powód: %s\n",
						c.id, c.priority, status.AllocatedMW, pDemand, status.Reason)
				}
			}
		}
	}
}

// --- Profile Zapotrzebowania ---

// ResidentialProfile (Odbiorcy domowi) - priorytet 3. Szczyty rano i wieczorem.
func ResidentialProfile() DemandFunc {
	return func(hour int) float64 {
		// Szczyt poranny (~7:00-9:00) i wieczorny (~18:00-22:00)
		if (hour >= 7 && hour <= 9) || (hour >= 18 && hour <= 22) {
			return 4.0 + rand.Float64()*2.0 // Zapotrzebowanie: 4-6 MW
		}
		return 1.0 + rand.Float64() // Poza szczytem: 1-2 MW
	}
}

// IndustrialProfile (Odbiorcy przemysłowi) - priorytet 2. Stabilnie w dzień + piki.
func IndustrialProfile() DemandFunc {
	return func(hour int) float64 {
		// Godziny pracy (~6:00-18:00)
		if hour >= 6 && hour <= 18 {
			baseDemand := 15.0
			// Symulacja rozruchu ciężkich maszyn (10% szans na potężny pik)
			if rand.Float64() < 0.1 {
				baseDemand += 20.0
			}
			return baseDemand
		}
		return 3.0 // Minimum technologiczne po godzinach pracy
	}
}

// CriticalProfile (Odbiorcy krytyczni - szpitale, straż) - priorytet 1. Płasko.
func CriticalProfile() DemandFunc {
	return func(hour int) float64 {
		return 10.0 // Stabilny i niezmienny pobór przez całą dobę
	}
}
