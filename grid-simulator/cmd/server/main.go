package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"grid-simulator/internal/consumer"
	"grid-simulator/internal/core"
	"grid-simulator/internal/energy"
	"grid-simulator/internal/grid"
	"grid-simulator/internal/stats"
	"grid-simulator/internal/weather"
)

func main() {
	// 1. Tworzymy kontekst, który można anulować (Graceful Shutdown)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Kanał do nasłuchiwania sygnałów OS
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 3. Gorutyna czekająca na Ctrl+C
	go func() {
		<-sigChan
		fmt.Println("\n[SYSTEM] Otrzymano sygnał przerwania. Zamykanie...")
		cancel() // Wysyła sygnał ctx.Done() do wszystkich gorutyn
	}()

	var wg sync.WaitGroup

	fmt.Println("[SYSTEM] Inicjalizacja komponentów...")

	// --- INICJALIZACJA KOMPONENTÓW ---

	// Logger
	logger, err := stats.NewCSVLogger("logs/grid_history.csv", &wg)
	if err != nil {
		fmt.Printf("[BŁĄD] Nie można zainicjować loggera: %v\n", err)
		return
	}

	// Infrastruktura pogodowa
	broadcaster := weather.NewBroadcaster()
	station := weather.NewStation(broadcaster)

	// Predictor
	predictor := grid.NewGridPredictor()

	// Źródła i Magazyn Energii
	// Celowe:

	windFarm := energy.NewWindFarm()
	coalPlant := energy.NewCoalPlant(50.0)    // Elektrownia węglowa o mocy 50 MW
	battery := energy.NewBattery(100.0, 50.0) // Bateria 100 MWh, początkowo w 50% pełna

	// Testowe:
	// windFarm := energy.NewWindFarm()
	// windFarm.SetCurtailment(0) // TEST: Natychmiastowe odcięcie wiatru (OZE produkuje 0 MW)

	// coalPlant := energy.NewCoalPlant(15.0)   // TEST: Słaby węgiel (tylko 15 MW, a zapotrzebowanie to ~25 MW)
	// battery := energy.NewBattery(100.0, 0.0) // TEST: Pusta bateria na start (SoC = 0%)

	// Główny Hub
	hub := grid.NewHub(windFarm, coalPlant, battery, logger)

	// Kanały komunikacyjne i subskrypcje
	hubWeatherChan := broadcaster.Subscribe()         // Hub pobiera aktualną pogodę
	predictorWeatherChan := broadcaster.Subscribe()   // Predictor pobiera pogodę do bufora
	forecastChan := make(chan core.ForecastReport, 1) // Kanał dla prognoz (bufor = 1)

	// Konsumenci
	consumers := []*consumer.Node{
		consumer.New("Osiedle_A", core.PriorityResidential, consumer.ResidentialProfile()),
		consumer.New("Fabryka_B", core.PriorityIndustrial, consumer.IndustrialProfile()),
		consumer.New("Szpital_C", core.PriorityCritical, consumer.CriticalProfile()),
	}

	// --- URUCHAMIANIE GORUTYN ---

	fmt.Println("[SYSTEM] Uruchamianie symulacji...")

	// Start Loggera (posiada własną obsługę WaitGroup)
	logger.Start(ctx)

	// Start Predictora
	wg.Add(1)
	go func() {
		defer wg.Done()
		predictor.Start(ctx, predictorWeatherChan, forecastChan)
	}()

	// Start Stacji Pogodowej
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Pusty kanał wysyłkowy jako drugi argument (stacja używa bezpośrednio Broadcastera)
		station.Start(ctx, nil)
	}()

	// Start Hub-a
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Przekazujemy wygenerowane subskrypcje
		go func() {
			for w := range hubWeatherChan {
				hub.GetWeatherChan() <- w
			}
		}()
		hub.Start(ctx, forecastChan)
	}()

	// Start Konsumentów
	for _, c := range consumers {
		wg.Add(1)
		go func(node *consumer.Node) {
			defer wg.Done()
			node.Run(ctx, hub.GetDemandChan())
		}(c)
	}

	// TEST 2: Dynamiczne dodanie konsumenta w trakcie działania
	go func() {
		// Czekamy 3 sekundy realnego czasu (co odpowiada ok. 30 godzinom symulacji)
		time.Sleep(3 * time.Second)
		fmt.Println("\n========================================================")
		fmt.Println("[TEST] ---> PODŁĄCZAM NOWE MIASTO DO SIECI! <---")
		fmt.Println("========================================================")

		noweMiasto := consumer.New("Nowe_Miasto_D", core.PriorityResidential, consumer.ResidentialProfile())

		wg.Add(1)
		go func() {
			defer wg.Done()
			// Podłączamy nowe miasto "w locie" pod ten sam, istniejący już kanał Fan-In
			noweMiasto.Run(ctx, hub.GetDemandChan())
		}()
	}()

	// 4. Czekamy na zakończenie wszystkich gorutyn
	wg.Wait()
	fmt.Println("[SYSTEM] Wszystkie komponenty zamknięte. Zrzut danych zakończony (Flush). Koniec.")
}
