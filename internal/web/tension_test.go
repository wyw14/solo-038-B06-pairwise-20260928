package web

import (
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/sensor"
)

func tensionPermitReading(id, origin string, value float64, unit string, observed time.Time) sensor.Reading {
	return sensor.Reading{ID: id, Kind: sensor.ReadingTension, Source: origin, Origin: origin, Value: value, Unit: unit, ObservedAt: observed}
}

// TestPermitRejectsDisagreementDespiteDerivedValue is the end-to-end version
// of the reported defect: the derived 9 N/mm tension must not let a ballot
// containing disagreeing force sensors pass.
func TestPermitRejectsDisagreementDespiteDerivedValue(t *testing.T) {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	forceA := sensor.Reading{ID: "fa", Kind: sensor.ReadingForce, Source: "load-cell-a", Origin: "load-cell-a", Value: 90, Unit: "N", ObservedAt: now}
	forceB := sensor.Reading{ID: "fb", Kind: sensor.ReadingForce, Source: "load-cell-b", Origin: "load-cell-b", Value: 49.5, Unit: "N", ObservedAt: now}
	derived, err := sensor.DeriveTension(forceA, 10, "ta")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	// Force ballot: clearly disagreeing forces, the derived tension is a
	// different dimension and cannot vote.
	permit, err := BuildTensionPermit([]sensor.Reading{forceA, forceB, derived}, sensor.ReadingForce, 2, now)
	if err != nil {
		t.Fatalf("build permit: %v", err)
	}
	if permit.Allowed {
		t.Fatalf("permit must not pass for 90 N vs 49.5 N: %+v", permit)
	}
	if permit.Unit != "N" {
		t.Fatalf("permit unit = %q, want N", permit.Unit)
	}
}

func TestPermitAllowsTwoAgreeingIndependentOrigins(t *testing.T) {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	a := tensionPermitReading("ta", "load-cell-a", 9.0, "N/mm", now)
	// 90.5 N/cm converts to 9.05 N/mm: a second independent origin in a
	// different (compatible) unit of the same dimension.
	b := tensionPermitReading("tb", "load-cell-b", 90.5, "N/cm", now)
	permit, err := BuildTensionPermit([]sensor.Reading{a, b}, sensor.ReadingTension, 0.25, now)
	if err != nil {
		t.Fatalf("build permit: %v", err)
	}
	if !permit.Allowed {
		t.Fatalf("two independent tensions 9.0 and 9.05 N/mm should agree: %s", permit.Reason)
	}
	if permit.Unit != "N/mm" {
		t.Fatalf("permit unit = %q, want N/mm", permit.Unit)
	}
}

func TestPermitRejectsSingleDerivedOrigin(t *testing.T) {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	forceA := sensor.Reading{ID: "fa", Kind: sensor.ReadingForce, Source: "load-cell-a", Origin: "load-cell-a", Value: 90, Unit: "N", ObservedAt: now}
	derived, err := sensor.DeriveTension(forceA, 10, "ta")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	permit, err := BuildTensionPermit([]sensor.Reading{forceA, derived}, sensor.ReadingTension, 0.25, now)
	if err != nil {
		t.Fatalf("build permit: %v", err)
	}
	if permit.Allowed {
		t.Fatal("one tension derived from a single force cannot prove two-source agreement")
	}
}
