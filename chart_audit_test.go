package main

import (
	"fmt"
	"math/rand"
	"testing"
)

func hardHand(total int) Hand {
	if total <= 10 {
		return Hand{{"2", "S"}, {fmt.Sprintf("%d", total-2), "H"}}
	}
	return Hand{{"10", "S"}, {fmt.Sprintf("%d", total-10), "H"}}
}

func TestChartAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("audit is slow")
	}
	rules := baseRules()
	est := &evEstimator{rules: rules, rng: rand.New(rand.NewSource(99)), rollouts: 4000}
	deck := fullShoeCards()

	ups := []Card{{"2", "S"}, {"3", "S"}, {"4", "S"}, {"5", "S"}, {"6", "S"}, {"7", "S"}, {"8", "S"}, {"9", "S"}, {"10", "S"}, {"A", "S"}}

	var hands []Hand
	for total := 5; total <= 17; total++ {
		hands = append(hands, hardHand(total))
	}
	for _, r := range []string{"2", "3", "4", "5", "6", "7", "8", "9"} {
		hands = append(hands, Hand{{"A", "S"}, {r, "H"}})
	}
	for _, r := range ranks {
		hands = append(hands, Hand{{r, "S"}, {r, "H"}})
	}

	worst := 0.0
	bad := 0
	for _, hand := range hands {
		for _, up := range ups {
			canDouble := len(hand) == 2 && rules.DoubleAnyTwo
			canSplit := len(hand) == 2 && hand.Pair()
			chart := BasicStrategy(rules, hand, up, canDouble, canSplit)

			legal := []Action{ActionHit, ActionStand}
			if canDouble {
				legal = append(legal, ActionDouble)
			}
			if canSplit {
				legal = append(legal, ActionSplit)
			}

			bestEV := -99.0
			bestAction := Action("")
			chartEV := 0.0
			for _, a := range legal {
				ev := est.estimate(hand, up, deck, a)
				if ev > bestEV {
					bestEV = ev
					bestAction = a
				}
				if a == chart {
					chartEV = ev
				}
			}
			diff := bestEV - chartEV
			if diff > 0.03 {
				bad++
				if diff > worst {
					worst = diff
				}
				t.Logf("chart=%s best=%s diff=%.4f  hand=%s up=%s (chartEV=%.4f bestEV=%.4f)",
					chart, bestAction, diff, hand, up, chartEV, bestEV)
			}
		}
	}
	t.Logf("cells worse than best by >0.03: %d, worst diff %.4f", bad, worst)
}
