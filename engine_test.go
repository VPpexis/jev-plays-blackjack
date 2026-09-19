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
	stops   []bool
	bi      int
	ai      int
	si      int
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

func (d *scriptDecider) ChooseContinue(ctx context.Context, req ContinueRequest) ContinueResponse {
	if d.si < len(d.stops) {
		stop := d.stops[d.si]
		d.si++
		return ContinueResponse{Continue: !stop, Stop: stop, Probability: 0.8}
	}
	return ContinueResponse{Continue: true}
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
		if got := g.betAmount(BetResponse{Option: opt}); got != want {
			t.Errorf("%s: got %v want %v", opt, got, want)
		}
	}

	small := &Game{bankroll: 7, unit: 1, minBet: 1}
	if got := small.betAmount(BetResponse{Option: "bet_5"}); got != 1 {
		t.Errorf("5%% of 7 floored to 0 should clamp to min bet 1, got %v", got)
	}
	odd := &Game{bankroll: 7, unit: 5, minBet: 5}
	if got := odd.betAmount(BetResponse{Option: "bet_10"}); got != 5 {
		t.Errorf("10%% of 7 with unit 5 should clamp to 5, got %v", got)
	}
	flat := &Game{bankroll: 100, unit: 1, minBet: 10}
	if got := flat.betAmount(BetResponse{Option: "flat", Flat: 25}); got != 25 {
		t.Errorf("flat bet should be 25, got %v", got)
	}
	if got := flat.betAmount(BetResponse{Option: "flat"}); got != 10 {
		t.Errorf("flat bet without size should use min bet 10, got %v", got)
	}
	poor := &Game{bankroll: 3, unit: 1, minBet: 10}
	if got := poor.betAmount(BetResponse{Option: "flat", Flat: 25}); got != 3 {
		t.Errorf("flat bet should clamp to bankroll 3, got %v", got)
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

func TestPayoutInvariants(t *testing.T) {
	rules := baseRules()
	actions := make([]Action, 400)
	for i := range actions {
		actions[i] = ActionDouble
	}
	dec := &scriptDecider{bets: []string{"bet_5"}, actions: actions}
	game := NewGame(rules, 100000, 1, 1, dec, rand.New(rand.NewSource(21)), false)

	doubled := 0
	for i := 0; i < 200; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		for _, h := range rec.Hands {
			if h.Doubled {
				doubled++
			}
			var multiplier float64
			switch h.Outcome {
			case "WIN":
				if h.Blackjack {
					multiplier = 2.5
				} else {
					multiplier = 2
				}
			case "PUSH":
				multiplier = 1
			case "SURRENDER":
				multiplier = 0.5
			case "LOSS":
				multiplier = 0
			}
			want := multiplier * h.Wagered
			if math.Abs(h.Payout-want) > 1e-9 {
				t.Fatalf("round %d hand %v: outcome %s payout %v want %v (bet %v wagered %v doubled %v)",
					i, h.Cards, h.Outcome, h.Payout, want, h.Bet, h.Wagered, h.Doubled)
			}
		}
	}
	if doubled == 0 {
		t.Skip("no doubles occurred")
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

func continueDecisions(rec RoundRecord) []DecisionRecord {
	var out []DecisionRecord
	for _, d := range rec.Decisions {
		if d.Kind == "continue" {
			out = append(out, d)
		}
	}
	return out
}

func TestSessionStopDecision(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_10"}, stops: []bool{false, false, true}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(13)), false)
	game.EnableStopPolicy("model", false, 1, 1, 100)

	rounds := 0
	for i := 0; i < 100; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatalf("round %d: %v", i+1, err)
		}
		rounds++
		got := continueDecisions(rec)
		if len(got) != 1 {
			t.Fatalf("round %d: expected one continue decision, got %d", rounds, len(got))
		}
		wantAction := "continue"
		if i == 2 {
			wantAction = "stop"
		}
		if got[0].Action != wantAction {
			t.Fatalf("round %d: continue action %q, want %q", rounds, got[0].Action, wantAction)
		}
		if rec.SessionEnd != (i == 2) {
			t.Fatalf("round %d: session_end %v", rounds, rec.SessionEnd)
		}
		if rec.SessionEnd {
			break
		}
	}
	if rounds != 3 {
		t.Fatalf("expected the session to end after 3 rounds, played %d", rounds)
	}
	if game.RoundsPlayed() != 3 {
		t.Fatalf("game played %d rounds, want 3", game.RoundsPlayed())
	}
}

func TestStopNotAskedOnFinalRound(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_10"}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(14)), false)
	game.EnableStopPolicy("model", false, 1, 1, 4)

	for i := 0; i < 4; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got := continueDecisions(rec)
		if i < 3 && len(got) != 1 {
			t.Fatalf("round %d: expected a continue decision, got %d", i+1, len(got))
		}
		if i == 3 && len(got) != 0 {
			t.Fatalf("round 4 is the last: no continue decision expected, got %d", len(got))
		}
	}
}

func TestStopOnlyAheadGate(t *testing.T) {
	g := &Game{stopPolicy: "model", stopOnlyAhead: true, stopMinRounds: 1, stopEvery: 1, totalRounds: 100, startBankroll: 100}
	g.round = 5
	g.bankroll = 100
	if g.shouldAskContinue() {
		t.Fatal("must not offer the stop while flat")
	}
	g.bankroll = 99
	if g.shouldAskContinue() {
		t.Fatal("must not offer the stop while behind")
	}
	g.bankroll = 101
	if !g.shouldAskContinue() {
		t.Fatal("must offer the stop while ahead")
	}

	loose := &Game{stopPolicy: "model", stopMinRounds: 1, stopEvery: 1, totalRounds: 100, startBankroll: 100}
	loose.round = 5
	loose.bankroll = 50
	if !loose.shouldAskContinue() {
		t.Fatal("without the gate the stop is offered while behind")
	}
}

func TestStopOfferedWhenAhead(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_10"}, stops: []bool{true}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(19)), false)
	game.EnableStopPolicy("model", true, 1, 1, 100)
	game.bankroll = 2000

	rec, err := game.PlayRound(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := continueDecisions(rec)
	if len(got) != 1 || got[0].Action != "stop" || !rec.SessionEnd {
		t.Fatalf("expected the stop to be offered and taken while ahead: %+v", got)
	}
}

func TestStopNotOfferedWhenBehind(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_5"}, stops: []bool{true}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(20)), false)
	game.EnableStopPolicy("model", true, 1, 1, 100)
	game.bankroll = 500

	rec, err := game.PlayRound(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(continueDecisions(rec)) != 0 || rec.SessionEnd {
		t.Fatal("the stop must not be offered while behind the starting bankroll")
	}
}

func TestStopMinRounds(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_10"}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(16)), false)
	game.EnableStopPolicy("model", false, 3, 1, 100)

	for i := 0; i < 3; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got := continueDecisions(rec)
		if i < 2 && len(got) != 0 {
			t.Fatalf("round %d: stop offered before the minimum of 3 rounds", i+1)
		}
		if i == 2 && len(got) != 1 {
			t.Fatalf("round 3: expected the stop decision, got %d", len(got))
		}
	}
}

func TestStopEveryN(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_10"}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(17)), false)
	game.EnableStopPolicy("model", false, 1, 2, 100)

	for i := 0; i < 6; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if (i+1)%2 == 0 {
			want = 1
		}
		if got := len(continueDecisions(rec)); got != want {
			t.Fatalf("round %d: %d continue decisions, want %d", i+1, got, want)
		}
	}
}

func TestStopDisabledByDefault(t *testing.T) {
	dec := &scriptDecider{bets: []string{"bet_10"}, stops: []bool{true}}
	game := NewGame(baseRules(), 1000, 1, 1, dec, rand.New(rand.NewSource(18)), false)

	for i := 0; i < 5; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(continueDecisions(rec)) != 0 || rec.SessionEnd {
			t.Fatalf("round %d: stop decisions should be off by default", i+1)
		}
	}
	if dec.si != 0 {
		t.Fatal("the decider should never be asked to continue when the policy is fixed")
	}
}
