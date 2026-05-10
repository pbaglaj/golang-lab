package grid

import (
	"testing"

	"grid-simulator/internal/core"

	"github.com/stretchr/testify/assert"
)

func TestHub_ExecuteLoadShedding(t *testing.T) {
	// Definiujemy tabelę z różnymi scenariuszami awaryjnymi
	tests := []struct {
		name           string
		availablePower float64
		consumers      map[string]core.DemandReport
		expected       map[string]float64 // Mapa [ID_Odbiorcy]OczekiwaneMW
	}{
		{
			name:           "Niedobór - prądu starcza tylko dla priorytetu 1 (Szpital)",
			availablePower: 10.0,
			consumers: map[string]core.DemandReport{
				"Szpital_C": {ID: "Szpital_C", Priority: core.PriorityCritical, PDemand: 10.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Fabryka_B": {ID: "Fabryka_B", Priority: core.PriorityIndustrial, PDemand: 15.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Osiedle_A": {ID: "Osiedle_A", Priority: core.PriorityResidential, PDemand: 5.0, ReplyChan: make(chan core.SupplyStatus, 1)},
			},
			expected: map[string]float64{
				"Szpital_C": 10.0, // Dostaje wszystko
				"Fabryka_B": 0.0,  // Odłączona (LoadShed)
				"Osiedle_A": 0.0,  // Odłączone (LoadShed)
			},
		},
		{
			name:           "Częściowy niedobór - prądu starcza dla priorytetu 1 i 2, odcina 3",
			availablePower: 25.0,
			consumers: map[string]core.DemandReport{
				"Szpital_C": {ID: "Szpital_C", Priority: core.PriorityCritical, PDemand: 10.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Fabryka_B": {ID: "Fabryka_B", Priority: core.PriorityIndustrial, PDemand: 15.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Osiedle_A": {ID: "Osiedle_A", Priority: core.PriorityResidential, PDemand: 5.0, ReplyChan: make(chan core.SupplyStatus, 1)},
			},
			expected: map[string]float64{
				"Szpital_C": 10.0, // Dostaje wszystko (10MW)
				"Fabryka_B": 15.0, // Dostaje resztę z puli 25MW (15MW)
				"Osiedle_A": 0.0,  // Zabrakło prądu, odłączone (LoadShed)
			},
		},
		{
			name:           "Skrajny niedobór - odcięci absolutnie wszyscy, nawet szpital",
			availablePower: 5.0,
			consumers: map[string]core.DemandReport{
				"Szpital_C": {ID: "Szpital_C", Priority: core.PriorityCritical, PDemand: 10.0, ReplyChan: make(chan core.SupplyStatus, 1)},
			},
			expected: map[string]float64{
				"Szpital_C": 0.0, // Pula 5MW to za mało na potrzeby 10MW, odcięty
			},
		},
		{
			name:           "Warunek Brzegowy: Idealny styk (Zapotrzebowanie równe dokładnie puli)",
			availablePower: 30.0, // Równo 30 MW
			consumers: map[string]core.DemandReport{
				"Szpital_C": {ID: "Szpital_C", Priority: core.PriorityCritical, PDemand: 10.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Fabryka_B": {ID: "Fabryka_B", Priority: core.PriorityIndustrial, PDemand: 15.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Osiedle_A": {ID: "Osiedle_A", Priority: core.PriorityResidential, PDemand: 5.0, ReplyChan: make(chan core.SupplyStatus, 1)},
			},
			expected: map[string]float64{
				"Szpital_C": 10.0, // Wszyscy muszą dostać 100% zasilania
				"Fabryka_B": 15.0,
				"Osiedle_A": 5.0,
			},
		},
		{
			name:           "Warunek Brzegowy: Puste żądania z sieci (Demand = 0)",
			availablePower: 10.0,
			consumers: map[string]core.DemandReport{
				"Szpital_C": {ID: "Szpital_C", Priority: core.PriorityCritical, PDemand: 10.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				// Fabryka i Osiedle mają zerowe zapotrzebowanie
				"Fabryka_B": {ID: "Fabryka_B", Priority: core.PriorityIndustrial, PDemand: 0.0, ReplyChan: make(chan core.SupplyStatus, 1)},
				"Osiedle_A": {ID: "Osiedle_A", Priority: core.PriorityResidential, PDemand: 0.0, ReplyChan: make(chan core.SupplyStatus, 1)},
			},
			expected: map[string]float64{
				"Szpital_C": 10.0,
				"Fabryka_B": 0.0, // Żąda 0, dostaje 0
				"Osiedle_A": 0.0, // Żąda 0, dostaje 0
			},
		},
	}

	for _, tt := range tests {
		// Odpalamy każdy scenariusz jako osobny podtest
		t.Run(tt.name, func(t *testing.T) {
			// Inicjujemy atrapę Hub-a tylko z potrzebnymi danymi (biała skrzynka)
			hub := &Hub{
				consumers: tt.consumers,
			}

			// Uruchamiamy testowaną metodę
			hub.executeLoadShedding(tt.availablePower)

			// Sprawdzamy wyniki za pomocą kanałów zwrotnych
			for id, expectedMW := range tt.expected {
				req := tt.consumers[id]

				// Pobieramy status z kanału zwrotnego
				status := <-req.ReplyChan

				// Asercja: Czy otrzymał dokładnie tyle MW, ile zakładamy w teście?
				assert.Equal(t, expectedMW, status.AllocatedMW, "Błędny przydział mocy dla: "+id)

				// Asercja: Czy powód odłączenia jest prawidłowo oznaczany?
				if expectedMW < req.PDemand {
					// Jeśli dostał mniej, niż prosił (niedobór), to powodem musi być LoadShed
					assert.Equal(t, "LoadShed", status.Reason, "Powód powinien być LoadShed dla: "+id)
				} else {
					// Jeśli dostał 100% tego, o co prosił (nawet jeśli to było 0 MW), to powód "OK"
					assert.Equal(t, "OK", status.Reason, "Powód powinien być OK dla: "+id)
				}
			}
		})
	}
}
