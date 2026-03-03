package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	boxes := [3]int{0, 0, 0}
	winningIndex := r.Intn(3)
	boxes[winningIndex] = 1

	// fmt.Print("Wybierz pudełko (1-3): ")
	// var input int
	// _, err := fmt.Scanln(&input)

	fmt.Print("Wybierz N ilość pudełek:")
	var n int
	_, err := fmt.Scanln(&n)

	if err != nil || n < 3 {
		fmt.Println("Błąd: Wybierz liczbę większą lub równą 3")
		return
	}

	fmt.Print("Wybierz pudełko (1-%d): ", n)
	var input int
	_, err := fmt.Scanln(&input)

	if err != nil || input < 1 || input > n {
		fmt.Println("Błąd: Wybierz liczbę od 1 do", n)

		return
	}

	userIndex := input - 1

	var openedByHost int
	for i := 0; i < 3; i++ {
		if i != userIndex && i != winningIndex {
			openedByHost = i
			break
		}
	}

	fmt.Printf("\nWybrałeś pudełko nr %d.\n", input)
	fmt.Printf("Prowadzący otwiera pudełko nr %d i pokazuje, że jest PUSTE!\n", openedByHost+1)

	fmt.Print("Czy chcesz zmienić swój wybór na ostatnie pozostałe pudełko? (t/n): ")
	var decision string
	fmt.Scanln(&decision)

	if decision == "t" {
		for i := 0; i < 3; i++ {
			if i != userIndex && i != openedByHost {
				userIndex = i
				break
			}
		}
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
