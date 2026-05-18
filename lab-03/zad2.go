package main

import (
	"fmt"
	"sort"
)

type Performance struct {
	Piece  string
	Scores []float64
}

type Participant struct {
	ID           int
	Name         string
	Performances []Performance
	FinalScore   float64
}

type ScoringStrategy func(scores []float64) float64

func AddPerformanceToParticipant(p Participant, piece string, scores []float64) Participant {
	newPerformances := make([]Performance, len(p.Performances), len(p.Performances)+1)

	copy(newPerformances, p.Performances)

	newPerformances = append(newPerformances, Performance{Piece: piece, Scores: scores})

	newP := p
	newP.Performances = newPerformances
	return newP
}

func ChopinCorrectionStrategy(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	var sum float64
	for _, score := range scores {
		sum += score
	}
	rawAvg := sum / float64(len(scores))

	var correctedSum float64
	for _, score := range scores {
		if score > rawAvg+3.0 {
			correctedSum += rawAvg + 3.0
		} else if score < rawAvg-3.0 {
			correctedSum += rawAvg - 3.0
		} else {
			correctedSum += score
		}
	}

	return correctedSum / float64(len(scores))
}

func EvaluateParticipants(participants []Participant, strategy ScoringStrategy) []Participant {
	for i, p := range participants {
		var allScores []float64
		for _, perf := range p.Performances {
			allScores = append(allScores, strategy(perf.Scores))
		}
		participants[i].FinalScore = strategy(allScores)
	}
	return participants
}

func SortParticipantsByScore(participants []Participant) []Participant {
	sorted := make([]Participant, len(participants))
	copy(sorted, participants)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FinalScore > sorted[j].FinalScore
	})

	return sorted
}

func GetBestInPiece(participants []Participant, piece string, strategy ScoringStrategy) (Participant, float64) {
	var best Participant
	best.FinalScore = -1.0

	for _, p := range participants {
		for _, perf := range p.Performances {
			if perf.Piece == piece {
				score := strategy(perf.Scores)
				if score > best.FinalScore {
					best = p
					best.FinalScore = score
				}
			}
		}
	}

	return best, best.FinalScore
}

func main() {
	p1 := Participant{ID: 1, Name: "Jan Kowalski"}
	p2 := Participant{ID: 2, Name: "Anna Nowak"}
	p3 := Participant{ID: 3, Name: "Piotr Wiśniewski"}

	fmt.Print("Uczestnicy konkursu:\n")
	fmt.Printf("1. %s\n", p1.Name)
	fmt.Printf("2. %s\n", p2.Name)
	fmt.Printf("3. %s\n", p3.Name)

	repertoire := []string{"Etiuda a-moll", "Ballada g-moll", "Polonez As-dur"}

	p1 = AddPerformanceToParticipant(p1, repertoire[0], []float64{8.5, 9.0, 8.0, 9.5, 10.0})
	p1 = AddPerformanceToParticipant(p1, repertoire[1], []float64{11.0, 12.0, 27.0, 9.0, 8.5})
	p1 = AddPerformanceToParticipant(p1, repertoire[2], []float64{8.5, 13.0, 2.0, 9.0, 8.0})

	p2 = AddPerformanceToParticipant(p2, repertoire[0], []float64{9.0, 8.0, 7.0, 6.0, 8.0})
	p2 = AddPerformanceToParticipant(p2, repertoire[1], []float64{6.0, 8.5, 7.0, 8.0, 7.5})
	p2 = AddPerformanceToParticipant(p2, repertoire[2], []float64{7.5, 8.0, 7.0, 8.5, 7.0})

	p3 = AddPerformanceToParticipant(p3, repertoire[0], []float64{9.0, 10.0, 9.0, 8.5, 9.0})
	p3 = AddPerformanceToParticipant(p3, repertoire[1], []float64{11.0, 9.5, 9.0, 8.5, 9.0})
	p3 = AddPerformanceToParticipant(p3, repertoire[2], []float64{9.0, 9.5, 13.0, 12.0, 11.0})

	evaluated := EvaluateParticipants([]Participant{p1, p2, p3}, ChopinCorrectionStrategy)

	// fmt.Print(evaluated)
	// fmt.Print("\n")

	bestParticipant, bestScore := GetBestInPiece(evaluated, "Ballada g-moll", ChopinCorrectionStrategy)

	fmt.Printf("Najlepszy w Balladzie g-moll: %s - %.2f pkt\n", bestParticipant.Name, bestScore)

	sorted := SortParticipantsByScore(evaluated)

	for _, p := range sorted {
		fmt.Printf("%s - %.2f pkt\n", p.Name, p.FinalScore)
	}
}
