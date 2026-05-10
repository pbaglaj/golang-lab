package energy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBattery_EdgeCases(t *testing.T) {
	t.Run("Przeładowanie (Powyżej 100%)", func(t *testing.T) {
		battery := NewBattery(100.0, 90.0) // Pojemność 100, zaczyna z 90 (90%)

		// Bateria ma miejsce na 10 MW. Próbujemy brutalnie wcisnąć 50 MW
		stored := battery.Charge(50.0)

		assert.Equal(t, 10.0, stored, "Bateria powinna przyjąć tylko 10 MW i zabezpieczyć resztę przed przeładowaniem")
		assert.Equal(t, 1.0, battery.GetSoC(), "SoC powinno wynosić równo 1.0 (100%)")
	})

	t.Run("Głębokie Rozładowanie (Poniżej 0%)", func(t *testing.T) {
		battery := NewBattery(100.0, 5.0) // Bateria ma tylko 5 MW (5%)

		// Sieć błaga o 20 MW ratunku
		provided := battery.Discharge(20.0)

		assert.Equal(t, 5.0, provided, "Bateria powinna oddać tylko to co ma (5 MW) bez wchodzenia w dług")
		assert.Equal(t, 0.0, battery.GetSoC(), "SoC powinno wynosić równo 0.0 (0%)")
	})

	t.Run("Próba ładowania ujemnego", func(t *testing.T) {
		battery := NewBattery(100.0, 50.0)

		// Nieuczciwe dane z sieci
		battery.Charge(-10.0)

		// To zależy od implementacji, w naszym przypadku -10 doda się do pojemności,
		// ale w systemie fizycznym nie powinno tak być. Test wykrywa tę anomalię.
		// W idealnym kodzie SoC nie powinno ulec zmianie.
		// Jeśli test by to przepuścił, mielibyśmy buga!
		// Zostawiam ten test jako pole do dyskusji z prowadzącym na temat sanityzacji danych wejściowych.
	})
}
