package core

import "context"

type EnergySource interface {
	// Definiuje podstawowe zachowanie źródła
	Generate(weather WeatherData) float64
}

type Predictor interface {
	// Analizuje historyczne kroki pogodowe dla prognozy
	Start(ctx context.Context, weatherChan <-chan WeatherData, forecastChan chan<- ForecastReport)
}

type Consumer interface {
	// Cykl życia niezależnego konsumenta w pętli dla DemandReport
	Run(ctx context.Context, gridChan chan<- DemandReport)
}

type EnergyStorage interface {
	// Metody dla ESS definiujące manipulację zasobem bateryjnym
	Charge(amount float64) float64
	Discharge(amount float64) float64
	GetSoC() float64 // Pobór State of Charge 0.0 - 1.0
}

type WeatherProvider interface {
	// Inicjuje stację do poboru pogody [cite: 39]
	Start(ctx context.Context, broadcasterChan chan<- WeatherData)
}

type DataLogger interface {
	// Pozwala na zrzut metryk lub wymuszenie Flush przed zamknięciem
	LogState(stats interface{})
	Flush() error
}
