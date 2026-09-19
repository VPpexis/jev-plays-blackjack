package main

import (
	"math"
	"strconv"
	"time"
)

type SideStats struct {
	Hands             int            `json:"hands"`
	Wins              int            `json:"wins"`
	Losses            int            `json:"losses"`
	Pushes            int            `json:"pushes"`
	Blackjacks        int            `json:"blackjacks"`
	Busts             int            `json:"busts"`
	AvgTotal          float64        `json:"avg_total"`
	MinTotal          int            `json:"min_total"`
	MaxTotal          int            `json:"max_total"`
	TotalDistribution map[string]int `json:"total_distribution"`
}

type BettingStats struct {
	Rounds          int                `json:"rounds"`
	StartBankroll   float64            `json:"start_bankroll"`
	EndBankroll     float64            `json:"end_bankroll"`
	PeakBankroll    float64            `json:"peak_bankroll"`
	MinBankroll     float64            `json:"min_bankroll"`
	Profit          float64            `json:"profit"`
	TotalWagered    float64            `json:"total_wagered"`
	RTP             float64            `json:"rtp"`
	HouseEdge       float64            `json:"house_edge"`
	EdgeStdErr      float64            `json:"edge_std_err"`
	EdgeCI95        [2]float64         `json:"edge_ci95"`
	EdgeN           float64            `json:"edge_effective_n"`
	AvgBet          float64            `json:"avg_bet"`
	AvgBetPct       float64            `json:"avg_bet_pct"`
	MaxDrawdown     float64            `json:"max_drawdown"`
	Ruined          bool               `json:"ruined"`
	Mix             map[string]int     `json:"bet_mix"`
	ProfitByOption  map[string]float64 `json:"profit_by_option"`
	RoundsByOption  map[string]int     `json:"rounds_by_option"`
	WinsByOption    map[string]int     `json:"wins_by_option"`
	WinRateByOption map[string]float64 `json:"win_rate_by_option"`
	AllInRounds     int                `json:"all_in_rounds"`
	AllInWins       int                `json:"all_in_wins"`
}

type DecisionStats struct {
	Total            int            `json:"total"`
	ByKind           map[string]int `json:"by_kind"`
	ByAction         map[string]int `json:"by_action"`
	Fallbacks        int            `json:"fallbacks"`
	Errors           int            `json:"errors"`
	AvgConfidence    float64        `json:"avg_confidence"`
	BrierScore       float64        `json:"brier_score"`
	BrierN           int            `json:"brier_n"`
	AgreementChecked int            `json:"agreement_checked"`
	AgreementCount   int            `json:"agreement_count"`
	AgreementRate    float64        `json:"agreement_rate"`
	AvgLatencyMs     float64        `json:"avg_latency_ms"`
	EVChecked        int            `json:"ev_checked"`
	EVLossTotal      float64        `json:"ev_loss_total"`
	EVLossPerChecked float64        `json:"ev_loss_per_checked"`
}

type RunConfig struct {
	Games         int     `json:"games"`
	RoundsPlayed  int     `json:"rounds_played"`
	Seed          int64   `json:"seed"`
	StartBankroll float64 `json:"start_bankroll"`
	Unit          float64 `json:"unit"`
	MinBet        float64 `json:"min_bet"`
	Fallback      string  `json:"fallback"`
	ShowCount     bool    `json:"show_count"`
	Bot           string  `json:"bot"`
	EVRollouts    int     `json:"ev_rollouts"`
	StopPolicy    string  `json:"stop_policy"`
	StopOnlyAhead bool    `json:"stop_only_ahead"`
	StopMinRounds int     `json:"stop_min_rounds"`
	StopEvery     int     `json:"stop_every"`
}

type Summary struct {
	Rounds        int     `json:"rounds"`
	Net           float64 `json:"net"`
	FinalBankroll float64 `json:"final_bankroll"`
	Ruined        bool    `json:"ruined"`
	StoppedEarly  bool    `json:"stopped_early"`
	StopRound     int     `json:"stop_round"`
	StopReason    string  `json:"stop_reason"`
}

type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Model         string        `json:"model"`
	Rules         Rules         `json:"rules"`
	Config        RunConfig     `json:"config"`
	StartedAt     time.Time     `json:"started_at"`
	FinishedAt    time.Time     `json:"finished_at"`
	Player        SideStats     `json:"player"`
	Dealer        SideStats     `json:"dealer"`
	Betting       BettingStats  `json:"betting"`
	Decisions     DecisionStats `json:"decisions"`
	Summary       Summary       `json:"summary"`
	Usage         CallUsage     `json:"usage"`
	Records       []RoundRecord `json:"records,omitempty"`
}

func bucket(total int) string {
	if total > 21 {
		return "bust"
	}
	return strconv.Itoa(total)
}

type playerAcc struct {
	hands      int
	wins       int
	losses     int
	pushes     int
	blackjacks int
	busts      int
	sumTotal   int
	minTotal   int
	maxTotal   int
	dist       map[string]int
	started    bool
}

func (a *playerAcc) add(h HandRecord) {
	if !a.started {
		a.dist = map[string]int{}
		a.minTotal = h.Total
		a.maxTotal = h.Total
		a.started = true
	}
	a.hands++
	switch h.Outcome {
	case "WIN":
		a.wins++
	case "LOSS", "SURRENDER":
		a.losses++
	case "PUSH":
		a.pushes++
	}
	if h.Blackjack {
		a.blackjacks++
	}
	if h.Bust {
		a.busts++
	}
	a.sumTotal += h.Total
	if h.Total < a.minTotal {
		a.minTotal = h.Total
	}
	if h.Total > a.maxTotal {
		a.maxTotal = h.Total
	}
	a.dist[bucket(h.Total)]++
}

func (a *playerAcc) addRound(r RoundRecord) {
	for _, h := range r.Hands {
		a.add(h)
	}
}

func (a *playerAcc) result() SideStats {
	s := SideStats{
		Hands:             a.hands,
		Wins:              a.wins,
		Losses:            a.losses,
		Pushes:            a.pushes,
		Blackjacks:        a.blackjacks,
		Busts:             a.busts,
		MinTotal:          a.minTotal,
		MaxTotal:          a.maxTotal,
		TotalDistribution: a.dist,
	}
	if s.TotalDistribution == nil {
		s.TotalDistribution = map[string]int{}
	}
	if a.hands > 0 {
		s.AvgTotal = float64(a.sumTotal) / float64(a.hands)
	}
	return s
}

type dealerAcc struct {
	rounds     int
	wins       int
	losses     int
	pushes     int
	blackjacks int
	busts      int
	sumTotal   int
	minTotal   int
	maxTotal   int
	dist       map[string]int
	started    bool
}

func (a *dealerAcc) add(r RoundRecord) {
	if !a.started {
		a.dist = map[string]int{}
		a.minTotal = r.DealerTotal
		a.maxTotal = r.DealerTotal
		a.started = true
	}
	a.rounds++
	if r.DealerBlackjack {
		a.blackjacks++
	}
	if r.DealerBust {
		a.busts++
	}
	a.sumTotal += r.DealerTotal
	if r.DealerTotal < a.minTotal {
		a.minTotal = r.DealerTotal
	}
	if r.DealerTotal > a.maxTotal {
		a.maxTotal = r.DealerTotal
	}
	a.dist[bucket(r.DealerTotal)]++
	for _, h := range r.Hands {
		switch h.Outcome {
		case "WIN":
			a.losses++
		case "LOSS", "SURRENDER":
			a.wins++
		case "PUSH":
			a.pushes++
		}
	}
}

func (a *dealerAcc) result() SideStats {
	s := SideStats{
		Hands:             a.rounds,
		Wins:              a.wins,
		Losses:            a.losses,
		Pushes:            a.pushes,
		Blackjacks:        a.blackjacks,
		Busts:             a.busts,
		MinTotal:          a.minTotal,
		MaxTotal:          a.maxTotal,
		TotalDistribution: a.dist,
	}
	if s.TotalDistribution == nil {
		s.TotalDistribution = map[string]int{}
	}
	if a.rounds > 0 {
		s.AvgTotal = float64(a.sumTotal) / float64(a.rounds)
	}
	return s
}

type betAcc struct {
	rounds       int
	totalWagered float64
	profit       float64
	sw           float64
	swx          float64
	swxx         float64
	sww          float64
	mix          map[string]int
	profitBy     map[string]float64
	roundsBy     map[string]int
	winsBy       map[string]int
	allInRounds  int
	allInWins    int
	peak         float64
	low          float64
	maxDD        float64
	started      bool
}

func (a *betAcc) add(r RoundRecord, start float64) {
	if !a.started {
		a.mix = map[string]int{}
		a.profitBy = map[string]float64{}
		a.roundsBy = map[string]int{}
		a.winsBy = map[string]int{}
		a.peak = start
		a.low = start
		a.started = true
	}
	a.rounds++
	roundWagered := r.Bet
	if len(r.Hands) > 0 {
		sum := 0.0
		for _, h := range r.Hands {
			sum += h.Wagered
		}
		if sum > 0 {
			roundWagered = sum
		}
	}
	a.totalWagered += roundWagered
	a.profit += r.Net
	a.mix[r.BetOption]++
	a.roundsBy[r.BetOption]++
	a.profitBy[r.BetOption] += r.Net
	if r.Net > 0 {
		a.winsBy[r.BetOption]++
	}
	if r.BetOption == "all_in" {
		a.allInRounds++
		if r.Net > 0 {
			a.allInWins++
		}
	}
	if roundWagered > 0 {
		w := roundWagered
		x := -r.Net / roundWagered
		a.sw += w
		a.swx += w * x
		a.swxx += w * x * x
		a.sww += w * w
	}
	if r.BankrollAfter > a.peak {
		a.peak = r.BankrollAfter
	}
	if r.BankrollAfter < a.low {
		a.low = r.BankrollAfter
	}
	if dd := a.peak - r.BankrollAfter; dd > a.maxDD {
		a.maxDD = dd
	}
}

func (a *betAcc) result(start float64, end float64, ruined bool) BettingStats {
	s := BettingStats{
		Rounds:          a.rounds,
		StartBankroll:   start,
		EndBankroll:     end,
		PeakBankroll:    a.peak,
		MinBankroll:     a.low,
		Profit:          a.profit,
		TotalWagered:    a.totalWagered,
		MaxDrawdown:     a.maxDD,
		Ruined:          ruined,
		Mix:             a.mix,
		ProfitByOption:  a.profitBy,
		RoundsByOption:  a.roundsBy,
		WinsByOption:    a.winsBy,
		WinRateByOption: map[string]float64{},
		AllInRounds:     a.allInRounds,
		AllInWins:       a.allInWins,
	}
	if s.Mix == nil {
		s.Mix = map[string]int{}
	}
	if s.ProfitByOption == nil {
		s.ProfitByOption = map[string]float64{}
	}
	if s.RoundsByOption == nil {
		s.RoundsByOption = map[string]int{}
	}
	if s.WinsByOption == nil {
		s.WinsByOption = map[string]int{}
	}
	if a.rounds == 0 {
		s.PeakBankroll = start
		s.MinBankroll = start
	}
	if s.TotalWagered > 0 {
		s.RTP = (s.TotalWagered + s.Profit) / s.TotalWagered
		s.HouseEdge = -s.Profit / s.TotalWagered
	}
	if a.rounds > 0 {
		s.AvgBet = s.TotalWagered / float64(a.rounds)
	}
	if a.sw > 0 && a.sww > 0 {
		nEff := a.sw * a.sw / a.sww
		denom := a.sw - a.sww/a.sw
		if denom > 0 {
			mean := a.swx / a.sw
			variance := (a.swxx - 2*mean*a.swx + mean*mean*a.sw) / denom
			if variance < 0 {
				variance = 0
			}
			se := math.Sqrt(variance / nEff)
			s.EdgeStdErr = se
			s.EdgeN = nEff
			s.EdgeCI95 = [2]float64{s.HouseEdge - 1.96*se, s.HouseEdge + 1.96*se}
		}
	}
	for opt, n := range s.RoundsByOption {
		if n > 0 {
			s.WinRateByOption[opt] = float64(s.WinsByOption[opt]) / float64(n)
		}
	}
	return s
}

type decAcc struct {
	total            int
	byKind           map[string]int
	byAction         map[string]int
	fallbacks        int
	errors           int
	confSum          float64
	confN            int
	brierSum         float64
	brierN           int
	agreementChecked int
	agreementCount   int
	latencySum       int64
	evChecked        int
	evLossSum        float64
}

func (a *decAcc) add(r RoundRecord) {
	if a.byKind == nil {
		a.byKind = map[string]int{}
		a.byAction = map[string]int{}
	}
	for _, d := range r.Decisions {
		a.total++
		a.byKind[d.Kind]++
		a.byAction[d.Action]++
		a.latencySum += d.LatencyMs
		if d.Fallback {
			a.fallbacks++
		}
		if d.Error != "" {
			a.errors++
		}
		if d.Confidence > 0 {
			a.confSum += d.Confidence
			a.confN++
		}
		if d.Agrees != nil {
			a.agreementChecked++
			if *d.Agrees {
				a.agreementCount++
			}
			if d.Confidence > 0 {
				correct := 0.0
				if *d.Agrees {
					correct = 1
				}
				a.brierSum += (d.Confidence - correct) * (d.Confidence - correct)
				a.brierN++
			}
		}
		if d.EVChecked {
			a.evChecked++
			a.evLossSum += d.EVLoss
		}
	}
}

func (a *decAcc) result() DecisionStats {
	s := DecisionStats{
		Total:            a.total,
		ByKind:           a.byKind,
		ByAction:         a.byAction,
		Fallbacks:        a.fallbacks,
		Errors:           a.errors,
		BrierN:           a.brierN,
		AgreementChecked: a.agreementChecked,
		AgreementCount:   a.agreementCount,
		EVChecked:        a.evChecked,
		EVLossTotal:      a.evLossSum,
	}
	if s.ByKind == nil {
		s.ByKind = map[string]int{}
	}
	if s.ByAction == nil {
		s.ByAction = map[string]int{}
	}
	if a.confN > 0 {
		s.AvgConfidence = a.confSum / float64(a.confN)
	}
	if a.brierN > 0 {
		s.BrierScore = a.brierSum / float64(a.brierN)
	}
	if a.agreementChecked > 0 {
		s.AgreementRate = float64(a.agreementCount) / float64(a.agreementChecked)
	}
	if a.total > 0 {
		s.AvgLatencyMs = float64(a.latencySum) / float64(a.total)
	}
	if a.evChecked > 0 {
		s.EVLossPerChecked = a.evLossSum / float64(a.evChecked)
	}
	return s
}

type Accumulator struct {
	player  playerAcc
	dealer  dealerAcc
	bet     betAcc
	dec     decAcc
	start   float64
	lastEnd float64
}

func NewAccumulator(start float64) *Accumulator {
	return &Accumulator{start: start, lastEnd: start}
}

func (a *Accumulator) Add(r RoundRecord) {
	a.player.addRound(r)
	a.dealer.add(r)
	a.bet.add(r, a.start)
	a.dec.add(r)
	a.lastEnd = r.BankrollAfter
}

func (a *Accumulator) Player() SideStats { return a.player.result() }

func (a *Accumulator) Dealer() SideStats { return a.dealer.result() }

func (a *Accumulator) Betting(ruined bool) BettingStats {
	return a.bet.result(a.start, a.lastEnd, ruined)
}

func (a *Accumulator) Decisions() DecisionStats { return a.dec.result() }

func aggregateSides(records []RoundRecord) (SideStats, SideStats) {
	acc := NewAccumulator(0)
	for _, r := range records {
		acc.player.addRound(r)
		acc.dealer.add(r)
	}
	return acc.Player(), acc.Dealer()
}

func aggregateBetting(records []RoundRecord, start float64, ruined bool) BettingStats {
	acc := NewAccumulator(start)
	for _, r := range records {
		acc.bet.add(r, start)
	}
	return acc.Betting(ruined)
}

func aggregateDecisions(records []RoundRecord) DecisionStats {
	acc := NewAccumulator(0)
	for _, r := range records {
		acc.dec.add(r)
	}
	return acc.Decisions()
}

func totalUsage(records []RoundRecord) CallUsage {
	var u CallUsage
	for _, r := range records {
		for _, d := range r.Decisions {
			u.Add(d.Usage)
		}
	}
	return u
}
