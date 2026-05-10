package grid

import (
	"context"
	"fmt"
	"sort"
	"time"

	"grid-simulator/internal/core"
	"grid-simulator/internal/energy"
	"grid-simulator/internal/stats"
)

// Hub zarządza całą siecią energetyczną.
type Hub struct {
	windFarm    *energy.WindFarm
	coalPlant   *energy.CoalPlant
	battery     *energy.Battery
	logger      core.DataLogger
	demandChan  chan core.DemandReport
	weatherChan chan core.WeatherData

	// Stan wewnętrzny
	consumers map[string]core.DemandReport
	stepCount int
}

func NewHub(wf *energy.WindFarm, cp *energy.CoalPlant, b *energy.Battery, logger core.DataLogger) *Hub {
	return &Hub{
		windFarm:    wf,
		coalPlant:   cp,
		battery:     b,
		logger:      logger,
		demandChan:  make(chan core.DemandReport, 100), // Fan-In agregator
		weatherChan: make(chan core.WeatherData, 1),
		consumers:   make(map[string]core.DemandReport),
	}
}

// GetDemandChan udostępnia kanał, na który konsumenci wysyłają żądania.
func (h *Hub) GetDemandChan() chan<- core.DemandReport {
	return h.demandChan
}

// GetWeatherChan udostępnia kanał dla subskrypcji aktualnej pogody z Broadcastera.
func (h *Hub) GetWeatherChan() chan<- core.WeatherData {
	return h.weatherChan
}

// Start uruchamia główną pętlę Hub-a.
func (h *Hub) Start(ctx context.Context, forecastChan <-chan core.ForecastReport) {
	ticker := time.NewTicker(core.GridStep)
	defer ticker.Stop()

	var currentWeather core.WeatherData

	for {
		select {
		case <-ctx.Done():
			return // Zakończenie pracy (graceful shutdown)

		// Zdarzenie: Aktualizacja aktualnej pogody (do wyliczania produkcji)
		case weather := <-h.weatherChan:
			currentWeather = weather

		// Zdarzenie: Odbiór zgłoszenia od konsumenta (Fan-In) [cite: 96]
		case req := <-h.demandChan:
			// Dynamiczna rejestracja lub aktualizacja [cite: 97]
			h.consumers[req.ID] = req

		// Zdarzenie: Odbiór prognozy z Predictor-a [cite: 75]
		case forecast := <-forecastChan:
			// Jeśli trend silnie ujemny i węglówka wyłączona, zaczynamy rozgrzewanie [cite: 76, 118]
			if forecast.TrendPercentage < -5.0 && h.coalPlant.GetState() == energy.StateOff {
				h.coalPlant.Start(ctx)
			}

		// Zdarzenie: Ticker bilansujący (co 1h symulacji) [cite: 74]
		case <-ticker.C:
			h.stepCount++
			h.balanceGrid(currentWeather)
		}
	}
}

func (h *Hub) balanceGrid(weather core.WeatherData) {
	// 1. Obliczenie całkowitego popytu
	totalDemand := 0.0
	for _, req := range h.consumers {
		totalDemand += req.PDemand
	}

	// 2. Obliczenie aktualnej produkcji
	windPower := h.windFarm.Generate(weather)
	// windPower := 0.0 // TEST: Brak produkcji OZE na start (np. brak wiatru)
	coalPower := h.coalPlant.Generate(weather)
	totalProduction := windPower + coalPower

	balance := totalProduction - totalDemand
	systemStatus := "STABLE"

	// 3. Zarządzanie ESS i Bilansowanie [cite: 78, 99]
	if balance > 0 {
		// Nadwyżka: Ładujemy baterie
		stored := h.battery.Charge(balance)
		remainingSurplus := balance - stored

		if remainingSurplus > 0 {
			// Bateria pełna, mamy wciąż nadwyżkę -> Ograniczamy OZE (Curtailment) [cite: 78, 136]
			h.windFarm.SetCurtailment(totalDemand + stored - coalPower)
		} else {
			h.windFarm.SetCurtailment(-1) // Zdejmujemy ewentualne limity
		}

		// Obsługa konsumentów (wszyscy dostają 100%)
		for _, req := range h.consumers {
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: req.PDemand, Reason: "OK"}
		}

	} else if balance < 0 {
		// Niedobór: Odblokowujemy OZE i próbujemy użyć baterii
		h.windFarm.SetCurtailment(-1)
		deficit := -balance
		provided := h.battery.Discharge(deficit)
		remainingDeficit := deficit - provided

		if remainingDeficit > 0 {
			// Bateria pusta, wciąż brakuje prądu -> Load Shedding [cite: 79, 98]
			systemStatus = "CRITICAL"
			h.executeLoadShedding(totalProduction + provided)
		} else {
			// Bateria pokryła deficyt (wszyscy dostają 100%)
			for _, req := range h.consumers {
				req.ReplyChan <- core.SupplyStatus{AllocatedMW: req.PDemand, Reason: "OK"}
			}
		}
	} else {
		// Idealny balans
		for _, req := range h.consumers {
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: req.PDemand, Reason: "OK"}
		}
	}

	// 4. Logowanie i Raportowanie
	statsData := stats.SystemStats{
		TimeStep:       h.stepCount,
		WindSpeed:      weather.WindSpeed,
		Sun:            weather.Sun,
		RenewablePower: windPower,
		CoalPower:      coalPower,
		Demand:         totalDemand,
		BatterySoC:     h.battery.GetSoC() * 100, // % SoC
		SystemStatus:   systemStatus,
	}

	h.logger.LogState(statsData) // [cite: 106, 107]

	// Wyświetlanie raportu w konsoli co N kroków (np. co 12 godzin symulacji)
	if h.stepCount%12 == 0 {
		fmt.Printf("\n[Raport Krok %d]\n", h.stepCount)
		fmt.Printf("[Pogoda] Wiatr: %.1f km/h | Słońce: %.1f%%\n", weather.WindSpeed, weather.Sun)
		fmt.Printf("[Produkcja] OZE: %.1f MW | Konwencjonalna: %.1f MW | Baterie: %.1f%% (SoC)\n", windPower, coalPower, h.battery.GetSoC()*100)
		actualBalance := totalProduction - totalDemand
		fmt.Printf("[Sieć] Popyt: %.1f MW | Bilans: %.1f MW | Stan: [%s]\n", totalDemand, actualBalance, systemStatus)
	}
}

func (h *Hub) executeLoadShedding(availablePower float64) {
	// Przygotowanie listy konsumentów do posortowania
	var consumers []core.DemandReport
	for _, req := range h.consumers {
		consumers = append(consumers, req)
	}

	// Sortowanie po priorytecie: od najniższego (Residential=3) do najwyższego (Critical=1)
	sort.Slice(consumers, func(i, j int) bool {
		return consumers[i].Priority < consumers[j].Priority // < oznacza, że wartość 1 jest pierwsza
	})

	remainingPower := availablePower

	for _, req := range consumers {
		if remainingPower >= req.PDemand {
			// Wystarczy energii dla tego odbiorcy
			remainingPower -= req.PDemand
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: req.PDemand, Reason: "OK"}
		} else {
			// Brak prądu (lub częściowy brak), odłączamy konsumenta [cite: 101]
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: 0, Reason: "LoadShed"} // [cite:102]
		}
	}
}
