package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"
)

// --- STAŁE SYMULACJI ---
const (
	WeatherStep         = 5 * time.Millisecond
	GridStep            = 100 * time.Millisecond
	WeatherPerGrid      = int(GridStep / WeatherStep) // 12 kroków
	ForecastHorizon     = 5
	PredictorBufferSize = WeatherPerGrid
)

// --- STRUKTURY DANYCH ---
type WeatherData struct {
	WindSpeed float64
	SunVolume float64
}

type DemandReport struct {
	ID       string
	PDemand  float64
	Priority int // 3: Residential, 2: Industrial, 1: Critical
	ReplyCh  chan SupplyStatus
}

type SupplyStatus struct {
	AllocatedMW float64
	Reason      string
}

type ForecastReport struct {
	TrendPercentage float64
}

// --- INTERFEJSY ---
type EnergySource interface {
	GetPower() float64
	SetCurtailment(limit float64)
	Start()
	Stop()
}

type Predictor interface {
	Start(ctx context.Context, weatherCh <-chan WeatherData, forecastCh chan<- ForecastReport)
}

type Consumer interface {
	Start(ctx context.Context, demandChan chan<- DemandReport)
}

type EnergyStorage interface {
	Charge(amount float64) float64
	Discharge(amount float64) float64
	GetSoC() float64
}

type WeatherProvider interface {
	Start(ctx context.Context, broadcastCh chan<- WeatherData)
}

type DataLogger interface {
	Log(msg string)
	Flush()
	Start(ctx context.Context)
}

// --- WEATHER STATION ---
type weatherStation struct {
	wind float64
	sun  float64
}

func NewWeatherStation() WeatherProvider {
	return &weatherStation{wind: 10.0, sun: 50.0}
}

func (ws *weatherStation) Start(ctx context.Context, outCh chan<- WeatherData) {
	ticker := time.NewTicker(WeatherStep)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Vt+1 = Vt + random(-1, 1)
			ws.wind += (rand.Float64() * 2) - 1
			ws.sun += (rand.Float64() * 2) - 1
			if ws.wind < 0 {
				ws.wind = 0
			}
			if ws.sun < 0 {
				ws.sun = 0
			}
			if ws.sun > 100 {
				ws.sun = 100
			}

			outCh <- WeatherData{WindSpeed: ws.wind, SunVolume: ws.sun}
		}
	}
}

// --- BROADCASTER (PUB/SUB) ---
type Broadcaster struct {
	subscribers []chan WeatherData
	mu          sync.Mutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make([]chan WeatherData, 0),
	}
}

func (b *Broadcaster) Subscribe() chan WeatherData {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan WeatherData, 1)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

func (b *Broadcaster) Start(ctx context.Context, inCh <-chan WeatherData) {
	for {
		select {
		case <-ctx.Done():
			return
		case data := <-inCh:
			b.mu.Lock()
			for _, ch := range b.subscribers {
				select {
				case ch <- data:
				default:
					// Subskrybent zajęty - porzucamy paczkę
				}
			}
			b.mu.Unlock()
		}
	}
}

// --- PREDICTOR ---
type predictorImpl struct {
	buffer []WeatherData
}

func NewPredictor() Predictor {
	return &predictorImpl{
		buffer: make([]WeatherData, 0, PredictorBufferSize),
	}
}

func (p *predictorImpl) Start(ctx context.Context, weatherCh <-chan WeatherData, forecastCh chan<- ForecastReport) {
	ticker := time.NewTicker(GridStep)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case w := <-weatherCh:
			p.buffer = append(p.buffer, w)
			if len(p.buffer) > PredictorBufferSize {
				p.buffer = p.buffer[1:]
			}
		case <-ticker.C:
			if len(p.buffer) < 2 {
				continue
			}
			// Ekstrapolacja trendu (uproszczona)
			first := p.buffer[0]
			last := p.buffer[len(p.buffer)-1]
			trend := (last.WindSpeed - first.WindSpeed) / first.WindSpeed * 100

			select {
			case forecastCh <- ForecastReport{TrendPercentage: trend}:
			default:
			}
		}
	}
}

// --- ESS (MAGAZYN ENERGII) ---
type batteryESS struct {
	capacity float64
	current  float64
}

func NewESS(capacity float64) EnergyStorage {
	return &batteryESS{capacity: capacity, current: capacity * 0.5} // Start na 50%
}

func (b *batteryESS) Charge(amount float64) float64 {
	space := b.capacity - b.current
	if amount <= space {
		b.current += amount
		return amount
	}
	b.current = b.capacity
	return space
}

func (b *batteryESS) Discharge(amount float64) float64 {
	if b.current >= amount {
		b.current -= amount
		return amount
	}
	available := b.current
	b.current = 0
	return available
}

func (b *batteryESS) GetSoC() float64 {
	return b.current / b.capacity
}

// --- KONSUMENT ---
type basicConsumer struct {
	id       string
	priority int
	baseDem  float64
}

func NewConsumer(id string, priority int, baseDem float64) Consumer {
	return &basicConsumer{id: id, priority: priority, baseDem: baseDem}
}

func (c *basicConsumer) Start(ctx context.Context, demandChan chan<- DemandReport) {
	ticker := time.NewTicker(GridStep)
	replyCh := make(chan SupplyStatus, 1)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req := DemandReport{
				ID:       c.id,
				PDemand:  c.baseDem + (rand.Float64() * 5), // Prosta symulacja fluktuacji
				Priority: c.priority,
				ReplyCh:  replyCh,
			}
			demandChan <- req

			// Czekaj na odpowiedź
			select {
			case status := <-replyCh:
				if status.AllocatedMW < req.PDemand {
					// Brak pełnego zasilania - stan częściowego odłączenia
				}
			case <-time.After(GridStep / 2):
				// Timeout
			}
		}
	}
}

// --- DATALOGGER ---
type csvLogger struct {
	logCh chan string
	file  *os.File
	w     *bufio.Writer
}

func NewDataLogger(filename string) (DataLogger, error) {
	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &csvLogger{
		logCh: make(chan string, 100),
		file:  f,
		w:     bufio.NewWriter(f),
	}, nil
}

func (l *csvLogger) Log(msg string) {
	l.logCh <- msg
}

func (l *csvLogger) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-l.logCh:
			l.w.WriteString(msg + "\n")
		}
	}
}

func (l *csvLogger) Flush() {
	l.w.Flush()
	l.file.Close()
	fmt.Println("DataLogger pomyślnie zapisał dane.")
}

// --- GRIDHUB ---
type GridHub struct {
	ess        EnergyStorage
	demandChan chan DemandReport
	forecastCh chan ForecastReport
	logger     DataLogger
}

func NewGridHub(ess EnergyStorage, logger DataLogger) *GridHub {
	return &GridHub{
		ess:        ess,
		demandChan: make(chan DemandReport, 100), // Fan-In dla konsumentów
		forecastCh: make(chan ForecastReport, 1),
		logger:     logger,
	}
}

func (gh *GridHub) Start(ctx context.Context) {
	ticker := time.NewTicker(GridStep)
	defer ticker.Stop()

	var currentDemands []DemandReport
	productionOZE := 150.0 // Przyjęta stała dla uproszczenia (symulowana produkcja)

	for {
		select {
		case <-ctx.Done():
			return
		case forecast := <-gh.forecastCh:
			if forecast.TrendPercentage < -10.0 {
				gh.logger.Log("Predictor alarm: OZE spadnie, uruchamianie rezerw...")
			}
		case req := <-gh.demandChan:
			currentDemands = append(currentDemands, req)
		case <-ticker.C:
			// Zdarzenie 1: Bilansowanie co 1h (GridStep)
			totalDemand := 0.0
			for _, d := range currentDemands {
				totalDemand += d.PDemand
			}

			balance := productionOZE - totalDemand

			// Sortowanie wg priorytetu na wypadek Load Shedding (3->2->1, najwyższy to 1)
			sort.Slice(currentDemands, func(i, j int) bool {
				return currentDemands[i].Priority > currentDemands[j].Priority // 3 (najmniejszy priorytet) na początku
			})

			// Zdarzenie 3 & 4: Zarządzanie ESS i Load Shedding
			if balance > 0 {
				charged := gh.ess.Charge(balance)
				if charged < balance {
					gh.logger.Log("Nadwyżka produkcji, SoC 100% -> Curtailment")
				}
				// Zaspokój wszystkich
				for _, d := range currentDemands {
					d.ReplyCh <- SupplyStatus{AllocatedMW: d.PDemand, Reason: "OK"}
				}
			} else {
				shortage := -balance
				discharged := gh.ess.Discharge(shortage)
				remainingShortage := shortage - discharged

				if remainingShortage > 0 {
					// Load Shedding
					gh.logger.Log(fmt.Sprintf("CRITICAL: Deficyt %f MW, uruchamiam Load Shedding", remainingShortage))
					for _, d := range currentDemands {
						if remainingShortage > 0 {
							// Odłączamy konsumenta
							d.ReplyCh <- SupplyStatus{AllocatedMW: 0, Reason: "LoadShed"}
							remainingShortage -= d.PDemand
						} else {
							d.ReplyCh <- SupplyStatus{AllocatedMW: d.PDemand, Reason: "OK"}
						}
					}
				} else {
					for _, d := range currentDemands {
						d.ReplyCh <- SupplyStatus{AllocatedMW: d.PDemand, Reason: "OK"}
					}
				}
			}

			// Raport konsolowy
			fmt.Printf("[Sieć] Popyt: %.1f MW | Bilans: %.1f MW | SoC Baterii: %.1f%%\n",
				totalDemand, balance, gh.ess.GetSoC()*100)

			// Wyczyść kolejkę żądań na następny krok
			currentDemands = currentDemands[:0]
		}
	}
}

// --- FUNKCJA MAIN ---
func main() {
	// Konfiguracja Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Inicjalizacja komponentów
	logger, err := NewDataLogger("grid_history.csv")
	if err != nil {
		panic(err)
	}

	broadcaster := NewBroadcaster()
	weatherStation := NewWeatherStation()
	predictor := NewPredictor()
	ess := NewESS(500.0) // Bateria 500 MWh
	gridHub := NewGridHub(ess, logger)

	// Uruchamianie Gorutyn
	go logger.Start(ctx)

	weatherCh := make(chan WeatherData)
	go weatherStation.Start(ctx, weatherCh)
	go broadcaster.Start(ctx, weatherCh)

	predWeatherCh := broadcaster.Subscribe()
	go predictor.Start(ctx, predWeatherCh, gridHub.forecastCh)

	// Rejestracja konsumentów
	c1 := NewConsumer("Dom_1", 3, 20.0)
	c2 := NewConsumer("Fabryka_1", 2, 80.0)
	c3 := NewConsumer("Szpital_1", 1, 15.0)

	go c1.Start(ctx, gridHub.demandChan)
	go c2.Start(ctx, gridHub.demandChan)
	go c3.Start(ctx, gridHub.demandChan)

	go gridHub.Start(ctx)

	fmt.Println("Symulacja uruchomiona. Naciśnij Ctrl+C aby zakończyć...")

	// Oczekiwanie na sygnał
	<-sigCh
	fmt.Println("\nRozpoczęto zamykanie systemu (Graceful Shutdown)...")
	cancel()

	// Krótkie opóźnienie, aby gorutyny zdążyły zareagować na context
	time.Sleep(200 * time.Millisecond)
	logger.Flush()
}
