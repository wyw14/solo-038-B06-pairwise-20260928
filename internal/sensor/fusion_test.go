package sensor

import (
	"testing"
	"time"
)

func readingAt() time.Time { return time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC) }

func force(id, origin string, value float64, unit string) Reading {
	return Reading{ID: id, Kind: ReadingForce, Source: origin, Origin: origin, Value: value, Unit: unit, ObservedAt: readingAt()}
}

// TestDisagreeingForceSensorsWithDerivedTension reproduces the reported
// incident: two load cells reporting 90 N and 49.5 N clearly disagree, yet a
// 9 N/mm tension derived from the first was previously averaged into its
// origin's force vote ((90+9)/2 == 49.5), making the bogus ballot pass.
func TestDisagreeingForceSensorsWithDerivedTension(t *testing.T) {
	forceA := force("reading-a", "load-cell-a", 90, "N")
	forceB := force("reading-b", "load-cell-b", 49.5, "N")
	derived, err := DeriveTension(forceA, 10, "tension-a")
	if err != nil {
		t.Fatalf("derive tension: %v", err)
	}

	all := []Reading{forceA, forceB, derived}

	votes, err := IndependentVotes(all, ReadingForce)
	if err != nil {
		t.Fatalf("independent votes: %v", err)
	}
	// The derived tension is a different dimension and must not cast a force
	// vote, leaving exactly the two raw load cells, each unmodified.
	if len(votes) != 2 {
		t.Fatalf("expected 2 force votes, got %d: %+v", len(votes), votes)
	}
	byOrigin := map[string]SourceVote{}
	for _, vote := range votes {
		byOrigin[vote.Origin] = vote
		if vote.Unit != "N" {
			t.Errorf("force vote %s in unit %q, want N", vote.Origin, vote.Unit)
		}
	}
	if byOrigin["load-cell-a"].Value != 90 {
		t.Errorf("load-cell-a vote = %v, want 90 (must not blend in 9 N/mm)", byOrigin["load-cell-a"].Value)
	}
	if byOrigin["load-cell-b"].Value != 49.5 {
		t.Errorf("load-cell-b vote = %v, want 49.5", byOrigin["load-cell-b"].Value)
	}
	agree, err := Agree(votes, 2)
	if err != nil {
		t.Fatalf("agree: %v", err)
	}
	if agree {
		t.Fatalf("90 N vs 49.5 N must not agree within 2 N")
	}

	// The derived tension from a single sensor cannot satisfy a tension vote
	// on its own: it shares load-cell-a's root origin, so there is only one
	// independent tension source.
	tensionVotes, err := IndependentVotes(all, ReadingTension)
	if err != nil {
		t.Fatalf("tension votes: %v", err)
	}
	if len(tensionVotes) != 1 {
		t.Fatalf("expected 1 tension origin (derived from a only), got %d", len(tensionVotes))
	}
	agree, err = Agree(tensionVotes, 0.25)
	if err != nil {
		t.Fatalf("agree tension: %v", err)
	}
	if agree {
		t.Fatalf("a single derived value cannot corroborate its own origin")
	}
}

// TestTwoIndependentDerivedTensionsAgree preserves the legitimate path:
// tensions derived independently from two agreeing load cells are two
// genuinely independent sources (different root origins) and must agree.
func TestTwoIndependentDerivedTensionsAgree(t *testing.T) {
	forceA := force("reading-a", "load-cell-a", 90, "N")
	forceB := force("reading-b", "load-cell-b", 91, "N")
	tensionA, err := DeriveTension(forceA, 10, "tension-a")
	if err != nil {
		t.Fatalf("derive tension a: %v", err)
	}
	tensionB, err := DeriveTension(forceB, 10, "tension-b")
	if err != nil {
		t.Fatalf("derive tension b: %v", err)
	}
	if !tensionB.IsDerived() || tensionB.DerivedFrom != "reading-b" {
		t.Fatalf("derived tension lost provenance: %+v", tensionB)
	}

	votes, err := IndependentVotes([]Reading{tensionA, tensionB}, ReadingTension)
	if err != nil {
		t.Fatalf("independent votes: %v", err)
	}
	if len(votes) != 2 {
		t.Fatalf("expected 2 independent tension origins, got %d", len(votes))
	}
	agree, err := Agree(votes, 0.25)
	if err != nil {
		t.Fatalf("agree: %v", err)
	}
	if !agree {
		t.Fatalf("9.0 N/mm vs 9.1 N/mm should agree within 0.25 N/mm")
	}
}

// TestUnitConversionAcrossCompatibleUnits ensures readings in compatible
// units of the same dimension are normalised before comparison.
func TestUnitConversionAcrossCompatibleUnits(t *testing.T) {
	inNewtons := force("reading-a", "load-cell-a", 90, "N")
	inKilonewtons := force("reading-b", "load-cell-b", 0.09, "kN")
	votes, err := IndependentVotes([]Reading{inNewtons, inKilonewtons}, ReadingForce)
	if err != nil {
		t.Fatalf("independent votes: %v", err)
	}
	if len(votes) != 2 {
		t.Fatalf("expected 2 votes, got %d", len(votes))
	}
	if votes[0].Value != votes[1].Value {
		t.Fatalf("90 N and 0.09 kN should canonicalise to the same value, got %v and %v", votes[0].Value, votes[1].Value)
	}
	agree, err := Agree(votes, 0.001)
	if err != nil {
		t.Fatalf("agree: %v", err)
	}
	if !agree {
		t.Fatal("converted readings should agree")
	}
}

// TestDimensionMismatchRejected ensures a value whose unit belongs to another
// dimension is invalid rather than silently compared.
func TestDimensionMismatchRejected(t *testing.T) {
	bogus := force("reading-a", "load-cell-a", 9, "N/mm")
	if err := bogus.Validate(); err == nil {
		t.Fatal("a force reading expressed in N/mm must fail validation")
	}
	if _, _, err := canonicalize(bogus); err == nil {
		t.Fatal("canonicalize must reject N/mm for a force reading")
	}
}

// TestDerivedTensionNormalisesSourceUnit checks the derived tension is computed
// from force in newtons even when the source force used another unit.
func TestDerivedTensionNormalisesSourceUnit(t *testing.T) {
	forceInKN := force("reading-a", "load-cell-a", 0.09, "kN")
	tension, err := DeriveTension(forceInKN, 10, "tension-a")
	if err != nil {
		t.Fatalf("derive tension: %v", err)
	}
	if tension.Unit != "N/mm" || tension.Value != 9 {
		t.Fatalf("derived tension = %v %q, want 9 N/mm", tension.Value, tension.Unit)
	}
}

func TestAgreeRejectsMixedDimensions(t *testing.T) {
	votes := []SourceVote{
		{Origin: "a", Kind: "force", Value: 9, Unit: "N"},
		{Origin: "b", Kind: "tension", Value: 9, Unit: "N/mm"},
	}
	if _, err := Agree(votes, 1); err == nil {
		t.Fatal("Agree must refuse to compare N with N/mm")
	}
}

func TestUnknownKindRejected(t *testing.T) {
	now := readingAt()
	bogus := Reading{ID: "x", Kind: ReadingKind("pressure"), Source: "s", Origin: "s", Value: 1, Unit: "Pa", ObservedAt: now}
	if err := bogus.Validate(); err == nil {
		t.Fatal("unknown reading kind should fail validation")
	}
	if _, err := IndependentVotes([]Reading{bogus}, bogus.Kind); err == nil {
		t.Fatal("IndependentVotes should reject unknown kind")
	}
}
