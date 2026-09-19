package main

import (
	"math/rand"
	"testing"
)

func TestHandTotals(t *testing.T) {
	cases := []struct {
		h    Hand
		want int
	}{
		{Hand{{"A", "S"}, {"K", "H"}}, 21},
		{Hand{{"A", "S"}, {"A", "H"}}, 12},
		{Hand{{"A", "S"}, {"9", "H"}, {"5", "D"}}, 15},
		{Hand{{"10", "S"}, {"9", "H"}, {"5", "D"}}, 24},
		{Hand{{"A", "S"}, {"A", "H"}, {"9", "D"}}, 21},
		{Hand{{"K", "S"}, {"Q", "H"}}, 20},
	}
	for _, c := range cases {
		if got := c.h.Total(); got != c.want {
			t.Errorf("%s: got %d want %d", c.h, got, c.want)
		}
	}
}

func TestHandSoft(t *testing.T) {
	if !(Hand{{"A", "S"}, {"6", "H"}}).Soft() {
		t.Error("A+6 should be soft")
	}
	if (Hand{{"A", "S"}, {"6", "H"}, {"K", "D"}}).Soft() {
		t.Error("A+6+K should be hard 17")
	}
	if (Hand{{"10", "S"}, {"7", "H"}}).Soft() {
		t.Error("10+7 should not be soft")
	}
}

func TestShoePenetration(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := NewShoe(6, 0.75, rng)
	if s.Remaining() != 312 {
		t.Fatalf("expected 312 cards, got %d", s.Remaining())
	}
	for i := 0; i < 234; i++ {
		s.Draw()
	}
	if !s.NeedsShuffle() {
		t.Fatal("expected shuffle needed at 75% penetration")
	}
	s.Shuffle()
	if s.Remaining() != 312 {
		t.Fatalf("after shuffle expected 312 cards, got %d", s.Remaining())
	}
	if s.NeedsShuffle() {
		t.Fatal("fresh shoe should not need a shuffle")
	}
	if s.RunningCount() != 0 {
		t.Fatal("running count should reset after shuffle")
	}
}

func TestShoeComposition(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	s := NewShoe(1, 0.75, rng)
	comp := s.Composition()
	total := 0
	for _, n := range comp {
		total += n
	}
	if total != 52 {
		t.Fatalf("composition should sum to 52, got %d", total)
	}
	s.Draw()
	comp = s.Composition()
	total = 0
	for _, n := range comp {
		total += n
	}
	if total != 51 {
		t.Fatalf("after one draw composition should sum to 51, got %d", total)
	}
}
