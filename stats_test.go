package main

import (
	"math"
	"reflect"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func sampleRecords() []RoundRecord {
	return []RoundRecord{
		{
			Round:          1,
			Bet:            10,
			BetOption:      "bet_10",
			BankrollBefore: 100,
			BankrollAfter:  110,
			Net:            10,
			Outcome:        "WIN",
			Hands: []HandRecord{
				{Cards: []string{"10S", "9H"}, Total: 19, Bet: 10, Wagered: 10, Outcome: "WIN", Payout: 20, Net: 10},
			},
			DealerCards: []string{"10S", "8H"},
			DealerTotal: 18,
			Decisions: []DecisionRecord{
				{Kind: "bet", Action: "bet_10", Confidence: 0.7, Usage: CallUsage{Calls: 1, Cost: 0.001}, LatencyMs: 50},
				{Kind: "action", Action: "stand", Confidence: 0.9, Agrees: boolPtr(true), Usage: CallUsage{Calls: 1, Cost: 0.001}, LatencyMs: 100},
			},
		},
		{
			Round:          2,
			Bet:            10,
			BetOption:      "all_in",
			BankrollBefore: 110,
			BankrollAfter:  100,
			Net:            -10,
			Outcome:        "LOSS",
			Hands: []HandRecord{
				{Cards: []string{"10S", "6H", "5D"}, Total: 21, Bet: 10, Wagered: 10, Outcome: "LOSS", Payout: 0, Net: -10},
			},
			DealerCards: []string{"10S", "10H"},
			DealerTotal: 20,
			Decisions: []DecisionRecord{
				{Kind: "bet", Action: "all_in", Confidence: 0.4, Usage: CallUsage{Calls: 1, Cost: 0.001}, LatencyMs: 50},
				{Kind: "action", Action: "hit", Confidence: 0.6, Agrees: boolPtr(false), Usage: CallUsage{Calls: 1, Cost: 0.001}, LatencyMs: 200},
			},
		},
	}
}

func TestAggregateSides(t *testing.T) {
	p, d := aggregateSides(sampleRecords())
	if p.Hands != 2 || p.Wins != 1 || p.Losses != 1 {
		t.Fatalf("player stats wrong: %+v", p)
	}
	if d.Wins != 1 || d.Losses != 1 {
		t.Fatalf("dealer stats should mirror player: %+v", d)
	}
	if math.Abs(p.AvgTotal-20) > 1e-9 {
		t.Fatalf("player avg total should be 20, got %v", p.AvgTotal)
	}
	if math.Abs(d.AvgTotal-19) > 1e-9 {
		t.Fatalf("dealer avg total should be 19, got %v", d.AvgTotal)
	}
}

func TestAggregateBetting(t *testing.T) {
	b := aggregateBetting(sampleRecords(), 100, false)
	if b.Profit != 0 {
		t.Fatalf("profit should be 0, got %v", b.Profit)
	}
	if b.TotalWagered != 20 {
		t.Fatalf("wagered should be 20, got %v", b.TotalWagered)
	}
	if math.Abs(b.HouseEdge) > 1e-9 {
		t.Fatalf("house edge should be 0, got %v", b.HouseEdge)
	}
	if b.PeakBankroll != 110 || b.MinBankroll != 100 {
		t.Fatalf("peak/low wrong: %v %v", b.PeakBankroll, b.MinBankroll)
	}
	if b.MaxDrawdown != 10 {
		t.Fatalf("drawdown should be 10, got %v", b.MaxDrawdown)
	}
	if b.AllInRounds != 1 || b.AllInWins != 0 {
		t.Fatalf("all-in stats wrong: %d %d", b.AllInRounds, b.AllInWins)
	}
	if b.WinRateByOption["bet_10"] != 1 {
		t.Fatalf("bet_10 win rate should be 1, got %v", b.WinRateByOption["bet_10"])
	}
}

func TestAggregateDecisions(t *testing.T) {
	d := aggregateDecisions(sampleRecords())
	if d.Total != 4 {
		t.Fatalf("expected 4 decisions, got %d", d.Total)
	}
	if d.AgreementChecked != 2 || d.AgreementCount != 1 {
		t.Fatalf("agreement wrong: %d/%d", d.AgreementCount, d.AgreementChecked)
	}
	if math.Abs(d.AgreementRate-0.5) > 1e-9 {
		t.Fatalf("agreement rate should be 0.5, got %v", d.AgreementRate)
	}
	if d.BrierN != 2 {
		t.Fatalf("expected 2 brier samples, got %d", d.BrierN)
	}
	expectedBrier := (math.Pow(0.9-1, 2) + math.Pow(0.6-0, 2)) / 2
	if math.Abs(d.BrierScore-expectedBrier) > 1e-9 {
		t.Fatalf("brier should be %v, got %v", expectedBrier, d.BrierScore)
	}
	if d.ByAction["stand"] != 1 || d.ByAction["hit"] != 1 {
		t.Fatalf("action mix wrong: %+v", d.ByAction)
	}
}

func TestEdgeCIContainsHeadline(t *testing.T) {
	records := []RoundRecord{
		{Bet: 100, BetOption: "bet_10", Net: -20, BankrollBefore: 1000, BankrollAfter: 980},
		{Bet: 10, BetOption: "bet_10", Net: 5, BankrollBefore: 980, BankrollAfter: 985},
		{Bet: 50, BetOption: "bet_10", Net: -30, BankrollBefore: 985, BankrollAfter: 955},
		{Bet: 20, BetOption: "bet_10", Net: 10, BankrollBefore: 955, BankrollAfter: 965},
	}
	b := aggregateBetting(records, 1000, false)
	if b.EdgeCI95[0] > b.HouseEdge || b.HouseEdge > b.EdgeCI95[1] {
		t.Fatalf("headline edge %.4f should lie inside CI %v", b.HouseEdge, b.EdgeCI95)
	}
	if b.EdgeN <= 0 {
		t.Fatalf("effective sample size should be positive, got %v", b.EdgeN)
	}
}

func TestEVAccumulation(t *testing.T) {
	agree := true
	records := []RoundRecord{
		{
			Decisions: []DecisionRecord{
				{Kind: "action", Action: "stand", Agrees: &agree, EVLoss: 0.05, EVChecked: true},
				{Kind: "action", Action: "hit", Agrees: &agree, EVLoss: 0.15, EVChecked: true},
			},
		},
	}
	d := aggregateDecisions(records)
	if d.EVChecked != 2 {
		t.Fatalf("expected 2 EV-checked decisions, got %d", d.EVChecked)
	}
	if math.Abs(d.EVLossTotal-0.2) > 1e-9 {
		t.Fatalf("expected total EV loss 0.2, got %v", d.EVLossTotal)
	}
	if math.Abs(d.EVLossPerChecked-0.1) > 1e-9 {
		t.Fatalf("expected per-decision EV loss 0.1, got %v", d.EVLossPerChecked)
	}
}

func TestDecisionStatsCountContinueKind(t *testing.T) {
	records := []RoundRecord{
		{
			Round:         1,
			BankrollAfter: 110,
			SessionEnd:    true,
			Decisions: []DecisionRecord{
				{Kind: "continue", Action: "stop", Confidence: 0.8, Probabilities: map[string]float64{"stop": 0.8, "continue": 0.2}},
			},
		},
	}
	d := aggregateDecisions(records)
	if d.ByKind["continue"] != 1 {
		t.Fatalf("continue kind not counted: %+v", d.ByKind)
	}
	if d.ByAction["stop"] != 1 {
		t.Fatalf("stop action not counted: %+v", d.ByAction)
	}
	if d.AgreementChecked != 0 || d.BrierN != 0 {
		t.Fatalf("continue decisions should not affect agreement or brier: %+v", d)
	}
}

func TestAccumulatorMatchesAggregates(t *testing.T) {
	records := sampleRecords()
	p1, d1 := aggregateSides(records)
	b1 := aggregateBetting(records, 100, false)
	x1 := aggregateDecisions(records)

	acc := NewAccumulator(100)
	for _, r := range records {
		acc.Add(r)
	}
	p2, d2 := acc.Player(), acc.Dealer()
	b2 := acc.Betting(false)
	x2 := acc.Decisions()

	if !reflect.DeepEqual(p1, p2) {
		t.Fatalf("player stats differ:\n%+v\n%+v", p1, p2)
	}
	if !reflect.DeepEqual(d1, d2) {
		t.Fatalf("dealer stats differ:\n%+v\n%+v", d1, d2)
	}
	if !reflect.DeepEqual(b1, b2) {
		t.Fatalf("betting stats differ:\n%+v\n%+v", b1, b2)
	}
	if !reflect.DeepEqual(x1, x2) {
		t.Fatalf("decision stats differ:\n%+v\n%+v", x1, x2)
	}
}
