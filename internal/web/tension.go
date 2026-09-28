package web

import (
	"fmt"
	"time"

	"github.com/wyw14/solo-038/internal/sensor"
)

type TensionPermit struct {
	Allowed   bool      `json:"allowed"`
	Origins   []string  `json:"origins"`
	Kind      string    `json:"kind"`
	Unit      string    `json:"unit"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Reason    string    `json:"reason"`
}

// BuildTensionPermit issues a permit only when at least two independent
// origins agree on the same measurement kind. Readings of other kinds (for
// example a tension derived from one of the force readings) do not vote, so a
// derived value can never mask a disagreement between the underlying sensors.
func BuildTensionPermit(readings []sensor.Reading, kind sensor.ReadingKind, tolerance float64, now time.Time) (TensionPermit, error) {
	unit, err := sensor.CanonicalUnit(kind)
	if err != nil {
		return TensionPermit{}, err
	}
	votes, err := sensor.IndependentVotes(readings, kind)
	if err != nil {
		return TensionPermit{}, err
	}
	agree, err := sensor.Agree(votes, tolerance)
	if err != nil {
		return TensionPermit{}, err
	}
	permit := TensionPermit{
		Allowed:   agree && len(votes) >= 2,
		Kind:      string(kind),
		Unit:      unit,
		IssuedAt:  now.UTC(),
		ExpiresAt: now.Add(2 * time.Second).UTC(),
	}
	for _, vote := range votes {
		permit.Origins = append(permit.Origins, vote.Origin)
	}
	if !permit.Allowed {
		permit.Reason = fmt.Sprintf("need two agreeing independent %s origins in %s, received %d", kind, unit, len(votes))
	}
	return permit, nil
}

func (permit TensionPermit) ValidAt(now time.Time) bool {
	return permit.Allowed && !now.Before(permit.IssuedAt) && now.Before(permit.ExpiresAt)
}
