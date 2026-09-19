package main

import "testing"

func baseRules() Rules {
	return Rules{
		Decks:            6,
		Penetration:      0.75,
		BlackjackPays:    1.5,
		DealerPeeks:      true,
		DoubleAllowed:    true,
		DoubleAnyTwo:     true,
		DoubleAfterSplit: true,
		MaxHands:         4,
		SplitAcesOneCard: true,
	}
}

func TestBasicStrategy(t *testing.T) {
	r := baseRules()
	r17 := baseRules()
	r17.DealerHitsSoft17 = true

	cases := []struct {
		name      string
		rules     Rules
		hand      Hand
		up        Card
		canDouble bool
		canSplit  bool
		want      Action
	}{
		{"hard 16 vs 10", r, Hand{{"10", "S"}, {"6", "H"}}, Card{"10", "D"}, true, true, ActionHit},
		{"hard 12 vs 4", r, Hand{{"10", "S"}, {"2", "H"}}, Card{"4", "D"}, true, true, ActionStand},
		{"hard 11 vs 6", r, Hand{{"5", "S"}, {"6", "H"}}, Card{"6", "D"}, true, true, ActionDouble},
		{"hard 11 vs 6 no double", r, Hand{{"5", "S"}, {"6", "H"}}, Card{"6", "D"}, false, true, ActionHit},
		{"hard 11 vs A S17", r, Hand{{"5", "S"}, {"6", "H"}}, Card{"A", "D"}, true, true, ActionHit},
		{"hard 11 vs A H17", r17, Hand{{"5", "S"}, {"6", "H"}}, Card{"A", "D"}, true, true, ActionDouble},
		{"soft 18 vs 3", r, Hand{{"A", "S"}, {"7", "H"}}, Card{"3", "D"}, true, true, ActionDouble},
		{"soft 18 vs 2 S17", r, Hand{{"A", "S"}, {"7", "H"}}, Card{"2", "D"}, true, true, ActionStand},
		{"soft 18 vs 2 H17", r17, Hand{{"A", "S"}, {"7", "H"}}, Card{"2", "D"}, true, true, ActionDouble},
		{"soft 18 vs 9", r, Hand{{"A", "S"}, {"7", "H"}}, Card{"9", "D"}, true, true, ActionHit},
		{"soft 17 vs 2", r, Hand{{"A", "S"}, {"6", "H"}}, Card{"2", "D"}, true, true, ActionHit},
		{"soft 19 vs 6 S17", r, Hand{{"A", "S"}, {"8", "H"}}, Card{"6", "D"}, true, true, ActionStand},
		{"pair 8s vs 10", r, Hand{{"8", "S"}, {"8", "H"}}, Card{"10", "D"}, true, true, ActionSplit},
		{"pair 8s no split", r, Hand{{"8", "S"}, {"8", "H"}}, Card{"10", "D"}, true, false, ActionHit},
		{"pair 10s vs 6", r, Hand{{"10", "S"}, {"10", "H"}}, Card{"6", "D"}, true, true, ActionStand},
		{"pair 9s vs 7", r, Hand{{"9", "S"}, {"9", "H"}}, Card{"7", "D"}, true, true, ActionStand},
		{"pair 9s vs 9", r, Hand{{"9", "S"}, {"9", "H"}}, Card{"9", "D"}, true, true, ActionSplit},
		{"pair aces", r, Hand{{"A", "S"}, {"A", "H"}}, Card{"6", "D"}, true, true, ActionSplit},
		{"pair 5s vs 6", r, Hand{{"5", "S"}, {"5", "H"}}, Card{"6", "D"}, true, true, ActionDouble},
		{"hard 17", r, Hand{{"10", "S"}, {"7", "H"}}, Card{"9", "D"}, true, true, ActionStand},
	}

	for _, c := range cases {
		if got := BasicStrategy(c.rules, c.hand, c.up, c.canDouble, c.canSplit); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestParseBlackjackPayout(t *testing.T) {
	if v, err := ParseBlackjackPayout("3:2"); err != nil || v != 1.5 {
		t.Errorf("3:2 -> %v %v", v, err)
	}
	if v, err := ParseBlackjackPayout("6:5"); err != nil || v != 1.2 {
		t.Errorf("6:5 -> %v %v", v, err)
	}
	if _, err := ParseBlackjackPayout("2:1"); err == nil {
		t.Error("expected error for 2:1")
	}
}

func TestRulesValidate(t *testing.T) {
	r := baseRules()
	if err := r.Validate(); err != nil {
		t.Fatalf("base rules should validate: %v", err)
	}
	bad := baseRules()
	bad.Decks = 0
	if err := bad.Validate(); err == nil {
		t.Error("expected error for 0 decks")
	}
}
