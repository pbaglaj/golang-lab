package core

import "time"

const (
	// Skale czasowe
	WeatherStep = 5 * time.Millisecond // ~5 minut czasu symulacji [cite: 11, 127]
	// WeatherStep = GridStep / 12          // ~8.33 ms realnego czasu = 5 minut symulacji
	GridStep = 100 * time.Millisecond // 1 godzina czasu symulacji [cite: 17, 127]

	// Parametry buforowania i symulacji
	WeatherPerGrid      = 12             // dokładnie 12 kroków [cite: 65, 128]
	ForecastHorizon     = 5              // Prognoza na 5 kroków w przód [cite: 128]
	PredictorBufferSize = WeatherPerGrid // Bufor rzędu 1 godziny [cite: 128]
)

// Priorytety konsumentów na użytek mechanizmu Load Shedding
const (
	PriorityCritical    = 1 // Krytyczny - odłączany na końcu [cite: 94]
	PriorityIndustrial  = 2 // Przemysłowy [cite: 91]
	PriorityResidential = 3 // Domowy [cite: 89]
)
