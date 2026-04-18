package main

import (
	"errors"
	"fmt"
)

type Airplane struct {
	Model    string
	Capacity int
}

type Passenger struct {
	ID        int
	FirstName string
	LastName  string
}

type Flight struct {
	Number           string
	Aircraft         Airplane
	Origin           string
	Destination      string
	BookedPassengers map[int]Passenger
}

func NewFlight(number string, aircraft Airplane, origin, destination string) *Flight {
	return &Flight{
		Number:           number,
		Aircraft:         aircraft,
		Origin:           origin,
		Destination:      destination,
		BookedPassengers: make(map[int]Passenger),
	}
}

type Reservation struct {
	ID        string
	Passenger Passenger
	Flight    *Flight
}

type Searcher interface {
	FindPassengerReservations(passengerID int) []Reservation
	FindFlightsFrom(port string) []*Flight
	FindFlightsTo(port string) []*Flight
}

type ReservationSystem struct {
	Flights      []*Flight
	Reservations []Reservation
	nextID       int
}

func NewReservationSystem() *ReservationSystem {
	return &ReservationSystem{
		Flights:      make([]*Flight, 0),
		Reservations: make([]Reservation, 0),
		nextID:       1,
	}
}

func (s *ReservationSystem) AddFlight(f *Flight) {
	s.Flights = append(s.Flights, f)
}

// Methods and Business Logic

// Check the number of available seats
func (f *Flight) AvailableSeats() int {
	return f.Aircraft.Capacity - len(f.BookedPassengers)
}

// Implementation of fmt.Stringer interface for Flight struct
func (f *Flight) String() string {
	return fmt.Sprintf("[Flight %s] %s -> %s | Aircraft: %s | Available seats: %d/%d",
		f.Number, f.Origin, f.Destination, f.Aircraft.Model, f.AvailableSeats(), f.Aircraft.Capacity)
}

func (s *ReservationSystem) Book(f *Flight, p Passenger) error {
	if f.AvailableSeats() <= 0 {
		return errors.New("no available seats on this flight")
	}

	// Prevent double booking
	if _, exists := f.BookedPassengers[p.ID]; exists {
		return fmt.Errorf("passenger %s %s already has a booking for flight %s", p.FirstName, p.LastName, f.Number)
	}

	// Add passenger to the flight
	f.BookedPassengers[p.ID] = p

	// Create reservation object
	newReservation := Reservation{
		ID:        fmt.Sprintf("RES-%d", s.nextID),
		Passenger: p,
		Flight:    f,
	}
	s.nextID++
	s.Reservations = append(s.Reservations, newReservation)

	return nil
}

// Cancel a reservation
func (s *ReservationSystem) Cancel(f *Flight, p Passenger) error {
	// Check if the passenger is actually booked
	if _, exists := f.BookedPassengers[p.ID]; !exists {
		return errors.New("reservation not found for this passenger on the specified flight")
	}

	// Remove passenger from the flight
	delete(f.BookedPassengers, p.ID)

	// Remove reservation from the global system registry
	for i, r := range s.Reservations {
		if r.Flight.Number == f.Number && r.Passenger.ID == p.ID {
			// Remove element from slice
			s.Reservations = append(s.Reservations[:i], s.Reservations[i+1:]...)
			break
		}
	}
	return nil
}

// Implementation of the Searcher interface by ReservationSystem
func (s *ReservationSystem) FindPassengerReservations(id int) []Reservation {
	var result []Reservation
	for _, r := range s.Reservations {
		if r.Passenger.ID == id {
			result = append(result, r)
		}
	}
	return result
}

func (s *ReservationSystem) FindFlightsFrom(port string) []*Flight {
	var result []*Flight
	for _, f := range s.Flights {
		if f.Origin == port {
			result = append(result, f)
		}
	}
	return result
}

func (s *ReservationSystem) FindFlightsTo(port string) []*Flight {
	var result []*Flight
	for _, f := range s.Flights {
		if f.Destination == port {
			result = append(result, f)
		}
	}
	return result
}

// Helper function demonstrating the use of the searcher interface
func PrintSearchResults(searcher Searcher, passengerID int, portFrom string, portTo string) {
	fmt.Println("\n--- SEARCH RESULTS (VIA INTERFACE) ---")

	// Searching for a specific passenger's reservations
	reservations := searcher.FindPassengerReservations(passengerID)
	fmt.Printf("Reservations for passenger ID=%d:\n", passengerID)
	if len(reservations) == 0 {
		fmt.Println(" - No reservations found")
	}
	for _, r := range reservations {
		fmt.Printf(" - %s (Flight: %s)\n", r.ID, r.Flight.Number)
	}

	// Searching for flights from a given port
	fmt.Printf("\nFlights from port: %s\n", portFrom)
	for _, f := range searcher.FindFlightsFrom(portFrom) {
		fmt.Printf(" - %s\n", f)
	}

	// Searching for flights to a given port
	fmt.Printf("\nFlights to port: %s\n", portTo)
	for _, f := range searcher.FindFlightsTo(portTo) {
		fmt.Printf(" - %s\n", f)
	}
	fmt.Println("--------------------------------------")
}

func main() {
	system := NewReservationSystem()

	boeing := Airplane{Model: "Boeing 737", Capacity: 150}
	cessna := Airplane{Model: "Cessna 172", Capacity: 2}

	flight1 := NewFlight("WAW-LHR-01", boeing, "Warsaw", "London")
	flight2 := NewFlight("KRK-GDN-02", cessna, "Krakow", "Gdansk")

	system.AddFlight(flight1)
	system.AddFlight(flight2)

	p1 := Passenger{ID: 1, FirstName: "John", LastName: "Doe"}
	p2 := Passenger{ID: 2, FirstName: "Anna", LastName: "Smith"}
	p3 := Passenger{ID: 3, FirstName: "Peter", LastName: "Jones"}

	fmt.Println("=== INITIAL STATE (Using Stringer interface) ===")
	fmt.Println(flight1)
	fmt.Println(flight2)

	fmt.Println("\n=== 1. BOOKINGS ===")
	if err := system.Book(flight1, p1); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("Booked flight %s for %s %s\n", flight1.Number, p1.FirstName, p1.LastName)
	}

	// Error: double booking
	if err := system.Book(flight1, p1); err != nil {
		fmt.Println("Expected Error (double booking):", err)
	}

	system.Book(flight2, p1)
	system.Book(flight2, p2)

	if err := system.Book(flight2, p3); err != nil {
		fmt.Println("Expected Error (no seats):", err)
	}

	fmt.Println("\n=== STATE AFTER BOOKINGS ===")
	fmt.Println(flight1)
	fmt.Println(flight2)

	PrintSearchResults(system, 1, "Krakow", "London")

	fmt.Println("\n=== 2. CANCELLING RESERVATIONS ===")
	if err := system.Cancel(flight2, p1); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("Cancelled reservation for %s %s from flight %s\n", p1.FirstName, p1.LastName, flight2.Number)
	}

	PrintSearchResults(system, 1, "Warsaw", "Gdansk")

	fmt.Println(flight1)
	fmt.Println(flight2)
}
