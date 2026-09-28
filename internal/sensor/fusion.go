package sensor

import (
	"fmt"
	"math"
	"sort"
)

// SourceVote is one origin's averaged position on a single kind of
// measurement, expressed in that kind's canonical unit. Votes of
// different kinds are never compared with each other.
type SourceVote struct {
	Origin  string      `json:"origin"`
	Kind    ReadingKind `json:"kind"`
	Value   float64     `json:"value"`
	Unit    string      `json:"unit"`
	Samples int         `json:"samples"`
}

// IndependentVotes reduces readings to one vote per (kind, origin) pair.
// Values are normalized to the kind's canonical unit before averaging, so
// only same-kind, mutually convertible measurements are ever combined.
// A reading derived from another (e.g. tension derived from a load cell)
// carries its parent's origin and therefore cannot become a new
// independent source; it only joins that origin's vote for its own kind.
func IndependentVotes(readings []Reading) ([]SourceVote, error) {
	type bucket struct {
		kind   ReadingKind
		origin string
	}
	totals := map[bucket]float64{}
	counts := map[bucket]int{}
	units := map[bucket]string{}
	for _, reading := range readings {
		if err := reading.Validate(); err != nil {
			return nil, err
		}
		value, unit, err := reading.CanonicalValue()
		if err != nil {
			return nil, err
		}
		key := bucket{kind: reading.Kind, origin: reading.Origin}
		totals[key] += value
		counts[key]++
		units[key] = unit
	}
	votes := make([]SourceVote, 0, len(totals))
	for key, total := range totals {
		votes = append(votes, SourceVote{Origin: key.origin, Kind: key.kind, Value: total / float64(counts[key]), Unit: units[key], Samples: counts[key]})
	}
	sort.Slice(votes, func(left, right int) bool {
		if votes[left].Kind != votes[right].Kind {
			return votes[left].Kind < votes[right].Kind
		}
		return votes[left].Origin < votes[right].Origin
	})
	return votes, nil
}

// Agree reports whether independent origins agree. Only votes of the same
// kind are ever compared: at least one kind must have two or more
// independent origins, and every kind that does must stay within
// tolerance. A kind with a single origin cannot establish agreement, and
// a kind whose origins disagree vetoes it.
func Agree(votes []SourceVote, tolerance float64) (bool, error) {
	if tolerance < 0 {
		return false, fmt.Errorf("tolerance cannot be negative")
	}
	byKind := map[ReadingKind][]float64{}
	for _, vote := range votes {
		byKind[vote.Kind] = append(byKind[vote.Kind], vote.Value)
	}
	decidable := false
	for _, values := range byKind {
		if len(values) < 2 {
			continue
		}
		decidable = true
		minimum, maximum := values[0], values[0]
		for _, value := range values[1:] {
			minimum = math.Min(minimum, value)
			maximum = math.Max(maximum, value)
		}
		if maximum-minimum > tolerance {
			return false, nil
		}
	}
	return decidable, nil
}
