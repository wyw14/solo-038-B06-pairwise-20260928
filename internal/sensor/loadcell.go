package sensor

import (
	"fmt"
	"math"
	"time"
)

type ReadingKind string

const (
	ReadingForce   ReadingKind = "force"
	ReadingTension ReadingKind = "tension"
	ReadingSpeed   ReadingKind = "speed"
)

type Reading struct {
	ID         string      `json:"id"`
	Kind       ReadingKind `json:"kind"`
	Source     string      `json:"source"`
	Origin     string      `json:"origin"`
	Value      float64     `json:"value"`
	Unit       string      `json:"unit"`
	ObservedAt time.Time   `json:"observed_at"`
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
	return nil
}

// canonicalUnits maps each reading kind to its canonical unit and the
// multiplicative factors that convert a supported unit into it. Readings
// may only be compared or combined after conversion through this table;
// anything absent here is rejected so unknown dimensions fail closed.
var canonicalUnits = map[ReadingKind]struct {
	unit    string
	factors map[string]float64
}{
	ReadingForce:   {unit: "N", factors: map[string]float64{"N": 1, "kN": 1000}},
	ReadingTension: {unit: "N/mm", factors: map[string]float64{"N/mm": 1, "N/cm": 0.1, "N/m": 0.001, "kN/m": 1}},
	ReadingSpeed:   {unit: "m/s", factors: map[string]float64{"m/s": 1, "mm/s": 0.001, "m/min": 1.0 / 60}},
}

// CanonicalValue returns the reading's value expressed in the canonical
// unit for its kind, along with that unit. It is the only sanctioned way
// to put two readings of the same kind on a comparable footing.
func (reading Reading) CanonicalValue() (float64, string, error) {
	kind, ok := canonicalUnits[reading.Kind]
	if !ok {
		return 0, "", fmt.Errorf("reading kind %q has no canonical unit", reading.Kind)
	}
	factor, ok := kind.factors[reading.Unit]
	if !ok {
		return 0, "", fmt.Errorf("unit %q cannot be converted to %s", reading.Unit, kind.unit)
	}
	return reading.Value * factor, kind.unit, nil
}

func DeriveTension(force Reading, webWidth float64, id string) (Reading, error) {
	if err := force.Validate(); err != nil {
		return Reading{}, err
	}
	if force.Kind != ReadingForce || webWidth <= 0 {
		return Reading{}, fmt.Errorf("force reading and positive web width are required")
	}
	newtons, _, err := force.CanonicalValue()
	if err != nil {
		return Reading{}, err
	}
	// A derived reading keeps the parent's origin on purpose: it is not an
	// independent source, so it must never count as a separate vote origin.
	return Reading{ID: id, Kind: ReadingTension, Source: "derived:" + force.Source, Origin: force.Origin, Value: newtons / webWidth, Unit: "N/mm", ObservedAt: force.ObservedAt}, nil
}
