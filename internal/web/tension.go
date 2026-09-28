package web

import (
	"fmt"
	"time"

	"github.com/wyw14/solo-038/internal/sensor"
)

type TensionPermit struct {
	Allowed   bool      `json:"allowed"`
	Origins   []string  `json:"origins"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Reason    string    `json:"reason"`
}

func BuildTensionPermit(readings []sensor.Reading, tolerance float64, now time.Time) (TensionPermit, error) {
	votes, err := sensor.IndependentVotes(readings)
	if err != nil {
		return TensionPermit{}, err
	}
	agree, err := sensor.Agree(votes, tolerance)
	if err != nil {
		return TensionPermit{}, err
	}
	permit := TensionPermit{Allowed: agree, IssuedAt: now.UTC(), ExpiresAt: now.Add(2 * time.Second).UTC()}
	seen := map[string]bool{}
	for _, vote := range votes {
		if !seen[vote.Origin] {
			seen[vote.Origin] = true
			permit.Origins = append(permit.Origins, vote.Origin)
		}
	}
	if !permit.Allowed {
		permit.Reason = fmt.Sprintf("need two independent origins of the same kind to agree within %.3f, received %d votes", tolerance, len(votes))
	}
	return permit, nil
}

func (permit TensionPermit) ValidAt(now time.Time) bool {
	return permit.Allowed && !now.Before(permit.IssuedAt) && now.Before(permit.ExpiresAt)
}
