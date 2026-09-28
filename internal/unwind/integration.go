package unwind

import (
	"fmt"

	"github.com/wyw14/solo-038/internal/sensor"
)

type TensionStatus struct {
	Permitted bool                `json:"permitted"`
	Kind      string              `json:"kind"`
	Unit      string              `json:"unit"`
	Votes     []sensor.SourceVote `json:"votes"`
	Reason    string              `json:"reason"`
}

// EvaluateTension builds independent votes for a single measurement kind and
// reports whether at least two independent origins agree. Only readings of
// that kind vote; values in other units are normalised first and derived
// readings cannot second the sensor they came from.
func EvaluateTension(readings []sensor.Reading, kind sensor.ReadingKind, tolerance float64) (TensionStatus, error) {
	unit, err := sensor.CanonicalUnit(kind)
	if err != nil {
		return TensionStatus{}, err
	}
	votes, err := sensor.IndependentVotes(readings, kind)
	if err != nil {
		return TensionStatus{}, err
	}
	agree, err := sensor.Agree(votes, tolerance)
	if err != nil {
		return TensionStatus{}, err
	}
	status := TensionStatus{Permitted: agree && len(votes) >= 2, Kind: string(kind), Unit: unit, Votes: votes}
	if len(votes) < 2 {
		status.Reason = fmt.Sprintf("fewer than two independent %s sources", kind)
	} else if !agree {
		status.Reason = fmt.Sprintf("source spread exceeds %.3f %s", tolerance, unit)
	} else {
		status.Reason = fmt.Sprintf("independent %s sources agree in %s", kind, unit)
	}
	return status, nil
}

func ApplyFeed(state *State, owner string, generation uint64, speed float64, status TensionStatus) error {
	if !status.Permitted {
		return fmt.Errorf("web tension is not permitted: %s", status.Reason)
	}
	return state.StartFeed(owner, generation, speed)
}
