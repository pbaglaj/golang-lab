// go mod init main
// go fmt nazwa.go -> formatowanie kodu
// go build -o nazwa.exe -> do pliku wynikowego
// go build -> po prostu kompiluje
// go run nazwa.go -> kompiluje i uruchamia

package main

// fmt - formatowanie tekstu, wejścia i wyjścia
import "fmt"

func main() {

	var liczba1, liczba2 int = 10, 20
	fmt.Println("Liczba 1:", liczba1)
	fmt.Println("Liczba 2:", liczba2)

	fmt.Println("To jest pierwszy program")
}
