package core

import "time"

const (
	// Skale czasowe
	WeatherStep = 5 * time.Millisecond // ~5 minut czasu symulacji
	// WeatherStep = GridStep / 12          // ~8.33 ms realnego czasu = 5 minut symulacji
	GridStep = 100 * time.Millisecond // 1 godzina czasu symulacji

	// Parametry buforowania i symulacji
	WeatherPerGrid      = 12             // dokładnie 12 kroków
	ForecastHorizon     = 5              // Prognoza na 5 kroków w przód
	PredictorBufferSize = WeatherPerGrid // Bufor rzędu 1 godziny
)

// Priorytety konsumentów na użytek mechanizmu Load Shedding
const (
	PriorityCritical    = 1 // Krytyczny - odłączany na końcu
	PriorityIndustrial  = 2 // Przemysłowy
	PriorityResidential = 3 // Domowy
)
