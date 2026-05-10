package grid

import (
	"context"

	"grid-simulator/internal/core"
)

// GridPredictor implementuje interfejs Predictor zdefiniowany w core.
type GridPredictor struct {
	bufferSize int
	buffer     []core.WeatherData
}

func NewGridPredictor() *GridPredictor {
	return &GridPredictor{
		bufferSize: core.PredictorBufferSize,
		buffer:     make([]core.WeatherData, 0, core.PredictorBufferSize),
	}
}

// Start uruchamia gorutynę Predictora.
func (p *GridPredictor) Start(ctx context.Context, weatherChan <-chan core.WeatherData, forecastChan chan<- core.ForecastReport) {
	// Licznik próbek do synchronizacji skali czasowej
	samplesCollected := 0

	for {
		select {
		case <-ctx.Done():
			return // Zakończenie pracy przy graceful shutdown

		case weatherData, ok := <-weatherChan:
			if !ok {
				return
			}

			// 1. Zarządzanie buforem - dodajemy najnowszy odczyt
			p.buffer = append(p.buffer, weatherData)

			// 2. Jeśli przekraczamy rozmiar (np. N=12), usuwamy najstarszy element [cite: 64, 65]
			if len(p.buffer) > p.bufferSize {
				p.buffer = p.buffer[1:]
			}

			samplesCollected++

			// 3. Po zebraniu 12 próbek mija dokładnie 1 GridStep (1 godzina symulacji)
			if samplesCollected >= core.WeatherPerGrid && len(p.buffer) >= 2 {
				samplesCollected = 0 // Resetujemy licznik do kolejnej godziny

				// 4. Ekstrapolacja trendu na podstawie zebranych punktów pomiarowych [cite: 67, 69]
				oldest := p.buffer[0]
				newest := p.buffer[len(p.buffer)-1]

				// Wyliczamy prostą pochodną / zmianę procentową dla wiatru
				// (Można to rozbudować o analizę słońca lub średnią kroczącą)
				var trendPercentage float64
				if oldest.WindSpeed > 0 {
					trendPercentage = ((newest.WindSpeed - oldest.WindSpeed) / oldest.WindSpeed) * 100.0
				}

				// 5. Budujemy raport prognozy na X kroków w przód (ForecastHorizon) [cite: 25]
				report := core.ForecastReport{
					TrendPercentage: trendPercentage,
					StepsAhead:      core.ForecastHorizon,
				}

				// 6. Wysyłamy prognozę dedykowanym, oddzielonym kanałem [cite: 70]
				// Używamy select + default, żeby wolny GridHub nie zablokował Predictora
				select {
				case forecastChan <- report:
					// Prognoza wysłana pomyślnie
				default:
					// GridHub jest zajęty, odrzucamy tę prognozę
				}
			}
		}
	}
}
