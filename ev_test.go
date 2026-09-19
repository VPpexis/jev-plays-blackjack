package main

import (
	"context"
	"math/rand"
	"testing"
)

func fullShoeCards() []Card {
	deck := make([]Card, 0, 6*52)
	for d := 0; d < 6; d++ {
		for _, s := range suits {
			for _, r := range ranks {
				deck = append(deck, Card{Rank: r, Suit: s})
			}
		}
	}
	return deck
}

func TestEVSanity(t *testing.T) {
	est := &evEstimator{rules: baseRules(), rng: rand.New(rand.NewSource(1)), rollouts: 4000}
	deck := fullShoeCards()

	stand20 := est.estimate(Hand{{"10", "S"}, {"10", "H"}}, Card{"6", "D"}, deck, ActionStand)
	hit20 := est.estimate(Hand{{"10", "S"}, {"10", "H"}}, Card{"6", "D"}, deck, ActionHit)
	if stand20 < 0.5 {
		t.Errorf("standing on 20 vs 6 should be strongly positive, got %v", stand20)
	}
	if stand20 <= hit20 {
		t.Errorf("standing on 20 (%v) should beat hitting (%v)", stand20, hit20)
	}

	stand16 := est.estimate(Hand{{"10", "S"}, {"6", "H"}}, Card{"10", "D"}, deck, ActionStand)
	if stand16 >= 0 {
		t.Errorf("standing on 16 vs 10 should be negative, got %v", stand16)
	}

	double11 := est.estimate(Hand{{"5", "S"}, {"6", "H"}}, Card{"6", "D"}, deck, ActionDouble)
	hit11 := est.estimate(Hand{{"5", "S"}, {"6", "H"}}, Card{"6", "D"}, deck, ActionHit)
	if double11 <= hit11 {
		t.Errorf("doubling 11 vs 6 (%v) should beat hitting (%v)", double11, hit11)
	}

	surrender := est.estimate(Hand{{"10", "S"}, {"6", "H"}}, Card{"10", "D"}, deck, ActionSurrender)
	if surrender != -0.5 {
		t.Errorf("surrender EV should be exactly -0.5, got %v", surrender)
	}
}

func TestEVLossDirection(t *testing.T) {
	est := &evEstimator{rules: baseRules(), rng: rand.New(rand.NewSource(2)), rollouts: 3000}
	deck := fullShoeCards()

	basicEV := est.estimate(Hand{{"10", "S"}, {"6", "H"}}, Card{"2", "D"}, deck, ActionStand)
	badEV := est.estimate(Hand{{"10", "S"}, {"6", "H"}}, Card{"2", "D"}, deck, ActionHit)
	if badEV >= basicEV {
		t.Errorf("hitting 16 vs 2 (%v) should be worse than standing (%v)", badEV, basicEV)
	}
}

func TestSimulateSplit(t *testing.T) {
	est := &evEstimator{rules: baseRules(), rng: rand.New(rand.NewSource(3)), rollouts: 500}
	deck := fullShoeCards()
	ev := est.estimate(Hand{{"8", "S"}, {"8", "H"}}, Card{"6", "D"}, deck, ActionSplit)
	if ev < 0 || ev > 2 {
		t.Errorf("splitting 8s vs 6 should have a sane positive EV, got %v", ev)
	}
}

type alwaysHitDecider struct{}

func (d *alwaysHitDecider) ChooseBet(ctx context.Context, req BetRequest) BetResponse {
	return BetResponse{Option: "flat", Flat: 10}
}

func (d *alwaysHitDecider) ChooseAction(ctx context.Context, req ActionRequest) ActionResponse {
	return ActionResponse{Action: ActionHit}
}

func (d *alwaysHitDecider) ChooseInsurance(ctx context.Context, req InsuranceRequest) InsuranceResponse {
	return InsuranceResponse{}
}

func TestEVLossRecordedForDeviations(t *testing.T) {
	rules := baseRules()
	game := NewGame(rules, 100000, 10, 1, &alwaysHitDecider{}, rand.New(rand.NewSource(4)), false)
	game.EnableEV(300, rand.New(rand.NewSource(5)))

	checked := 0
	totalLoss := 0.0
	for i := 0; i < 60; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		for _, d := range rec.Decisions {
			if d.EVChecked {
				checked++
				totalLoss += d.EVLoss
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected EV-checked deviations")
	}
	if totalLoss <= 0 {
		t.Fatalf("always hitting should lose EV against basic strategy, got %v", totalLoss)
	}
}
