package main

import (
	"math"
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
