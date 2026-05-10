package core

import "context"

type EnergySource interface {
	// Definiuje podstawowe zachowanie źródła [cite: 33]
	Generate(weather WeatherData) float64
}

type Predictor interface {
	// Analizuje historyczne kroki pogodowe dla prognozy [cite: 34]
	Start(ctx context.Context, weatherChan <-chan WeatherData, forecastChan chan<- ForecastReport)
}

type Consumer interface {
	// Cykl życia niezależnego konsumenta w pętli dla DemandReport [cite: 35, 82]
	Run(ctx context.Context, gridChan chan<- DemandReport)
}

type EnergyStorage interface {
	// Metody dla ESS definiujące manipulację zasobem bateryjnym [cite: 38]
	Charge(amount float64) float64
	Discharge(amount float64) float64
	GetSoC() float64 // Pobór State of Charge 0.0 - 1.0 [cite: 6]
}

type WeatherProvider interface {
	// Inicjuje stację do poboru pogody [cite: 39]
	Start(ctx context.Context, broadcasterChan chan<- WeatherData)
}

type DataLogger interface {
	// Pozwala na zrzut metryk lub wymuszenie Flush przed zamknięciem [cite: 41, 109]
	LogState(stats interface{})
	Flush() error
}
