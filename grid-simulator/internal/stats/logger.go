package stats

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"sync"
)

// SystemStats to struktura przesyłana z GridHub do logowania aktualnego stanu.
type SystemStats struct {
	TimeStep       int
	WindSpeed      float64
	Sun            float64
	RenewablePower float64
	CoalPower      float64
	Demand         float64
	BatterySoC     float64
	SystemStatus   string
}

// CSVLogger implementuje interfejs core.DataLogger.
type CSVLogger struct {
	statsChan chan SystemStats
	file      *os.File
	writer    *bufio.Writer
	csvWriter *csv.Writer
	wg        *sync.WaitGroup
}

// NewCSVLogger inicjalizuje logger, plik oraz nagłówki CSV.
func NewCSVLogger(filepath string, wg *sync.WaitGroup) (*CSVLogger, error) {
	// Upewniamy się, że folder istnieje
	err := os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(filepath)
	if err != nil {
		return nil, err
	}

	// Optymalizacja operacji dyskowych z bufio.Writer
	writer := bufio.NewWriter(file)
	csvWriter := csv.NewWriter(writer)

	// Zapisujemy nagłówki w pliku CSV
	csvWriter.Write([]string{
		"TimeStep", "WindSpeed", "Sun", "RenewablePower", "CoalPower", "Demand", "BatterySoC", "Status",
	})

	return &CSVLogger{
		statsChan: make(chan SystemStats, 100), // Buforowany kanał do odbierania paczek
		file:      file,
		writer:    writer,
		csvWriter: csvWriter,
		wg:        wg,
	}, nil
}

// Start uruchamia asynchronicznego workera zrzucającego dane na dysk.
func (l *CSVLogger) Start(ctx context.Context) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		// Gwarantuje zrzut bufora do pliku przy kończeniu pracy
		defer l.Flush()

		for {
			select {
			case <-ctx.Done():
				return // Zamknięcie logowania z context.Context

			case stats := <-l.statsChan:
				// Zapisujemy wiersz danych
				l.csvWriter.Write([]string{
					strconv.Itoa(stats.TimeStep),
					fmt.Sprintf("%.2f", stats.WindSpeed),
					fmt.Sprintf("%.2f", stats.Sun),
					fmt.Sprintf("%.2f", stats.RenewablePower),
					fmt.Sprintf("%.2f", stats.CoalPower),
					fmt.Sprintf("%.2f", stats.Demand),
					fmt.Sprintf("%.2f", stats.BatterySoC),
					stats.SystemStatus,
				})
			}
		}
	}()
}

// StatsChan udostępnia kanał wejściowy loggera - alternatywa kanałowa dla LogState.
// Hub używa go do wysyłania statystyk metodą non-blocking (przez select-default).
func (l *CSVLogger) StatsChan() chan<- SystemStats {
	return l.statsChan
}

// LogState realizuje bezblokowe wysyłanie statystyk do wewnętrznego kanału.
func (l *CSVLogger) LogState(stats interface{}) {
	if s, ok := stats.(SystemStats); ok {
		select {
		case l.statsChan <- s:
			// Przekazano z sukcesem do workera zapisu
		default:
			// W przypadku zapchania kanału porzucamy log zamiast blokować mózg systemu
		}
	}
}

// Flush wymusza zapis wszystkich buforów na dysk fizyczny i zamyka plik.
func (l *CSVLogger) Flush() error {
	l.csvWriter.Flush()
	err := l.writer.Flush()
	l.file.Close()
	return err
}
