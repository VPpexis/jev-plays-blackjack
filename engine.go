package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
)

type CallUsage struct {
	Calls        int     `json:"calls"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost"`
	LatencyMs    int64   `json:"latency_ms"`
}

func (u *CallUsage) Add(o CallUsage) {
	u.Calls += o.Calls
	u.InputTokens += o.InputTokens
	u.OutputTokens += o.OutputTokens
	u.Cost += o.Cost
	u.LatencyMs += o.LatencyMs
}

type DecisionRecord struct {
	Kind          string             `json:"kind"`
	Round         int                `json:"round"`
	HandIndex     int                `json:"hand_index"`
	PlayerCards   []string           `json:"player_cards,omitempty"`
	PlayerTotal   int                `json:"player_total,omitempty"`
	Soft          bool               `json:"soft,omitempty"`
	DealerUp      string             `json:"dealer_up,omitempty"`
	Legal         []string           `json:"legal,omitempty"`
	Action        string             `json:"action"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	BasicAction   string             `json:"basic_action,omitempty"`
	Agrees        *bool              `json:"agrees,omitempty"`
	EVLoss        float64            `json:"ev_loss,omitempty"`
	EVChecked     bool               `json:"ev_checked,omitempty"`
	Fallback      bool               `json:"fallback,omitempty"`
	Error         string             `json:"error,omitempty"`
	LatencyMs     int64              `json:"latency_ms,omitempty"`
	Usage         CallUsage          `json:"usage"`
}

type HandRecord struct {
	Cards       []string `json:"cards"`
	Total       int      `json:"total"`
	Soft        bool     `json:"soft"`
	Bet         float64  `json:"bet"`
	Wagered     float64  `json:"wagered"`
	Doubled     bool     `json:"doubled,omitempty"`
	FromSplit   bool     `json:"from_split,omitempty"`
	SplitAces   bool     `json:"split_aces,omitempty"`
	Surrendered bool     `json:"surrendered,omitempty"`
	Blackjack   bool     `json:"blackjack,omitempty"`
	Bust        bool     `json:"bust,omitempty"`
	Outcome     string   `json:"outcome"`
	Payout      float64  `json:"payout"`
	Net         float64  `json:"net"`
}

type RoundRecord struct {
	Round           int              `json:"round"`
	BetOption       string           `json:"bet_option"`
	Bet             float64          `json:"bet"`
	BankrollBefore  float64          `json:"bankroll_before"`
	BankrollAfter   float64          `json:"bankroll_after"`
	Hands           []HandRecord     `json:"hands"`
	DealerCards     []string         `json:"dealer_cards"`
	DealerTotal     int              `json:"dealer_total"`
	DealerBlackjack bool             `json:"dealer_blackjack"`
	DealerBust      bool             `json:"dealer_bust"`
	InsuranceBet    float64          `json:"insurance_bet,omitempty"`
	Net             float64          `json:"net"`
	Outcome         string           `json:"outcome"`
	Decisions       []DecisionRecord `json:"decisions"`
	ShoeRemaining   int              `json:"shoe_remaining"`
	RunningCount    int              `json:"running_count"`
}

type RoundSummary struct {
	Round    int     `json:"round"`
	Bet      float64 `json:"bet"`
	Outcome  string  `json:"outcome"`
	Net      float64 `json:"net"`
	Bankroll float64 `json:"bankroll"`
}

func (r RoundRecord) Usage() CallUsage {
	var u CallUsage
	for _, d := range r.Decisions {
		u.Add(d.Usage)
	}
	return u
}

type BetRequest struct {
	Round        int
	TotalRounds  int
	Bankroll     float64
	MinBet       float64
	Unit         float64
	Profit       float64
	Rules        Rules
	History      []RoundSummary
	RunningCount int
	Remaining    map[string]int
	ShowCount    bool
}

type BetResponse struct {
	Option        string
	Flat          float64
	Probabilities map[string]float64
	Usage         CallUsage
	Fallback      bool
	Error         string
}

type ActionRequest struct {
	Round        int
	Bankroll     float64
	Hand         Hand
	Total        int
	Soft         bool
	HandIndex    int
	NumHands     int
	FromSplit    bool
	SplitAces    bool
	DealerUp     Card
	Legal        []Action
	Rules        Rules
	RunningCount int
	Remaining    map[string]int
	ShowCount    bool
}

type ActionResponse struct {
	Action        Action
	Probabilities map[string]float64
	Usage         CallUsage
	Fallback      bool
	Error         string
}

type InsuranceRequest struct {
	Round    int
	Bankroll float64
	Bet      float64
	Rules    Rules
}

type InsuranceResponse struct {
	Take        bool
	Probability float64
	Usage       CallUsage
	Fallback    bool
	Error       string
}

type Decider interface {
	ChooseBet(ctx context.Context, req BetRequest) BetResponse
	ChooseAction(ctx context.Context, req ActionRequest) ActionResponse
	ChooseInsurance(ctx context.Context, req InsuranceRequest) InsuranceResponse
}

type playerHand struct {
	cards       Hand
	bet         float64
	wagered     float64
	doubled     bool
	fromSplit   bool
	splitAces   bool
	surrendered bool
	done        bool
	payout      float64
}

func (h *playerHand) natural() bool { return !h.fromSplit && h.cards.Blackjack() }

type Game struct {
	rules         Rules
	shoe          *Shoe
	bankroll      float64
	startBankroll float64
	minBet        float64
	unit          float64
	decider       Decider
	ev            *evEstimator
	round         int
	history       []RoundSummary
	showCount     bool
	ruined        bool
	peak          float64
	low           float64
	maxDD         float64
}

func NewGame(rules Rules, startBankroll, minBet, unit float64, decider Decider, rng *rand.Rand, showCount bool) *Game {
	return &Game{
		rules:         rules,
		shoe:          NewShoe(rules.Decks, rules.Penetration, rng),
		bankroll:      startBankroll,
		startBankroll: startBankroll,
		minBet:        minBet,
		unit:          unit,
		decider:       decider,
		showCount:     showCount,
		peak:          startBankroll,
		low:           startBankroll,
	}
}

func (g *Game) Bankroll() float64    { return g.bankroll }
func (g *Game) Ruined() bool         { return g.ruined }
func (g *Game) RoundsPlayed() int    { return g.round }
func (g *Game) MaxDrawdown() float64 { return g.maxDD }

func (g *Game) EnableEV(rollouts int, rng *rand.Rand) {
	if rollouts > 0 {
		g.ev = &evEstimator{rules: g.rules, rng: rng, rollouts: rollouts}
	}
}

func (g *Game) PlayRound(ctx context.Context) (RoundRecord, error) {
	if g.ruined {
		return RoundRecord{}, fmt.Errorf("bankroll %.2f is below the minimum bet %.2f", g.bankroll, g.minBet)
	}
	if g.shoe.NeedsShuffle() {
		g.shoe.Shuffle()
	}

	g.round++
	rec := RoundRecord{
		Round:          g.round,
		BankrollBefore: g.bankroll,
		RunningCount:   g.shoe.RunningCount(),
	}

	betResp := g.decider.ChooseBet(ctx, g.betRequest())
	rec.BetOption = betResp.Option
	rec.Bet = g.betAmount(betResp)
	rec.Decisions = append(rec.Decisions, betDecisionRecord(g.round, betResp))
	g.bankroll -= rec.Bet

	player := &playerHand{cards: Hand{g.shoe.Draw(), g.shoe.Draw()}, bet: rec.Bet, wagered: rec.Bet}
	dealer := Hand{g.shoe.Draw(), g.shoe.Draw()}

	if g.rules.Insurance && dealer[0].Rank == "A" {
		ins := g.decider.ChooseInsurance(ctx, InsuranceRequest{Round: g.round, Bankroll: g.bankroll, Bet: rec.Bet, Rules: g.rules})
		rec.Decisions = append(rec.Decisions, insuranceDecisionRecord(g.round, ins))
		if ins.Take {
			amount := math.Floor(math.Min(rec.Bet/2, g.bankroll)/g.unit) * g.unit
			if amount > 0 {
				g.bankroll -= amount
				rec.InsuranceBet = amount
			}
		}
	}

	hands := []*playerHand{player}

	if !player.natural() && !(g.rules.DealerPeeks && dealer.Blackjack()) {
		for i := 0; i < len(hands); i++ {
			h := hands[i]
			if h.done {
				continue
			}
			if h.splitAces && g.rules.SplitAcesOneCard {
				h.done = true
				continue
			}
			for !h.done {
				if h.cards.Total() >= 21 {
					h.done = true
					break
				}
				legal := g.legalActions(h, hands, dealer)
				if len(legal) == 0 {
					h.done = true
					break
				}
				resp := g.decider.ChooseAction(ctx, g.actionRequest(h, i, hands, dealer[0], legal))
				act := resp.Action
				if !containsAction(legal, act) {
					act = ActionStand
					resp.Action = act
				}
				evLoss, evChecked := g.evLoss(h.cards, dealer[0], legal, resp.Action)
				rec.Decisions = append(rec.Decisions, actionDecisionRecord(g.round, i, h, dealer[0], legal, resp, g.rules, evLoss, evChecked))

				switch act {
				case ActionHit:
					h.cards = append(h.cards, g.shoe.Draw())
					if h.cards.Total() >= 21 {
						h.done = true
					}
				case ActionStand:
					h.done = true
				case ActionDouble:
					g.bankroll -= h.bet
					h.wagered += h.bet
					h.bet = h.wagered
					h.doubled = true
					h.cards = append(h.cards, g.shoe.Draw())
					h.done = true
				case ActionSplit:
					aceSplit := h.cards[0].Rank == "A"
					newHand := &playerHand{cards: Hand{h.cards[1], g.shoe.Draw()}, bet: h.bet, wagered: h.bet, fromSplit: true, splitAces: aceSplit}
					h.cards = Hand{h.cards[0], g.shoe.Draw()}
					h.fromSplit = true
					h.splitAces = aceSplit
					g.bankroll -= h.bet
					rebuilt := make([]*playerHand, 0, len(hands)+1)
					rebuilt = append(rebuilt, hands[:i+1]...)
					rebuilt = append(rebuilt, newHand)
					rebuilt = append(rebuilt, hands[i+1:]...)
					hands = rebuilt
					if h.splitAces && g.rules.SplitAcesOneCard {
						h.done = true
					}
				case ActionSurrender:
					h.surrendered = true
					h.done = true
				}
			}
		}
	}

	dealerPlays := false
	if !dealer.Blackjack() {
		for _, h := range hands {
			if !h.cards.Bust() && !h.natural() && !h.surrendered {
				dealerPlays = true
				break
			}
		}
	}
	if dealerPlays {
		for dealerShouldHit(dealer, g.rules) {
			dealer = append(dealer, g.shoe.Draw())
		}
	}

	for _, h := range hands {
		h.payout = resolveHand(h, dealer, g.rules)
		g.bankroll += h.payout
	}
	if rec.InsuranceBet > 0 && dealer.Blackjack() {
		g.bankroll += rec.InsuranceBet * 3
	}

	rec.DealerCards = dealer.CardStrings()
	rec.DealerTotal = dealer.Total()
	rec.DealerBlackjack = dealer.Blackjack()
	rec.DealerBust = dealer.Bust()
	rec.Hands = handRecords(hands, dealer, g.rules)
	rec.BankrollAfter = g.bankroll
	rec.Net = g.bankroll - rec.BankrollBefore
	rec.Outcome = roundOutcome(rec.Net)
	rec.ShoeRemaining = g.shoe.Remaining()

	g.updateRisk()
	g.history = append(g.history, RoundSummary{Round: g.round, Bet: rec.Bet, Outcome: rec.Outcome, Net: rec.Net, Bankroll: g.bankroll})
	if g.bankroll < g.minBet {
		g.ruined = true
	}
	return rec, nil
}

func (g *Game) betAmount(resp BetResponse) float64 {
	if resp.Flat > 0 || resp.Option == "flat" {
		amount := resp.Flat
		if amount <= 0 {
			amount = g.minBet
		}
		if amount > g.bankroll {
			amount = g.bankroll
		}
		if amount < g.minBet {
			amount = math.Min(g.minBet, g.bankroll)
		}
		return amount
	}
	pct := 0.05
	switch resp.Option {
	case "bet_10":
		pct = 0.10
	case "bet_50":
		pct = 0.50
	case "all_in":
		pct = 1.0
	}
	amount := math.Floor(g.bankroll*pct/g.unit) * g.unit
	if resp.Option == "all_in" {
		amount = g.bankroll
	}
	if amount < g.minBet {
		amount = math.Min(g.minBet, g.bankroll)
	}
	if amount > g.bankroll {
		amount = g.bankroll
	}
	return amount
}

func (g *Game) betRequest() BetRequest {
	return BetRequest{
		Round:        g.round,
		Bankroll:     g.bankroll,
		MinBet:       g.minBet,
		Unit:         g.unit,
		Profit:       g.bankroll - g.startBankroll,
		Rules:        g.rules,
		History:      g.history,
		RunningCount: g.shoe.RunningCount(),
		Remaining:    g.shoe.Composition(),
		ShowCount:    g.showCount,
	}
}

func (g *Game) actionRequest(h *playerHand, index int, hands []*playerHand, up Card, legal []Action) ActionRequest {
	return ActionRequest{
		Round:        g.round,
		Bankroll:     g.bankroll,
		Hand:         h.cards,
		Total:        h.cards.Total(),
		Soft:         h.cards.Soft(),
		HandIndex:    index,
		NumHands:     len(hands),
		FromSplit:    h.fromSplit,
		SplitAces:    h.splitAces,
		DealerUp:     up,
		Legal:        legal,
		Rules:        g.rules,
		RunningCount: g.shoe.RunningCount(),
		Remaining:    g.shoe.Composition(),
		ShowCount:    g.showCount,
	}
}

func (g *Game) legalActions(h *playerHand, hands []*playerHand, dealer Hand) []Action {
	legal := []Action{ActionHit, ActionStand}

	canDouble := g.rules.DoubleAllowed && len(h.cards) == 2 && g.bankroll >= h.bet
	if canDouble && !g.rules.DoubleAnyTwo {
		t := h.cards.Total()
		canDouble = t >= 9 && t <= 11
	}
	if h.fromSplit && !g.rules.DoubleAfterSplit {
		canDouble = false
	}
	if canDouble {
		legal = append(legal, ActionDouble)
	}

	if len(h.cards) == 2 && h.cards.Pair() && len(hands) < g.rules.MaxHands && g.bankroll >= h.bet {
		if !h.splitAces || g.rules.ResplitAces {
			legal = append(legal, ActionSplit)
		}
	}

	if g.rules.LateSurrender && len(h.cards) == 2 && !h.fromSplit && !dealer.Blackjack() {
		legal = append(legal, ActionSurrender)
	}
	return legal
}

func (g *Game) evLoss(hand Hand, up Card, legal []Action, chosen Action) (float64, bool) {
	if g.ev == nil {
		return 0, false
	}
	basic := BasicStrategy(g.rules, hand, up, containsAction(legal, ActionDouble), containsAction(legal, ActionSplit))
	if basic == chosen {
		return 0, false
	}
	remaining := g.shoe.RemainingCards()
	basicEV := g.ev.estimate(hand, up, remaining, basic)
	chosenEV := g.ev.estimate(hand, up, remaining, chosen)
	if math.IsNaN(basicEV) || math.IsNaN(chosenEV) {
		return 0, false
	}
	return basicEV - chosenEV, true
}

func (g *Game) updateRisk() {
	if g.bankroll > g.peak {
		g.peak = g.bankroll
	}
	if g.bankroll < g.low {
		g.low = g.bankroll
	}
	if dd := g.peak - g.bankroll; dd > g.maxDD {
		g.maxDD = dd
	}
}

func dealerShouldHit(h Hand, r Rules) bool {
	t := h.Total()
	if t < 17 {
		return true
	}
	return t == 17 && h.Soft() && r.DealerHitsSoft17
}

func resolveHand(h *playerHand, dealer Hand, r Rules) float64 {
	if h.surrendered {
		return h.bet * 0.5
	}
	if h.cards.Bust() {
		return 0
	}
	if h.natural() {
		if dealer.Blackjack() {
			return h.bet
		}
		return h.bet * (1 + r.BlackjackPays)
	}
	if dealer.Blackjack() {
		return 0
	}
	if dealer.Bust() {
		return h.bet * 2
	}
	pt, dt := h.cards.Total(), dealer.Total()
	switch {
	case pt > dt:
		return h.bet * 2
	case pt < dt:
		return 0
	default:
		return h.bet
	}
}

func handOutcome(h *playerHand, dealer Hand, r Rules) string {
	if h.surrendered {
		return "SURRENDER"
	}
	if h.cards.Bust() {
		return "LOSS"
	}
	if h.natural() {
		if dealer.Blackjack() {
			return "PUSH"
		}
		return "WIN"
	}
	if dealer.Blackjack() {
		return "LOSS"
	}
	if dealer.Bust() {
		return "WIN"
	}
	switch {
	case h.cards.Total() > dealer.Total():
		return "WIN"
	case h.cards.Total() < dealer.Total():
		return "LOSS"
	default:
		return "PUSH"
	}
}

func handRecords(hands []*playerHand, dealer Hand, r Rules) []HandRecord {
	out := make([]HandRecord, 0, len(hands))
	for _, h := range hands {
		out = append(out, HandRecord{
			Cards:       h.cards.CardStrings(),
			Total:       h.cards.Total(),
			Soft:        h.cards.Soft(),
			Bet:         h.bet,
			Wagered:     h.wagered,
			Doubled:     h.doubled,
			FromSplit:   h.fromSplit,
			SplitAces:   h.splitAces,
			Surrendered: h.surrendered,
			Blackjack:   h.natural(),
			Bust:        h.cards.Bust(),
			Outcome:     handOutcome(h, dealer, r),
			Payout:      h.payout,
			Net:         h.payout - h.wagered,
		})
	}
	return out
}

func roundOutcome(net float64) string {
	switch {
	case net > 0:
		return "WIN"
	case net < 0:
		return "LOSS"
	default:
		return "PUSH"
	}
}

func betDecisionRecord(round int, resp BetResponse) DecisionRecord {
	d := DecisionRecord{
		Kind:          "bet",
		Round:         round,
		Action:        resp.Option,
		Probabilities: resp.Probabilities,
		Fallback:      resp.Fallback,
		Error:         resp.Error,
		LatencyMs:     resp.Usage.LatencyMs,
		Usage:         resp.Usage,
	}
	if p, ok := resp.Probabilities[resp.Option]; ok {
		d.Confidence = p
	}
	return d
}

func actionDecisionRecord(round, index int, h *playerHand, up Card, legal []Action, resp ActionResponse, r Rules, evLoss float64, evChecked bool) DecisionRecord {
	legalStrs := make([]string, len(legal))
	for i, a := range legal {
		legalStrs[i] = string(a)
	}
	basic := BasicStrategy(r, h.cards, up, containsAction(legal, ActionDouble), containsAction(legal, ActionSplit))
	agree := basic == resp.Action
	d := DecisionRecord{
		Kind:          "action",
		Round:         round,
		HandIndex:     index,
		PlayerCards:   h.cards.CardStrings(),
		PlayerTotal:   h.cards.Total(),
		Soft:          h.cards.Soft(),
		DealerUp:      up.String(),
		Legal:         legalStrs,
		Action:        string(resp.Action),
		Probabilities: resp.Probabilities,
		BasicAction:   string(basic),
		Agrees:        &agree,
		EVLoss:        evLoss,
		EVChecked:     evChecked,
		Fallback:      resp.Fallback,
		Error:         resp.Error,
		LatencyMs:     resp.Usage.LatencyMs,
		Usage:         resp.Usage,
	}
	if p, ok := resp.Probabilities[string(resp.Action)]; ok {
		d.Confidence = p
	}
	return d
}

func insuranceDecisionRecord(round int, resp InsuranceResponse) DecisionRecord {
	action := "decline"
	if resp.Take {
		action = "take"
	}
	return DecisionRecord{
		Kind:       "insurance",
		Round:      round,
		Action:     action,
		Confidence: resp.Probability,
		Fallback:   resp.Fallback,
		Error:      resp.Error,
		LatencyMs:  resp.Usage.LatencyMs,
		Usage:      resp.Usage,
	}
}
