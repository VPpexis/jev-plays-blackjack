package main

import (
	"context"
	"math"
	"math/rand"
	"testing"
)

type scriptDecider struct {
	bets    []string
	actions []Action
	bi      int
	ai      int
}

func (d *scriptDecider) ChooseBet(ctx context.Context, req BetRequest) BetResponse {
	if d.bi < len(d.bets) {
		o := d.bets[d.bi]
		d.bi++
		return BetResponse{Option: o}
	}
	return BetResponse{Option: "bet_5"}
}

func (d *scriptDecider) ChooseAction(ctx context.Context, req ActionRequest) ActionResponse {
	if d.ai < len(d.actions) {
		a := d.actions[d.ai]
		d.ai++
		if containsAction(req.Legal, a) {
			return ActionResponse{Action: a}
		}
	}
	return ActionResponse{Action: ActionStand}
}

func (d *scriptDecider) ChooseInsurance(ctx context.Context, req InsuranceRequest) InsuranceResponse {
	return InsuranceResponse{Take: false}
}

func TestDealerShouldHit(t *testing.T) {
	s17 := baseRules()
	h17 := baseRules()
	h17.DealerHitsSoft17 = true

	if !dealerShouldHit(Hand{{"10", "S"}, {"6", "H"}}, s17) {
		t.Error("dealer should hit 16")
	}
	if dealerShouldHit(Hand{{"10", "S"}, {"7", "H"}}, s17) {
		t.Error("dealer should stand on hard 17")
	}
	if dealerShouldHit(Hand{{"A", "S"}, {"6", "H"}}, s17) {
		t.Error("dealer should stand on soft 17 when S17")
	}
	if !dealerShouldHit(Hand{{"A", "S"}, {"6", "H"}}, h17) {
		t.Error("dealer should hit soft 17 when H17")
	}
}

func TestResolveHandPayouts(t *testing.T) {
	r := baseRules()
	dealer18 := Hand{{"10", "S"}, {"8", "H"}}
	dealerBJ := Hand{{"A", "S"}, {"K", "H"}}

	bj := &playerHand{cards: Hand{{"A", "S"}, {"K", "H"}}, bet: 10, wagered: 10}
	if got := resolveHand(bj, dealer18, r); got != 25 {
		t.Errorf("blackjack should pay 25, got %v", got)
	}
	if got := resolveHand(bj, dealerBJ, r); got != 10 {
		t.Errorf("blackjack vs blackjack should push 10, got %v", got)
	}

	win := &playerHand{cards: Hand{{"10", "S"}, {"9", "H"}}, bet: 10, wagered: 10}
	if got := resolveHand(win, dealer18, r); got != 20 {
		t.Errorf("win should pay 20, got %v", got)
	}

	push := &playerHand{cards: Hand{{"10", "S"}, {"8", "H"}}, bet: 10, wagered: 10}
	if got := resolveHand(push, dealer18, r); got != 10 {
		t.Errorf("push should return 10, got %v", got)
	}

	bust := &playerHand{cards: Hand{{"10", "S"}, {"9", "H"}, {"5", "D"}}, bet: 10, wagered: 10}
	if got := resolveHand(bust, dealer18, r); got != 0 {
		t.Errorf("bust should pay 0, got %v", got)
	}

	surr := &playerHand{cards: Hand{{"10", "S"}, {"6", "H"}}, bet: 10, wagered: 10, surrendered: true}
	if got := resolveHand(surr, dealer18, r); got != 5 {
		t.Errorf("surrender should return 5, got %v", got)
	}

	dealerBust := Hand{{"10", "S"}, {"6", "H"}, {"K", "D"}}
	stand := &playerHand{cards: Hand{{"10", "S"}, {"7", "H"}}, bet: 10, wagered: 10}
	if got := resolveHand(stand, dealerBust, r); got != 20 {
		t.Errorf("dealer bust should pay 20, got %v", got)
	}
}

func TestBetAmount(t *testing.T) {
	g := &Game{bankroll: 100, unit: 1, minBet: 1}
	cases := map[string]float64{"bet_5": 5, "bet_10": 10, "bet_50": 50, "all_in": 100}
	for opt, want := range cases {
		if got := g.betAmount(opt); got != want {
			t.Errorf("%s: got %v want %v", opt, got, want)
		}
	}

	small := &Game{bankroll: 7, unit: 1, minBet: 1}
	if got := small.betAmount("bet_5"); got != 1 {
		t.Errorf("5%% of 7 floored to 0 should clamp to min bet 1, got %v", got)
	}
	odd := &Game{bankroll: 7, unit: 5, minBet: 5}
	if got := odd.betAmount("bet_10"); got != 5 {
		t.Errorf("10%% of 7 with unit 5 should clamp to 5, got %v", got)
	}
}

func TestLegalActions(t *testing.T) {
	r := baseRules()
	r.DoubleAfterSplit = false
	g := &Game{rules: r, bankroll: 100, unit: 1, minBet: 1}

	pair := &playerHand{cards: Hand{{"8", "S"}, {"8", "H"}}, bet: 10, wagered: 10}
	legal := g.legalActions(pair, []*playerHand{pair}, Hand{{"6", "S"}, {"5", "H"}})
	if !containsAction(legal, ActionSplit) {
		t.Error("pair of 8s should allow split")
	}
	if !containsAction(legal, ActionDouble) {
		t.Error("first two cards should allow double")
	}

	split := &playerHand{cards: Hand{{"5", "S"}, {"6", "H"}}, bet: 10, wagered: 10, fromSplit: true}
	legal = g.legalActions(split, []*playerHand{split}, Hand{{"6", "S"}, {"5", "H"}})
	if containsAction(legal, ActionDouble) {
		t.Error("no double after split when DAS is off")
	}

	multi := []*playerHand{pair, pair, pair, pair}
	legal = g.legalActions(pair, multi, Hand{{"6", "S"}, {"5", "H"}})
	if containsAction(legal, ActionSplit) {
		t.Error("should not split beyond max hands")
	}

	poor := &Game{rules: baseRules(), bankroll: 5, unit: 1, minBet: 1}
	legal = poor.legalActions(pair, []*playerHand{pair}, Hand{{"6", "S"}, {"5", "H"}})
	if containsAction(legal, ActionDouble) {
		t.Error("should not double without funds")
	}
	if containsAction(legal, ActionSplit) {
		t.Error("should not split without funds")
	}

	noDoubleRules := baseRules()
	noDoubleRules.DoubleAllowed = false
	noDouble := &Game{rules: noDoubleRules, bankroll: 100, unit: 1, minBet: 1}
	legal = noDouble.legalActions(pair, []*playerHand{pair}, Hand{{"6", "S"}, {"5", "H"}})
	if containsAction(legal, ActionDouble) {
		t.Error("doubling should be unavailable when DoubleAllowed is false")
	}

	nineEleven := baseRules()
	nineEleven.DoubleAnyTwo = false
	restricted := &Game{rules: nineEleven, bankroll: 100, unit: 1, minBet: 1}
	legal = restricted.legalActions(pair, []*playerHand{pair}, Hand{{"6", "S"}, {"5", "H"}})
	if containsAction(legal, ActionDouble) {
		t.Error("8 should not be doublable when only 9-11 is allowed")
	}
	eleven := &playerHand{cards: Hand{{"5", "S"}, {"6", "H"}}, bet: 10, wagered: 10}
	legal = restricted.legalActions(eleven, []*playerHand{eleven}, Hand{{"6", "S"}, {"5", "H"}})
	if !containsAction(legal, ActionDouble) {
		t.Error("11 should be doublable when only 9-11 is allowed")
	}
}

func TestRoundAccounting(t *testing.T) {
	rules := baseRules()
	dec := &scriptDecider{bets: []string{"bet_10"}}
	rng := rand.New(rand.NewSource(7))
	game := NewGame(rules, 100, 1, 1, dec, rng, false)

	var totalNet float64
	for i := 0; i < 40; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if got := rec.BankrollAfter - rec.BankrollBefore; math.Abs(got-rec.Net) > 1e-9 {
			t.Fatalf("round %d: net %v but bankroll delta %v", i, rec.Net, got)
		}
		var handNet float64
		for _, h := range rec.Hands {
			handNet += h.Net
		}
		if math.Abs(handNet-rec.Net) > 1e-9 {
			t.Fatalf("round %d: hand nets %v but round net %v", i, handNet, rec.Net)
		}
		if len(rec.Hands) == 0 {
			t.Fatalf("round %d: expected hands", i)
		}
		totalNet += rec.Net
	}

	if math.Abs((100+totalNet)-game.Bankroll()) > 1e-9 {
		t.Fatalf("bankroll %v does not match start + net %v", game.Bankroll(), 100+totalNet)
	}
}

func TestSplitForced(t *testing.T) {
	rules := baseRules()
	dec := &scriptDecider{
		bets:    []string{"bet_10"},
		actions: []Action{ActionSplit, ActionStand, ActionStand, ActionStand, ActionStand, ActionStand},
	}
	rng := rand.New(rand.NewSource(3))
	game := NewGame(rules, 1000, 1, 1, dec, rng, false)

	sawSplit := false
	for i := 0; i < 200 && !sawSplit; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		for _, h := range rec.Hands {
			if h.FromSplit {
				sawSplit = true
			}
		}
	}
	if !sawSplit {
		t.Skip("no split opportunity occurred in 200 rounds")
	}
}

func TestRuin(t *testing.T) {
	rules := baseRules()
	dec := &scriptDecider{bets: []string{"all_in"}}
	rng := rand.New(rand.NewSource(11))
	game := NewGame(rules, 2, 1, 1, dec, rng, false)

	for i := 0; i < 100 && !game.Ruined(); i++ {
		if _, err := game.PlayRound(context.Background()); err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
	}
	if !game.Ruined() {
		t.Fatal("all-in with bankroll 2 should eventually ruin")
	}
	if _, err := game.PlayRound(context.Background()); err == nil {
		t.Fatal("expected error when playing after ruin")
	}
}
