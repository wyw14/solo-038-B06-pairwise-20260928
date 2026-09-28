package sensor

import (
	"fmt"
	"math"
	"sort"
)

type SourceVote struct {
	Origin string  `json:"origin"`
	Kind   string  `json:"kind"`
	Value  float64 `json:"value"`
	// Unit is the canonical unit Value is expressed in. Votes are only
	// comparable when Kind and Unit match.
	Unit string `json:"unit"`
	// Kinds is retained for reporting; it is always 1 for independent votes,
	// which are scoped to a single measurement kind.
	Kinds int `json:"kinds"`
}

type originTotal struct {
	observed []float64
	derived  []float64
}

// IndependentVotes builds one vote per independent sensor origin for a single
// measurement kind. Readings of other kinds are ignored and every value is
// converted to the kind's canonical unit, so different dimensions are never
// compared.
//
// Votes are keyed by the reading's root origin: a value derived from a sensor
// shares that sensor's origin and cannot count as a second source for it. When
// an origin supplies both directly observed and derived readings of the same
// kind, only the observed readings vote. Tensions derived independently from
// two different load cells have different roots and therefore still count as
// two genuinely independent sources.
func IndependentVotes(readings []Reading, kind ReadingKind) ([]SourceVote, error) {
	canonicalUnit, err := CanonicalUnit(kind)
	if err != nil {
		return nil, err
	}
	byOrigin := map[string]*originTotal{}
	for _, reading := range readings {
		if reading.Kind != kind {
			continue
		}
		if err := reading.Validate(); err != nil {
			return nil, err
		}
		value, unit, err := canonicalize(reading)
		if err != nil {
			return nil, err
		}
		if unit != canonicalUnit {
			return nil, fmt.Errorf("internal error: %s canonicalized to %s", kind, unit)
		}
		total, ok := byOrigin[reading.Origin]
		if !ok {
			total = &originTotal{}
			byOrigin[reading.Origin] = total
		}
		if reading.IsDerived() {
			total.derived = append(total.derived, value)
		} else {
			total.observed = append(total.observed, value)
		}
	}
	votes := make([]SourceVote, 0, len(byOrigin))
	for origin, total := range byOrigin {
		values := total.observed
		if len(values) == 0 {
			// The origin does not measure this kind directly; a value derived
			// from it is its sole vote, but still counts once.
			values = total.derived
		}
		sum := 0.0
		for _, value := range values {
			sum += value
		}
		votes = append(votes, SourceVote{
			Origin: origin,
			Kind:   string(kind),
			Value:  sum / float64(len(values)),
			Unit:   canonicalUnit,
			Kinds:  1,
		})
	}
	sort.Slice(votes, func(left, right int) bool { return votes[left].Origin < votes[right].Origin })
	return votes, nil
}

// Agree reports whether the votes agree within tolerance. It only compares
// votes for the same measurement kind expressed in the same canonical unit;
// comparing different dimensions is an error rather than a disagreement.
func Agree(votes []SourceVote, tolerance float64) (bool, error) {
	if tolerance < 0 {
		return false, fmt.Errorf("tolerance cannot be negative")
	}
	if len(votes) < 2 {
		return false, nil
	}
	kind := votes[0].Kind
	unit := votes[0].Unit
	if kind == "" || unit == "" {
		return false, fmt.Errorf("votes must carry a measurement kind and unit")
	}
	minimum, maximum := votes[0].Value, votes[0].Value
	for _, vote := range votes[1:] {
		if vote.Kind != kind || vote.Unit != unit {
			return false, fmt.Errorf("cannot compare %s in %s with %s in %s", kind, unit, vote.Kind, vote.Unit)
		}
		minimum = math.Min(minimum, vote.Value)
		maximum = math.Max(maximum, vote.Value)
	}
	return maximum-minimum <= tolerance, nil
}

// CanonicalUnit returns the unit independent votes for the given kind use.
func CanonicalUnit(kind ReadingKind) (string, error) {
	unit, ok := canonicalUnits[kind]
	if !ok {
		return "", fmt.Errorf("unknown reading kind %q", kind)
	}
	return unit, nil
}
