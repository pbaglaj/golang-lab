package core

// DemandReport to żądanie konsumenta wysyłane do węzła GridHub[cite: 6].
type DemandReport struct {
	ID        string
	PDemand   float64           // Aktualne zapotrzebowanie odbiorcy w MW [cite: 83]
	Priority  int               // Typ profilu klienta [cite: 83]
	ReplyChan chan SupplyStatus // Kanał zwrotny, żeby odbiorca otrzymał SupplyStatus z GridHub [cite: 84]
}

// SupplyStatus to fizyczny przydział mocy zwrotnie wysłany przez Hub[cite: 6].
type SupplyStatus struct {
	AllocatedMW float64
	Reason      string // Przydatne np. do oznaczania "LoadShed" przy braku energii [cite: 102]
}

// WeatherData przechowuje aktualne metryki stacji pogodowej.
type WeatherData struct {
	WindSpeed float64
	Sun       float64
}

// ForecastReport to prognoza stworzona na bazie odczytów WeatherStep[cite: 23, 67, 125].
type ForecastReport struct {
	TrendPercentage float64
	StepsAhead      int
}
