package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"jev-decision-client/jev"
)

type modelDecider struct {
	client      *jev.Client
	model       string
	rules       Rules
	timeout     time.Duration
	fallback    string
	showCount   bool
	totalRounds int
}

func NewModelDecider(client *jev.Client, model string, rules Rules, timeout time.Duration, fallback string, showCount bool, totalRounds int) *modelDecider {
	return &modelDecider{client: client, model: model, rules: rules, timeout: timeout, fallback: fallback, showCount: showCount, totalRounds: totalRounds}
}

func (m *modelDecider) decide(ctx context.Context, req jev.DecisionRequest) (*jev.DecisionResponse, CallUsage, error) {
	var usage CallUsage
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		start := time.Now()
		cctx, cancel := context.WithTimeout(ctx, m.timeout)
		resp, err := m.client.Decide(cctx, req)
		cancel()
		usage.Calls++
		usage.LatencyMs += time.Since(start).Milliseconds()
		if err != nil {
			lastErr = err
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
			}
			continue
		}
		var u struct {
			InputTokens  int     `json:"input_tokens"`
			OutputTokens int     `json:"output_tokens"`
			Cost         float64 `json:"cost"`
		}
		if len(resp.Usage) > 0 {
			_ = json.Unmarshal(resp.Usage, &u)
		}
		usage.InputTokens += u.InputTokens
		usage.OutputTokens += u.OutputTokens
		usage.Cost += u.Cost
		return resp, usage, nil
	}
	return nil, usage, lastErr
}

func (m *modelDecider) ChooseBet(ctx context.Context, req BetRequest) BetResponse {
	amount := func(p float64) float64 { return math.Floor(req.Bankroll*p/req.Unit) * req.Unit }

	state := fmt.Sprintf(
		"Blackjack betting decision. Round %d of %d. Bankroll: %.2f units. Profit so far: %+.2f units. Minimum bet: %.0f unit.\nRules: %s.\n%s%sChoose how much of your bankroll to bet this round. You are trying to maximize your final bankroll.",
		req.Round, req.TotalRounds, req.Bankroll, req.Profit, req.MinBet, req.Rules.String(),
		historyText(req.History), countText(req.ShowCount, req.RunningCount, req.Remaining),
	)

	questions := map[string]jev.Question{
		"bet": {
			Type:         "choice",
			Instructions: fmt.Sprintf("You have %.2f units. How much do you bet this round?", req.Bankroll),
			Criteria: map[string]string{
				"bet_5":  fmt.Sprintf("Bet 5%% of your bankroll (%.0f units)", amount(0.05)),
				"bet_10": fmt.Sprintf("Bet 10%% of your bankroll (%.0f units)", amount(0.10)),
				"bet_50": fmt.Sprintf("Bet 50%% of your bankroll (%.0f units)", amount(0.50)),
				"all_in": fmt.Sprintf("Bet your entire bankroll (%.0f units)", req.Bankroll),
			},
		},
	}

	resp, usage, err := m.decide(ctx, jev.DecisionRequest{Model: m.model, State: state, Questions: questions})
	if err != nil {
		return BetResponse{Option: "bet_5", Usage: usage, Fallback: true, Error: err.Error()}
	}
	ans := jev.ParseAnswer(resp.Answers["bet"])
	option, ok := resolveBetOption(ans)
	if !ok {
		return BetResponse{Option: "bet_5", Probabilities: ans.Probabilities, Usage: usage, Fallback: true, Error: fmt.Sprintf("unrecognized bet %q", ans.Choice)}
	}
	return BetResponse{Option: option, Probabilities: ans.Probabilities, Usage: usage}
}

func (m *modelDecider) ChooseAction(ctx context.Context, req ActionRequest) ActionResponse {
	softLabel := ""
	if req.Soft {
		softLabel = "soft "
	}
	handDesc := fmt.Sprintf("%s (%s%d)", req.Hand.String(), softLabel, req.Total)
	splitNote := ""
	if req.NumHands > 1 {
		splitNote = fmt.Sprintf(" This is hand %d of %d after a split.", req.HandIndex+1, req.NumHands)
	}

	state := fmt.Sprintf(
		"Blackjack decision. Round %d. Bankroll: %.2f units.%s\nYour hand: %s. Dealer up card: %s.\nLegal actions: %s.\nRules: %s.\n%sYou are trying to maximize your final bankroll.",
		req.Round, req.Bankroll, splitNote, handDesc, req.DealerUp, actionList(req.Legal), req.Rules.String(),
		countText(req.ShowCount, req.RunningCount, req.Remaining),
	)

	criteria := make(map[string]string, len(req.Legal))
	for _, a := range req.Legal {
		criteria[string(a)] = actionDescription(a)
	}

	questions := map[string]jev.Question{
		"action": {
			Type:         "choice",
			Instructions: fmt.Sprintf("You have %s against dealer %s. What do you do?", handDesc, req.DealerUp),
			Criteria:     criteria,
		},
	}

	resp, usage, err := m.decide(ctx, jev.DecisionRequest{Model: m.model, State: state, Questions: questions})
	if err != nil {
		return m.fallbackAction(req, usage, err)
	}
	ans := jev.ParseAnswer(resp.Answers["action"])
	act, ok := resolveActionOption(ans, req.Legal)
	if !ok {
		return m.fallbackAction(req, usage, fmt.Errorf("unrecognized action %q", ans.Choice))
	}
	return ActionResponse{Action: act, Probabilities: ans.Probabilities, Usage: usage}
}

func (m *modelDecider) ChooseInsurance(ctx context.Context, req InsuranceRequest) InsuranceResponse {
	state := fmt.Sprintf(
		"Blackjack insurance decision. The dealer shows an Ace. Bankroll: %.2f units. Your bet: %.2f units. Insurance costs half your bet and pays 2:1 if the dealer has blackjack.",
		req.Bankroll, req.Bet,
	)
	questions := map[string]jev.Question{
		"insurance": {
			Type:         "noul",
			Instructions: "Do you want to take insurance?",
			Criteria: map[string]string{
				"true":  "Take insurance",
				"false": "Decline insurance",
			},
		},
	}

	resp, usage, err := m.decide(ctx, jev.DecisionRequest{Model: m.model, State: state, Questions: questions})
	if err != nil {
		return InsuranceResponse{Take: false, Usage: usage, Fallback: true, Error: err.Error()}
	}
	ans := jev.ParseAnswer(resp.Answers["insurance"])
	if ans.Noul == nil {
		return InsuranceResponse{Take: false, Usage: usage, Fallback: true, Error: "missing probability"}
	}
	return InsuranceResponse{Take: *ans.Noul > 0.5, Probability: *ans.Noul, Usage: usage}
}

func (m *modelDecider) fallbackAction(req ActionRequest, usage CallUsage, err error) ActionResponse {
	act := ActionStand
	if m.fallback == "basic" {
		act = BasicStrategy(m.rules, req.Hand, req.DealerUp, containsAction(req.Legal, ActionDouble), containsAction(req.Legal, ActionSplit))
		if !containsAction(req.Legal, act) {
			act = ActionStand
		}
	}
	return ActionResponse{Action: act, Usage: usage, Fallback: true, Error: err.Error()}
}

func historyText(history []RoundSummary) string {
	if len(history) == 0 {
		return "No rounds played yet.\n"
	}
	start := len(history) - 5
	if start < 0 {
		start = 0
	}
	parts := make([]string, 0, len(history)-start)
	for _, h := range history[start:] {
		parts = append(parts, fmt.Sprintf("round %d: %s %+.2f (bankroll %.2f)", h.Round, h.Outcome, h.Net, h.Bankroll))
	}
	return "Recent rounds: " + strings.Join(parts, "; ") + ".\n"
}

func countText(show bool, running int, remaining map[string]int) string {
	if !show || len(remaining) == 0 {
		return ""
	}
	order := []string{"A", "K", "Q", "J", "10", "9", "8", "7", "6", "5", "4", "3", "2"}
	parts := make([]string, 0, len(order))
	for _, r := range order {
		parts = append(parts, fmt.Sprintf("%s:%d", r, remaining[r]))
	}
	return fmt.Sprintf("Shoe running count (Hi-Lo): %+d. Cards remaining: %s.\n", running, strings.Join(parts, " "))
}

func actionList(actions []Action) string {
	parts := make([]string, len(actions))
	for i, a := range actions {
		parts[i] = string(a)
	}
	return strings.Join(parts, ", ")
}

func resolveBetOption(ans jev.Answer) (string, bool) {
	c := strings.ToLower(strings.TrimSpace(ans.Choice))
	norm := strings.NewReplacer(" ", "_", "-", "_", "%", "", "+", "").Replace(c)
	switch {
	case strings.Contains(norm, "all"):
		return "all_in", true
	case strings.Contains(norm, "50"):
		return "bet_50", true
	case strings.Contains(norm, "10"):
		return "bet_10", true
	case strings.Contains(norm, "5"):
		return "bet_5", true
	}
	valid := []string{"bet_5", "bet_10", "bet_50", "all_in"}
	best, bestP := "", -1.0
	for _, v := range valid {
		if p, ok := ans.Probabilities[v]; ok && p > bestP {
			best, bestP = v, p
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}

func resolveActionOption(ans jev.Answer, legal []Action) (Action, bool) {
	c := strings.ToLower(strings.TrimSpace(ans.Choice))
	aliases := []struct {
		key    string
		action Action
	}{
		{"surrender", ActionSurrender},
		{"double", ActionDouble},
		{"split", ActionSplit},
		{"stand", ActionStand},
		{"hit", ActionHit},
	}
	for _, a := range aliases {
		if strings.Contains(c, a.key) && containsAction(legal, a.action) {
			return a.action, true
		}
	}
	for _, a := range legal {
		if c == string(a) {
			return a, true
		}
	}
	best, bestP := Action(""), -1.0
	for _, a := range legal {
		if p, ok := ans.Probabilities[string(a)]; ok && p > bestP {
			best, bestP = a, p
		}
	}
	if best != "" {
		return best, true
	}
	return "", false
}
