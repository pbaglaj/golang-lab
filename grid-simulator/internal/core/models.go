package core

// DemandReport to żądanie konsumenta wysyłane do węzła GridHub
type DemandReport struct {
	ID        string
	PDemand   float64           // Aktualne zapotrzebowanie odbiorcy w MW
	Priority  int               // Typ profilu klienta
	ReplyChan chan SupplyStatus // Kanał zwrotny, żeby odbiorca otrzymał SupplyStatus z GridHub
}

// SupplyStatus to fizyczny przydział mocy zwrotnie wysłany przez Hub
type SupplyStatus struct {
	AllocatedMW float64
	Reason      string // Przydatne np. do oznaczania "LoadShed" przy braku energii
}

// WeatherData przechowuje aktualne metryki stacji pogodowej.
type WeatherData struct {
	WindSpeed float64
	Sun       float64
}

// ForecastReport to prognoza stworzona na bazie odczytów WeatherStep
type ForecastReport struct {
	TrendPercentage float64
	StepsAhead      int
}
