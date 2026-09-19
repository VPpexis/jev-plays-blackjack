package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jev-decision-client/jev"
)

func TestModelDeciderEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		_ = json.Unmarshal(body, &req)

		answers := map[string]any{}
		switch {
		case req.Questions["bet"] != nil:
			answers["bet"] = map[string]any{"type": "choice", "choice": "bet_10", "probabilities": map[string]float64{"bet_5": 0.1, "bet_10": 0.7, "bet_50": 0.1, "all_in": 0.1}}
		case req.Questions["action"] != nil:
			answers["action"] = map[string]any{"type": "choice", "choice": "stand", "probabilities": map[string]float64{"hit": 0.3, "stand": 0.7}}
		default:
			answers["insurance"] = map[string]any{"type": "noul", "noul": 0.1}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":   "mock",
			"answers": answers,
			"usage":   map[string]any{"input_tokens": 120, "output_tokens": 12, "cost": 0.00002},
		})
	}))
	defer srv.Close()

	client := jev.NewClient("test")
	client.BaseURL = srv.URL

	rules := baseRules()
	dec := NewModelDecider(client, "mock", rules, 5*time.Second, "basic", false, 5)
	rng := rand.New(rand.NewSource(5))
	game := NewGame(rules, 100, 1, 1, dec, rng, false)

	rec, err := game.PlayRound(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rec.Bet != 10 {
		t.Fatalf("expected bet_10 to bet 10, got %v", rec.Bet)
	}
	var betSeen, actionSeen bool
	for _, d := range rec.Decisions {
		if d.Kind == "bet" && d.Action == "bet_10" {
			betSeen = true
		}
		if d.Kind == "action" && d.Action == "stand" {
			actionSeen = true
		}
	}
	if !betSeen {
		t.Error("expected a bet decision")
	}
	if !actionSeen {
		t.Error("expected an action decision")
	}
	if rec.Usage().Calls == 0 {
		t.Error("expected usage to be recorded")
	}
}

func TestModelFallbackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := jev.NewClient("test")
	client.BaseURL = srv.URL
	dec := NewModelDecider(client, "mock", baseRules(), 2*time.Second, "basic", false, 5)

	resp := dec.ChooseAction(context.Background(), ActionRequest{
		Hand:     Hand{{"10", "S"}, {"6", "H"}},
		DealerUp: Card{"10", "D"},
		Legal:    []Action{ActionHit, ActionStand},
	})
	if !resp.Fallback {
		t.Fatal("expected fallback on API error")
	}
	if resp.Action != ActionHit {
		t.Fatalf("basic strategy hits 16 vs 10, got %s", resp.Action)
	}
	if resp.Error == "" {
		t.Fatal("expected error to be recorded")
	}

	bet := dec.ChooseBet(context.Background(), BetRequest{Bankroll: 100, Unit: 1, MinBet: 1, TotalRounds: 5})
	if !bet.Fallback || bet.Option != "bet_5" {
		t.Fatalf("expected bet_5 fallback, got %s fallback=%v", bet.Option, bet.Fallback)
	}
}

func TestModelChooseContinue(t *testing.T) {
	for _, tc := range []struct {
		p        float64
		wantStop bool
	}{
		{0.8, true},
		{0.2, false},
	} {
		t.Run(fmt.Sprintf("p=%.1f", tc.p), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"model":   "mock",
					"answers": map[string]any{"continue": map[string]any{"type": "noul", "noul": tc.p}},
					"usage":   map[string]any{"input_tokens": 50, "output_tokens": 5, "cost": 0.00001},
				})
			}))
			defer srv.Close()

			client := jev.NewClient("test")
			client.BaseURL = srv.URL
			dec := NewModelDecider(client, "mock", baseRules(), 2*time.Second, "basic", false, 100)

			resp := dec.ChooseContinue(context.Background(), ContinueRequest{
				Round: 5, TotalRounds: 100, Bankroll: 110, StartBankroll: 100, Profit: 10, Peak: 115, MinBankroll: 95,
			})
			if resp.Stop != tc.wantStop || resp.Continue == tc.wantStop {
				t.Fatalf("p=%.1f: got stop=%v continue=%v", tc.p, resp.Stop, resp.Continue)
			}
			if math.Abs(resp.Probability-tc.p) > 1e-9 {
				t.Fatalf("p=%.1f: probability %v", tc.p, resp.Probability)
			}
			if resp.Usage.Calls == 0 || resp.Fallback || resp.Error != "" {
				t.Fatalf("unexpected usage or fallback: %+v", resp)
			}
		})
	}
}

func TestModelContinueFallbackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := jev.NewClient("test")
	client.BaseURL = srv.URL
	dec := NewModelDecider(client, "mock", baseRules(), time.Second, "basic", false, 10)

	resp := dec.ChooseContinue(context.Background(), ContinueRequest{Round: 2, TotalRounds: 10})
	if !resp.Continue || resp.Stop {
		t.Fatalf("expected the session to continue after an API error, got %+v", resp)
	}
	if !resp.Fallback || resp.Error == "" {
		t.Fatalf("expected a recorded fallback, got %+v", resp)
	}
}
