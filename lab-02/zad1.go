package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	fmt.Print("Wybierz N ilość pudełek:")
	var n int
	_, err := fmt.Scanln(&n)

	if err != nil || n < 3 {
		fmt.Println("Błąd: Wybierz liczbę większą lub równą 3")
		return
	}

	maxK := n - 2
	fmt.Printf("Wybierz k ilość pudełek otwieranych przez prowadzącego (1-%d): ", maxK)
	var k int
	_, err = fmt.Scanln(&k)
	if err != nil || k < 1 || k > maxK {
		fmt.Printf("Błąd: Wybierz liczbę od 1 do %d\n", maxK)
		return
	}

	boxes := make([]int, n)
	winningIndex := r.Intn(n)
	boxes[winningIndex] = 1

	var input int
	fmt.Printf("Wybierz pudełko (1-%d): ", n)
	_, err = fmt.Scanln(&input)

	if err != nil || input < 1 || input > n {
		fmt.Printf("Błąd: Wybierz liczbę od 1 do %d\n", n)

		return
	}

	userIndex := input - 1

	var availableToOpen []int
	for i := 0; i < n; i++ {
		if i != userIndex && i != winningIndex {
			availableToOpen = append(availableToOpen, i)
		}
	}

	openedByHost := availableToOpen[:k]

	isOpen := make([]bool, n)
	for _, box := range openedByHost {
		isOpen[box] = true
	}

	fmt.Printf("\nWybrałeś pudełko nr %d.\n", input)
	fmt.Print("Prowadzący otwiera pudełka nr: ")
	for i, box := range openedByHost {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%d", box+1)
	}
	fmt.Println(" i pokazuje, że są PUSTE!")

	fmt.Print("Czy chcesz zmienić swój wybór na inne pozostałe pudełko? (t/n): ")
	var decision string
	fmt.Scanln(&decision)

	if decision == "t" {
		var remaining []int
		for i := 0; i < n; i++ {
			if i != userIndex && !isOpen[i] {
				remaining = append(remaining, i)
			}
		}

		// tutaj zmiana na wybranie przez uzytkownika spośród pozostałych pudełek?
		userIndex = remaining[r.Intn(len(remaining))]
		fmt.Printf("Zmieniłeś wybór na pudełko nr %d.\n", userIndex+1)
	} else {
		fmt.Println("Zostajesz przy swoim pierwotnym wyborze.")
	}

	if userIndex == winningIndex {
		fmt.Println("GRATULACJE! Wygrałeś!")
	} else {
		fmt.Printf("Niestety, nagroda była w pudełku nr %d. Przegrałeś.\n", winningIndex+1)
	}
}
