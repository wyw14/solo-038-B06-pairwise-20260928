package sensor

import "fmt"

// canonicalUnits is the single unit per measurement kind that votes are
// compared in, so readings supplied in compatible units never disagree just
// because of scale.
var canonicalUnits = map[ReadingKind]string{
	ReadingForce:   "N",
	ReadingTension: "N/mm",
	ReadingSpeed:   "m/s",
}

// unitFactors maps a supplied unit to the factor that converts a value in that
// unit into the kind's canonical unit. Only units that share the kind's
// dimension are present: a force reading may never be expressed in N/mm.
var unitFactors = map[ReadingKind]map[string]float64{
	ReadingForce: {
		"N":   1,
		"kN":  1000,
		"lbf": 4.4482216152605,
	},
	ReadingTension: {
		"N/mm": 1,
		"N/cm": 0.1,
		"N/m":  0.001,
	},
	ReadingSpeed: {
		"m/s":  1,
		"mm/s": 0.001,
	},
}

// canonicalize converts the reading into the canonical unit of its own
// measurement kind. It rejects unknown kinds and units that do not belong to
// the reading's dimension.
func canonicalize(reading Reading) (float64, string, error) {
	factors, ok := unitFactors[reading.Kind]
	if !ok {
		return 0, "", fmt.Errorf("unknown reading kind %q", reading.Kind)
	}
	factor, ok := factors[reading.Unit]
	if !ok {
		return 0, "", fmt.Errorf("unit %q is not a %s measurement", reading.Unit, reading.Kind)
	}
	return reading.Value * factor, canonicalUnits[reading.Kind], nil
}
