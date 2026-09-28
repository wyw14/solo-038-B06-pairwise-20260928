package sensor

import (
	"testing"
	"time"
)

func reading(id string, kind ReadingKind, origin string, value float64, unit string) Reading {
	return Reading{ID: id, Kind: kind, Source: origin, Origin: origin, Value: value, Unit: unit, ObservedAt: time.Now()}
}

func voteFor(t *testing.T, votes []SourceVote, kind ReadingKind, origin string) SourceVote {
	t.Helper()
	for _, vote := range votes {
		if vote.Kind == kind && vote.Origin == origin {
			return vote
		}
	}
	t.Fatalf("no vote for kind %q origin %q in %v", kind, origin, votes)
	return SourceVote{}
}

// The reported defect: 90 N and 49.5 N plainly disagree, yet folding the
// 9 N/mm tension derived from the 90 N cell into the same origin bucket
// averaged the vote down to 49.5 and the permit passed.
func TestIndependentVotesDoNotMixKindsOrDerivedValues(t *testing.T) {
	forceA := reading("f-a", ReadingForce, "load-cell-a", 90, "N")
	forceB := reading("f-b", ReadingForce, "load-cell-b", 49.5, "N")
	tension, err := DeriveTension(forceA, 10, "t-a")
	if err != nil {
		t.Fatal(err)
	}
	votes, err := IndependentVotes([]Reading{forceA, forceB, tension})
	if err != nil {
		t.Fatal(err)
	}
	if got := voteFor(t, votes, ReadingForce, "load-cell-a").Value; got != 90 {
		t.Fatalf("derived tension diluted force vote: got %v, want 90", got)
	}
	if got := voteFor(t, votes, ReadingForce, "load-cell-b").Value; got != 49.5 {
		t.Fatalf("force vote changed: got %v, want 49.5", got)
	}
	derived := voteFor(t, votes, ReadingTension, "load-cell-a")
	if derived.Value != 9 || derived.Unit != "N/mm" {
		t.Fatalf("derived tension vote = %v %q, want 9 N/mm", derived.Value, derived.Unit)
	}
	agree, err := Agree(votes, 2)
	if err != nil {
		t.Fatal(err)
	}
	if agree {
		t.Fatal("90 N and 49.5 N must not be judged in agreement")
	}
}

func TestGenuineDualSourceAgreementStillPasses(t *testing.T) {
	votes, err := IndependentVotes([]Reading{
		reading("f-a", ReadingForce, "load-cell-a", 90, "N"),
		reading("f-b", ReadingForce, "load-cell-b", 91, "N"),
	})
	if err != nil {
		t.Fatal(err)
	}
	agree, err := Agree(votes, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !agree {
		t.Fatal("90 N and 91 N should agree within tolerance 2")
	}
}

func TestConvertibleUnitsAreNormalizedBeforeVoting(t *testing.T) {
	votes, err := IndependentVotes([]Reading{
		reading("f-a", ReadingForce, "load-cell-a", 0.09, "kN"),
		reading("f-b", ReadingForce, "load-cell-b", 90, "N"),
	})
	if err != nil {
		t.Fatal(err)
	}
	agree, err := Agree(votes, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !agree {
		t.Fatal("0.09 kN and 90 N are the same force and must agree")
	}
}

func TestUnknownUnitFailsClosed(t *testing.T) {
	if _, err := IndependentVotes([]Reading{reading("f-a", ReadingForce, "load-cell-a", 90, "lbf")}); err == nil {
		t.Fatal("an unconvertible unit must be rejected, not silently compared")
	}
	if _, err := IndependentVotes([]Reading{reading("x", "vibration", "sensor-a", 1, "mm/s")}); err == nil {
		t.Fatal("an unknown reading kind must be rejected")
	}
}

func TestDerivedReadingCannotManufactureASecondOrigin(t *testing.T) {
	forceA := reading("f-a", ReadingForce, "load-cell-a", 90, "N")
	tension, err := DeriveTension(forceA, 10, "t-a")
	if err != nil {
		t.Fatal(err)
	}
	votes, err := IndependentVotes([]Reading{forceA, tension})
	if err != nil {
		t.Fatal(err)
	}
	agree, err := Agree(votes, 2)
	if err != nil {
		t.Fatal(err)
	}
	if agree {
		t.Fatal("one origin plus its own derived value is not dual-source agreement")
	}
}

func TestConflictingKindVetoesAgreement(t *testing.T) {
	votes, err := IndependentVotes([]Reading{
		reading("f-a", ReadingForce, "load-cell-a", 90, "N"),
		reading("f-b", ReadingForce, "load-cell-b", 91, "N"),
		reading("t-a", ReadingTension, "tension-cell-a", 9, "N/mm"),
		reading("t-b", ReadingTension, "tension-cell-b", 4, "N/mm"),
	})
	if err != nil {
		t.Fatal(err)
	}
	agree, err := Agree(votes, 2)
	if err != nil {
		t.Fatal(err)
	}
	if agree {
		t.Fatal("agreeing force must not override disagreeing tension")
	}
}

func TestDerivedTensionAgreesWithDirectTensionAcrossOrigins(t *testing.T) {
	forceA := reading("f-a", ReadingForce, "load-cell-a", 90, "N")
	derived, err := DeriveTension(forceA, 10, "t-derived")
	if err != nil {
		t.Fatal(err)
	}
	direct := reading("t-b", ReadingTension, "tension-cell-b", 9.1, "N/mm")
	votes, err := IndependentVotes([]Reading{derived, direct})
	if err != nil {
		t.Fatal(err)
	}
	agree, err := Agree(votes, 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if !agree {
		t.Fatal("derived tension and an independent direct tension should agree")
	}
}

func TestDeriveTensionNormalizesForceUnits(t *testing.T) {
	force := reading("f-a", ReadingForce, "load-cell-a", 0.09, "kN")
	tension, err := DeriveTension(force, 10, "t-a")
	if err != nil {
		t.Fatal(err)
	}
	if tension.Value != 9 || tension.Unit != "N/mm" {
		t.Fatalf("0.09 kN over 10 mm should derive 9 N/mm, got %v %q", tension.Value, tension.Unit)
	}
	if tension.Origin != force.Origin {
		t.Fatalf("derived reading must keep parent origin %q, got %q", force.Origin, tension.Origin)
	}
}
