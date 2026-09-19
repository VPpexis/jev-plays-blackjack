package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"jev-decision-client/jev"
)

const (
	defaultModel  = "~typesafe/jev-latest"
	defaultResult = "blackjack_results.json"
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	_ = godotenv.Load()

	model := flag.String("model", defaultModel, "Jev model to use")
	games := flag.Int("games", envInt("BLACKJACK_GAMES", 100), "number of rounds to play (env: BLACKJACK_GAMES)")
	seed := flag.Int64("seed", time.Now().UnixNano(), "random seed for the shoe")
	results := flag.String("results", defaultResult, "path to the results JSON file")
	timeout := flag.Duration("timeout", 60*time.Second, "per-decision API timeout")
	delay := flag.Duration("delay", 0, "delay between rounds")
	quiet := flag.Bool("quiet", false, "suppress per-round console output")
	bot := flag.String("bot", "model", "who plays: model (Jev via API) or basic (local basic-strategy bot)")
	flatBet := flag.Float64("flat-bet", 0, "flat bet size for -bot basic (0 = use -min-bet)")
	noRecords := flag.Bool("no-records", false, "aggregate statistics without storing per-round records (for very large runs)")
	evRollouts := flag.Int("ev-rollouts", 200, "Monte Carlo rollouts per deviation for the EV-loss metric (0 disables)")
	stopPolicy := flag.String("stop-policy", "fixed", "session stop policy: fixed (play -games rounds) or model (the model may end the session early)")
	stopOnlyAhead := flag.Bool("stop-only-ahead", false, "only offer the stop decision while the bankroll is above the starting bankroll")
	stopMinRounds := flag.Int("stop-min-rounds", 1, "minimum rounds played before the stop decision is offered")
	stopEvery := flag.Int("stop-every", 1, "offer the stop decision every N rounds")

	decks := flag.Int("decks", 6, "number of decks in the shoe")
	penetration := flag.Float64("penetration", 0.75, "fraction of the shoe dealt before reshuffling")
	bjpay := flag.String("bjpay", "3:2", "blackjack payout: 3:2, 6:5 or 1:1")
	h17 := flag.Bool("h17", false, "dealer hits soft 17")
	peek := flag.Bool("peek", true, "dealer peeks for blackjack on ace or 10 up (US rules; disable for ENHC)")
	double := flag.Bool("double", true, "allow doubling down")
	doubleAny := flag.Bool("double-any", true, "allow doubling on any two cards (false: only on 9-11)")
	das := flag.Bool("das", true, "allow double after split")
	maxHands := flag.Int("max-hands", 4, "maximum number of hands after splits")
	surrender := flag.Bool("surrender", false, "allow late surrender")
	insurance := flag.Bool("insurance", false, "offer insurance when the dealer shows an ace")

	startBankroll := flag.Float64("bankroll", 100, "starting bankroll in units")
	unit := flag.Float64("unit", 1, "bet unit size")
	minBet := flag.Float64("min-bet", 1, "minimum bet")
	fallback := flag.String("fallback", "basic", "fallback when the API fails: basic or stand")
	showCount := flag.Bool("show-count", false, "include shoe composition and Hi-Lo count in prompts")

	flag.Parse()

	payout, err := ParseBlackjackPayout(*bjpay)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if *games < 1 {
		fmt.Println("Error: -games must be at least 1")
		os.Exit(1)
	}
	if *bot != "model" && *bot != "basic" {
		fmt.Println("Error: -bot must be 'model' or 'basic'")
		os.Exit(1)
	}
	if *evRollouts < 0 {
		fmt.Println("Error: -ev-rollouts must not be negative")
		os.Exit(1)
	}
	if *stopPolicy != "fixed" && *stopPolicy != "model" {
		fmt.Println("Error: -stop-policy must be 'fixed' or 'model'")
		os.Exit(1)
	}
	if *stopMinRounds < 0 {
		fmt.Println("Error: -stop-min-rounds must not be negative")
		os.Exit(1)
	}
	if *stopEvery < 1 {
		fmt.Println("Error: -stop-every must be at least 1")
		os.Exit(1)
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if *bot == "model" && apiKey == "" {
		fmt.Println("Error: OPENROUTER_API_KEY is not set in .env or system environment.")
		os.Exit(1)
	}
	if *fallback != "basic" && *fallback != "stand" {
		fmt.Println("Error: -fallback must be 'basic' or 'stand'")
		os.Exit(1)
	}
	if *startBankroll < *minBet {
		fmt.Println("Error: -bankroll must be at least -min-bet")
		os.Exit(1)
	}

	rules := Rules{
		Decks:            *decks,
		Penetration:      *penetration,
		BlackjackPays:    payout,
		DealerHitsSoft17: *h17,
		DealerPeeks:      *peek,
		DoubleAllowed:    *double,
		DoubleAnyTwo:     *doubleAny,
		DoubleAfterSplit: *das,
		MaxHands:         *maxHands,
		ResplitAces:      false,
		SplitAcesOneCard: true,
		LateSurrender:    *surrender,
		Insurance:        *insurance,
	}
	if err := rules.Validate(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	rng := rand.New(rand.NewSource(*seed))
	var decider Decider
	if *bot == "basic" {
		decider = NewBasicBot(rules, *flatBet)
	} else {
		client := jev.NewClient(apiKey)
		decider = NewModelDecider(client, *model, rules, *timeout, *fallback, *showCount, *games)
	}
	game := NewGame(rules, *startBankroll, *minBet, *unit, decider, rng, *showCount)
	if *evRollouts > 0 {
		game.EnableEV(*evRollouts, rand.New(rand.NewSource(*seed+1)))
	}
	if *stopPolicy == "model" {
		game.EnableStopPolicy(*stopPolicy, *stopOnlyAhead, *stopMinRounds, *stopEvery, *games)
	}

	fmt.Println("=== Blackjack: player vs Dealer ===")
	fmt.Printf("Bot:      %s\n", *bot)
	if *bot == "model" {
		fmt.Printf("Model:    %s\n", *model)
	}
	fmt.Printf("Rules:    %s\n", rules.String())
	fmt.Printf("Bankroll: %.2f units (min bet %.2f, unit %.2f)\n", *startBankroll, *minBet, *unit)
	fmt.Printf("Rounds:   %d\n", *games)
	fmt.Printf("Seed:     %d\n", *seed)
	fmt.Printf("Fallback: %s\n", *fallback)
	fmt.Printf("EV:       %d rollouts per deviation\n", *evRollouts)
	if *stopPolicy == "model" {
		fmt.Printf("Stop:     model decides every %d round(s), only when ahead %v, min rounds %d\n", *stopEvery, *stopOnlyAhead, *stopMinRounds)
	}
	fmt.Printf("Results:  %s\n\n", *results)

	started := time.Now()
	acc := NewAccumulator(*startBankroll)
	var records []RoundRecord
	if !*noRecords {
		records = make([]RoundRecord, 0, min(*games, 100000))
	}
	ctx := context.Background()

	stopped := false
	for i := 1; i <= *games && !game.Ruined(); i++ {
		rec, err := game.PlayRound(ctx)
		if err != nil {
			fmt.Printf("Round %d aborted: %v\n", i, err)
			break
		}
		acc.Add(rec)
		if !*noRecords {
			records = append(records, rec)
		}
		if !*quiet {
			printRound(rec)
		}
		if rec.SessionEnd {
			stopped = true
			break
		}
		if *delay > 0 && i < *games {
			time.Sleep(*delay)
		}
	}

	stopReason := "rounds"
	if game.Ruined() {
		stopReason = "ruined"
	} else if stopped {
		stopReason = "model"
	}

	finished := time.Now()
	player, dealer := acc.Player(), acc.Dealer()
	betting := acc.Betting(game.Ruined())
	decisions := acc.Decisions()

	report := Report{
		SchemaVersion: 3,
		Model:         *model,
		Rules:         rules,
		Config: RunConfig{
			Games:         *games,
			RoundsPlayed:  game.RoundsPlayed(),
			Seed:          *seed,
			StartBankroll: *startBankroll,
			Unit:          *unit,
			MinBet:        *minBet,
			Fallback:      *fallback,
			ShowCount:     *showCount,
			Bot:           *bot,
			EVRollouts:    *evRollouts,
			StopPolicy:    *stopPolicy,
			StopOnlyAhead: *stopOnlyAhead,
			StopMinRounds: *stopMinRounds,
			StopEvery:     *stopEvery,
		},
		StartedAt:  started,
		FinishedAt: finished,
		Player:     player,
		Dealer:     dealer,
		Betting:    betting,
		Decisions:  decisions,
		Summary: Summary{
			Rounds:        game.RoundsPlayed(),
			Net:           betting.Profit,
			FinalBankroll: game.Bankroll(),
			Ruined:        game.Ruined(),
			StoppedEarly:  stopReason == "model",
			StopRound:     game.RoundsPlayed(),
			StopReason:    stopReason,
		},
		Usage:   totalUsage(records),
		Records: records,
	}

	if err := writeReport(*results, report); err != nil {
		fmt.Printf("Warning: could not write results: %v\n", err)
	} else {
		fmt.Printf("Results written to %s\n", *results)
	}

	printStats(report, started, finished)
}

func writeReport(path string, r Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func printRound(rec RoundRecord) {
	fmt.Printf("Round %d | bet %.2f (%s) | bankroll %.2f -> %.2f\n", rec.Round, rec.Bet, rec.BetOption, rec.BankrollBefore, rec.BankrollAfter)
	for _, d := range rec.Decisions {
		switch d.Kind {
		case "bet":
			fmt.Printf("  bet: %s%s\n", d.Action, probLabel(d.Probabilities))
		case "insurance":
			fmt.Printf("  insurance: %s\n", d.Action)
		case "continue":
			fmt.Printf("  session: %s%s\n", strings.ToUpper(d.Action), probLabel(d.Probabilities))
		case "action":
			mark := ""
			if d.Agrees != nil && !*d.Agrees {
				mark = fmt.Sprintf(" [basic: %s]", d.BasicAction)
			}
			fmt.Printf("  action: %s (%d) vs %s -> %s%s%s\n", strings.Join(d.PlayerCards, " "), d.PlayerTotal, d.DealerUp, strings.ToUpper(d.Action), probLabel(d.Probabilities), mark)
		}
	}
	for i, h := range rec.Hands {
		label := fmt.Sprintf("hand %d", i+1)
		if len(rec.Hands) == 1 {
			label = "player"
		}
		fmt.Printf("  %s: %s (%d%s) -> %s\n", label, strings.Join(h.Cards, " "), h.Total, softMark(h.Soft), h.Outcome)
	}
	fmt.Printf("  dealer: %s (%d)%s\n", strings.Join(rec.DealerCards, " "), rec.DealerTotal, bjMark(rec.DealerBlackjack))
	fmt.Printf("  net %+.2f | %s\n\n", rec.Net, rec.Outcome)
}

func probLabel(probs map[string]float64) string {
	if len(probs) == 0 {
		return ""
	}
	keys := []string{"hit", "stand", "double", "split", "surrender", "bet_5", "bet_10", "bet_50", "all_in", "stop", "continue"}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if p, ok := probs[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%.0f%%", k, p*100))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "  [" + strings.Join(parts, " ") + "]"
}

func softMark(soft bool) string {
	if soft {
		return ", soft"
	}
	return ""
}

func bjMark(bj bool) string {
	if bj {
		return " blackjack"
	}
	return ""
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}

func printStats(r Report, started, finished time.Time) {
	p, d, b, dec := r.Player, r.Dealer, r.Betting, r.Decisions

	fmt.Println("\n=== Player vs Dealer ===")
	fmt.Printf("Player: %d hands | W %.1f%% L %.1f%% P %.1f%% | blackjack %.1f%% | bust %.1f%% | avg total %.2f (min %d, max %d)\n",
		p.Hands, pct(p.Wins, p.Hands), pct(p.Losses, p.Hands), pct(p.Pushes, p.Hands), pct(p.Blackjacks, p.Hands), pct(p.Busts, p.Hands), p.AvgTotal, p.MinTotal, p.MaxTotal)
	fmt.Printf("Dealer: %d rounds | W %.1f%% L %.1f%% P %.1f%% | blackjack %.1f%% | bust %.1f%% | avg total %.2f (min %d, max %d)\n",
		d.Hands, pct(d.Wins, d.Hands), pct(d.Losses, d.Hands), pct(d.Pushes, d.Hands), pct(d.Blackjacks, d.Hands), pct(d.Busts, d.Hands), d.AvgTotal, d.MinTotal, d.MaxTotal)

	fmt.Println("\n=== Bankroll ===")
	fmt.Printf("Start %.2f -> End %.2f | profit %+.2f | wagered %.2f | RTP %.2f%% | house edge %.2f%% (95%% CI %.2f%%..%.2f%%, n_eff %.1f)\n",
		b.StartBankroll, b.EndBankroll, b.Profit, b.TotalWagered, b.RTP*100, b.HouseEdge*100, b.EdgeCI95[0]*100, b.EdgeCI95[1]*100, b.EdgeN)
	fmt.Printf("Avg bet %.2f | peak %.2f | low %.2f | max drawdown %.2f | ruined %v | rounds %d\n",
		b.AvgBet, b.PeakBankroll, b.MinBankroll, b.MaxDrawdown, b.Ruined, b.Rounds)
	fmt.Printf("Bet mix: bet_5 %d, bet_10 %d, bet_50 %d, all_in %d\n",
		b.Mix["bet_5"], b.Mix["bet_10"], b.Mix["bet_50"], b.Mix["all_in"])
	for _, opt := range []string{"bet_5", "bet_10", "bet_50", "all_in"} {
		if n := b.RoundsByOption[opt]; n > 0 {
			fmt.Printf("  %-7s profit %+8.2f over %4d rounds | win rate %.1f%%\n", opt, b.ProfitByOption[opt], n, b.WinRateByOption[opt]*100)
		}
	}

	fmt.Println("\n=== Decisions ===")
	fmt.Printf("Total %d | actions: hit %d, stand %d, double %d, split %d, surrender %d | fallbacks %d | errors %d\n",
		dec.Total, dec.ByAction["hit"], dec.ByAction["stand"], dec.ByAction["double"], dec.ByAction["split"], dec.ByAction["surrender"], dec.Fallbacks, dec.Errors)
	fmt.Printf("Avg confidence %.3f | Brier %.3f (n=%d) | basic-strategy agreement %.1f%% (%d/%d) | avg latency %.0f ms\n",
		dec.AvgConfidence, dec.BrierScore, dec.BrierN, dec.AgreementRate*100, dec.AgreementCount, dec.AgreementChecked, dec.AvgLatencyMs)
	if dec.EVChecked > 0 {
		fmt.Printf("EV lost to deviations: %.4f units over %d checked decisions (%.4f per decision)\n",
			dec.EVLossTotal, dec.EVChecked, dec.EVLossPerChecked)
	}

	fmt.Println("\n=== Usage ===")
	fmt.Printf("API calls %d | input tokens %d | output tokens %d | cost $%.6f | avg latency %.0f ms | wall time %s\n",
		r.Usage.Calls, r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.Cost, dec.AvgLatencyMs, finished.Sub(started).Round(time.Second))

	if r.Ruined() {
		fmt.Println("\nThe model went broke: bankroll fell below the minimum bet.")
	}
	if r.Summary.StopReason == "model" {
		fmt.Printf("\nThe model ended the session after round %d with a bankroll of %.2f (%+.2f).\n",
			r.Summary.StopRound, r.Summary.FinalBankroll, r.Summary.Net)
		fmt.Println("Note: early-stopped sessions are not comparable to fixed-horizon runs.")
	}
}

func (r Report) Ruined() bool { return r.Summary.Ruined }
