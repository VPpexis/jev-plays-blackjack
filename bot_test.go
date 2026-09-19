package main

import (
	"context"
	"math/rand"
	"testing"
)

func TestBasicBotActions(t *testing.T) {
	bot := NewBasicBot(baseRules(), 0)
	ctx := context.Background()

	cases := []struct {
		name  string
		hand  Hand
		up    Card
		legal []Action
		want  Action
	}{
		{"16 vs 10 hits", Hand{{"10", "S"}, {"6", "H"}}, Card{"10", "D"}, []Action{ActionHit, ActionStand}, ActionHit},
		{"16 vs 6 stands", Hand{{"10", "S"}, {"6", "H"}}, Card{"6", "D"}, []Action{ActionHit, ActionStand, ActionDouble}, ActionStand},
		{"8s vs 10 splits", Hand{{"8", "S"}, {"8", "H"}}, Card{"10", "D"}, []Action{ActionHit, ActionStand, ActionSplit}, ActionSplit},
		{"11 vs 6 doubles", Hand{{"5", "S"}, {"6", "H"}}, Card{"6", "D"}, []Action{ActionHit, ActionStand, ActionDouble}, ActionDouble},
		{"11 vs 6 without double hits", Hand{{"5", "S"}, {"6", "H"}}, Card{"6", "D"}, []Action{ActionHit, ActionStand}, ActionHit},
	}

	for _, c := range cases {
		resp := bot.ChooseAction(ctx, ActionRequest{Hand: c.hand, DealerUp: c.up, Legal: c.legal})
		if resp.Action != c.want {
			t.Errorf("%s: got %s want %s", c.name, resp.Action, c.want)
		}
	}
}

func TestBasicBotBetAndInsurance(t *testing.T) {
	bot := NewBasicBot(baseRules(), 0)
	bet := bot.ChooseBet(context.Background(), BetRequest{MinBet: 10, Bankroll: 100, Unit: 1})
	if bet.Option != "flat" || bet.Flat != 10 {
		t.Fatalf("expected flat bet of 10, got %+v", bet)
	}

	flat := NewBasicBot(baseRules(), 25)
	bet = flat.ChooseBet(context.Background(), BetRequest{MinBet: 10, Bankroll: 100, Unit: 1})
	if bet.Flat != 25 {
		t.Fatalf("expected flat bet of 25, got %v", bet.Flat)
	}

	ins := bot.ChooseInsurance(context.Background(), InsuranceRequest{})
	if ins.Take {
		t.Fatal("basic strategy never takes insurance")
	}

	cont := bot.ChooseContinue(context.Background(), ContinueRequest{Round: 50, TotalRounds: 100, Bankroll: 200, StartBankroll: 100, Profit: 100})
	if !cont.Continue || cont.Stop {
		t.Fatal("the basic bot never ends the session early")
	}
}

func TestBasicBotPlaysRound(t *testing.T) {
	rules := baseRules()
	bot := NewBasicBot(rules, 0)
	game := NewGame(rules, 1000, 10, 1, bot, rand.New(rand.NewSource(9)), false)

	for i := 0; i < 20; i++ {
		rec, err := game.PlayRound(context.Background())
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if rec.Bet != 10 {
			t.Fatalf("expected flat bet 10, got %v", rec.Bet)
		}
		for _, d := range rec.Decisions {
			if d.Kind == "action" && d.Agrees != nil && !*d.Agrees {
				t.Fatalf("bot should always agree with basic strategy: %+v", d)
			}
			if d.Fallback || d.Error != "" {
				t.Fatalf("bot should never fall back: %+v", d)
			}
		}
	}
}
