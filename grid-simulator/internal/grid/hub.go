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

// Hub zarządza całą siecią energetyczną wyłącznie poprzez kanały.
type Hub struct {
	// Kanały do OZE (WindFarm)
	windGenChan     chan<- energy.GenRequest
	windCurtailChan chan<- float64

	// Kanały do elektrowni węglowej (CoalPlant)
	coalGenChan   chan<- energy.GenRequest
	coalStartChan chan<- struct{}
	coalStateChan chan<- energy.StateRequest

	// Kanał do magazynu energii (Battery)
	batteryChan chan<- energy.BatteryRequest

	// Kanał do loggera (fan-in statystyk)
	loggerChan chan<- stats.SystemStats

	// Kanały od konsumentów (Fan-In) i z broadcastera pogody
	demandChan  chan core.DemandReport
	weatherChan chan core.WeatherData

	// Stan wewnętrzny
	consumers map[string]core.DemandReport
	stepCount int
}

// NewHub zbiera kanały aktorów i tworzy Hub. Od tego momentu Hub komunikuje się
// z OZE/węglówką/baterią/loggerem wyłącznie przez kanały, nie wywołując ich metod bezpośrednio.
func NewHub(wf *energy.WindFarm, cp *energy.CoalPlant, b *energy.Battery, logger *stats.CSVLogger) *Hub {
	return &Hub{
		windGenChan:     wf.GenChan(),
		windCurtailChan: wf.CurtailChan(),

		coalGenChan:   cp.GenChan(),
		coalStartChan: cp.StartChan(),
		coalStateChan: cp.StateChan(),

		batteryChan: b.RequestChan(),
		loggerChan:  logger.StatsChan(),

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

		// Zdarzenie: Odbiór zgłoszenia od konsumenta (Fan-In)
		case req := <-h.demandChan:
			// Dynamiczna rejestracja lub aktualizacja
			h.consumers[req.ID] = req

		// Zdarzenie: Odbiór prognozy z Predictor-a
		case forecast := <-forecastChan:
			// Jeśli trend silnie ujemny i węglówka wyłączona, zaczynamy rozgrzewanie
			state, ok := h.askCoalState(ctx)
			if !ok {
				return
			}
			if forecast.TrendPercentage < -5.0 && state == energy.StateOff {
				// Sygnał startu - fire-and-forget, bufor=1, więc się nie zablokujemy
				select {
				case h.coalStartChan <- struct{}{}:
				default:
				}
			}

		// Zdarzenie: Ticker bilansujący (co 1h symulacji)
		case <-ticker.C:
			h.stepCount++
			h.balanceGrid(ctx, currentWeather)
		}
	}
}

// askCoalState wysyła synchroniczne zapytanie kanałowe o stan elektrowni węglowej.
// Zwraca (stan, ok); ok=false oznacza, że ctx został anulowany — wywołujący powinien zakończyć pracę.
func (h *Hub) askCoalState(ctx context.Context) (energy.PlantState, bool) {
	reply := make(chan energy.PlantState, 1)
	select {
	case h.coalStateChan <- energy.StateRequest{Reply: reply}:
	case <-ctx.Done():
		return energy.StateOff, false
	}
	select {
	case s := <-reply:
		return s, true
	case <-ctx.Done():
		return energy.StateOff, false
	}
}

// askGeneration odpytuje aktora źródła energii (OZE lub węglówki) o aktualną produkcję.
func (h *Hub) askGeneration(ctx context.Context, genChan chan<- energy.GenRequest, weather core.WeatherData) (float64, bool) {
	reply := make(chan float64, 1)
	select {
	case genChan <- energy.GenRequest{Weather: weather, Reply: reply}:
	case <-ctx.Done():
		return 0, false
	}
	select {
	case v := <-reply:
		return v, true
	case <-ctx.Done():
		return 0, false
	}
}

// batteryOp wysyła pojedynczą operację do aktora baterii i zwraca wynik.
func (h *Hub) batteryOp(ctx context.Context, op energy.BatteryOp, amount float64) (float64, bool) {
	reply := make(chan float64, 1)
	select {
	case h.batteryChan <- energy.BatteryRequest{Op: op, Amount: amount, Reply: reply}:
	case <-ctx.Done():
		return 0, false
	}
	select {
	case v := <-reply:
		return v, true
	case <-ctx.Done():
		return 0, false
	}
}

// sendCurtail wysyła sygnał curtailment do WindFarm nieblokująco i bezpiecznie wobec shutdownu.
func (h *Hub) sendCurtail(ctx context.Context, limit float64) {
	select {
	case h.windCurtailChan <- limit:
	case <-ctx.Done():
	default:
		// Bufor zajęty — porzucamy sygnał; kolejny tick i tak ustawi limit.
	}
}

func (h *Hub) balanceGrid(ctx context.Context, weather core.WeatherData) {
	// 1. Obliczenie całkowitego popytu
	totalDemand := 0.0
	for _, req := range h.consumers {
		totalDemand += req.PDemand
	}

	// 2. Obliczenie aktualnej produkcji - przez kanały aktorów
	windPower, ok := h.askGeneration(ctx, h.windGenChan, weather)
	if !ok {
		return
	}
	coalPower, ok := h.askGeneration(ctx, h.coalGenChan, weather)
	if !ok {
		return
	}
	totalProduction := windPower + coalPower

	balance := totalProduction - totalDemand
	systemStatus := "STABLE"

	// 3. Zarządzanie ESS i Bilansowanie
	if balance > 0 {
		// Nadwyżka: Ładujemy baterie przez kanał baterii
		stored, ok := h.batteryOp(ctx, energy.OpCharge, balance)
		if !ok {
			return
		}
		remainingSurplus := balance - stored

		if remainingSurplus > 0 {
			// Bateria pełna, mamy wciąż nadwyżkę -> Ograniczamy OZE (Curtailment)
			// Limit = demand - coal (bateria pełna, nie może wchłonąć więcej)
			h.sendCurtail(ctx, totalDemand-coalPower)
		} else {
			h.sendCurtail(ctx, -1) // Zdejmujemy ewentualne limity
		}

		// Obsługa konsumentów (wszyscy dostają 100%)
		for _, req := range h.consumers {
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: req.PDemand, Reason: "OK"}
		}

	} else if balance < 0 {
		// Niedobór: Odblokowujemy OZE i próbujemy użyć baterii
		h.sendCurtail(ctx, -1)
		deficit := -balance
		provided, ok := h.batteryOp(ctx, energy.OpDischarge, deficit)
		if !ok {
			return
		}
		remainingDeficit := deficit - provided

		if remainingDeficit > 0 {
			// Bateria pusta, wciąż brakuje prądu -> Load Shedding
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

	// 4. Logowanie i Raportowanie - SoC pobierany przez kanał baterii
	soc, ok := h.batteryOp(ctx, energy.OpGetSoC, 0)
	if !ok {
		return
	}

	statsData := stats.SystemStats{
		TimeStep:       h.stepCount,
		WindSpeed:      weather.WindSpeed,
		Sun:            weather.Sun,
		RenewablePower: windPower,
		CoalPower:      coalPower,
		Demand:         totalDemand,
		BatterySoC:     soc * 100, // % SoC
		SystemStatus:   systemStatus,
	}

	// Logger przez kanał - non-blocking, by nie blokować pętli bilansu, gdy worker nie nadąża
	select {
	case h.loggerChan <- statsData:
	default:
	}

	// Wyświetlanie raportu w konsoli co N kroków (np. co 12 godzin symulacji)
	if h.stepCount%12 == 0 {
		fmt.Printf("\n[Raport Krok %d]\n", h.stepCount)
		fmt.Printf("[Pogoda] Wiatr: %.1f km/h | Słońce: %.1f%%\n", weather.WindSpeed, weather.Sun)
		fmt.Printf("[Produkcja] OZE: %.1f MW | Konwencjonalna: %.1f MW | Baterie: %.1f%% (SoC)\n", windPower, coalPower, soc*100)
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

	// Sortowanie rosnące po numerze priorytetu: Critical (1) -> Industrial (2) -> Residential (3).
	// Alokujemy moc od początku listy, więc Critical obsługiwany jest pierwszy,
	// a Residential (najniższy priorytet wg PDF) odpada jako pierwszy.
	sort.Slice(consumers, func(i, j int) bool {
		return consumers[i].Priority < consumers[j].Priority
	})

	remainingPower := availablePower

	for _, req := range consumers {
		if remainingPower >= req.PDemand {
			// Wystarczy energii dla tego odbiorcy
			remainingPower -= req.PDemand
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: req.PDemand, Reason: "OK"}
		} else {
			// Brak prądu (lub częściowy brak), odłączamy konsumenta
			req.ReplyChan <- core.SupplyStatus{AllocatedMW: 0, Reason: "LoadShed"}
		}
	}
}
