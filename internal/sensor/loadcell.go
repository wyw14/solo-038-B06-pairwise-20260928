package sensor

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type ReadingKind string

const (
	ReadingForce   ReadingKind = "force"
	ReadingTension ReadingKind = "tension"
	ReadingSpeed   ReadingKind = "speed"
)

// DerivedSourcePrefix marks a reading that was computed from another reading
// rather than observed directly by a sensor.
const DerivedSourcePrefix = "derived:"

type Reading struct {
	ID     string      `json:"id"`
	Kind   ReadingKind `json:"kind"`
	Source string      `json:"source"`
	Origin string      `json:"origin"`
	// DerivedFrom identifies the source reading this value was computed from.
	// It is empty for directly observed readings. A derived reading shares its
	// ancestor's origin and therefore is never independent evidence for it.
	DerivedFrom string    `json:"derived_from,omitempty"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	ObservedAt  time.Time `json:"observed_at"`
}

func (reading Reading) IsDerived() bool {
	return reading.DerivedFrom != "" || strings.HasPrefix(reading.Source, DerivedSourcePrefix)
}

func (reading Reading) Validate() error {
	if reading.ID == "" || reading.Source == "" || reading.Origin == "" {
		return fmt.Errorf("reading identity is incomplete")
	}
	if math.IsNaN(reading.Value) || math.IsInf(reading.Value, 0) {
		return fmt.Errorf("reading value is invalid")
	}
	if reading.ObservedAt.IsZero() {
		return fmt.Errorf("reading timestamp is required")
	}
	if _, _, err := canonicalize(reading); err != nil {
		return err
	}
	return nil
}

func DeriveTension(force Reading, webWidth float64, id string) (Reading, error) {
	if err := force.Validate(); err != nil {
		return Reading{}, err
	}
	if force.Kind != ReadingForce || webWidth <= 0 {
		return Reading{}, fmt.Errorf("force reading and positive web width are required")
	}
	// Force is canonicalised to newtons so the derived tension is always
	// produced in N/mm regardless of the force reading's original unit.
	forceN, _, err := canonicalize(force)
	if err != nil {
		return Reading{}, err
	}
	return Reading{
		ID:          id,
		Kind:        ReadingTension,
		Source:      DerivedSourcePrefix + force.Source,
		Origin:      force.Origin,
		DerivedFrom: force.ID,
		Value:       forceN / webWidth,
		Unit:        canonicalUnits[ReadingTension],
		ObservedAt:  force.ObservedAt,
	}, nil
}
