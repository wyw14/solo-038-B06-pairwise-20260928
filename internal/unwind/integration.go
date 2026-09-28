package unwind

import (
	"fmt"

	"github.com/wyw14/solo-038/internal/sensor"
)

type TensionStatus struct {
	Permitted bool                `json:"permitted"`
	Votes     []sensor.SourceVote `json:"votes"`
	Reason    string              `json:"reason"`
}

func EvaluateTension(readings []sensor.Reading, tolerance float64) (TensionStatus, error) {
	votes, err := sensor.IndependentVotes(readings)
	if err != nil {
		return TensionStatus{}, err
	}
	agree, err := sensor.Agree(votes, tolerance)
	if err != nil {
		return TensionStatus{}, err
	}
	status := TensionStatus{Permitted: agree, Votes: votes}
	originsPerKind := map[sensor.ReadingKind]int{}
	for _, vote := range votes {
		originsPerKind[vote.Kind]++
	}
	comparable := false
	for _, origins := range originsPerKind {
		if origins >= 2 {
			comparable = true
			break
		}
	}
	if !comparable {
		status.Reason = "fewer than two independent sources of the same kind"
	} else if !agree {
		status.Reason = fmt.Sprintf("source spread exceeds %.3f", tolerance)
	} else {
		status.Reason = "independent sources agree"
	}
	return status, nil
}

func ApplyFeed(state *State, owner string, generation uint64, speed float64, status TensionStatus) error {
	if !status.Permitted {
		return fmt.Errorf("web tension is not permitted: %s", status.Reason)
	}
	return state.StartFeed(owner, generation, speed)
}
