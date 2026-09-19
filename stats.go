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
}

type Summary struct {
	Rounds        int     `json:"rounds"`
	Net           float64 `json:"net"`
	FinalBankroll float64 `json:"final_bankroll"`
	Ruined        bool    `json:"ruined"`
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
	Records       []RoundRecord `json:"records"`
}

func bucket(total int) string {
	if total > 21 {
		return "bust"
	}
	return strconv.Itoa(total)
}

func aggregateSides(records []RoundRecord) (SideStats, SideStats) {
	p := SideStats{TotalDistribution: map[string]int{}}
	d := SideStats{TotalDistribution: map[string]int{}}
	d.MinTotal = 99
	pMin := 99
	var pSum, dSum int

	for _, r := range records {
		d.Hands++
		if r.DealerBlackjack {
			d.Blackjacks++
		}
		if r.DealerBust {
			d.Busts++
		}
		dSum += r.DealerTotal
		if r.DealerTotal < d.MinTotal {
			d.MinTotal = r.DealerTotal
		}
		if r.DealerTotal > d.MaxTotal {
			d.MaxTotal = r.DealerTotal
		}
		d.TotalDistribution[bucket(r.DealerTotal)]++

		for _, h := range r.Hands {
			p.Hands++
			switch h.Outcome {
			case "WIN":
				p.Wins++
				d.Losses++
			case "LOSS", "SURRENDER":
				p.Losses++
				d.Wins++
			case "PUSH":
				p.Pushes++
				d.Pushes++
			}
			if h.Blackjack {
				p.Blackjacks++
			}
			if h.Bust {
				p.Busts++
			}
			pSum += h.Total
			if h.Total < pMin {
				pMin = h.Total
			}
			if h.Total > p.MaxTotal {
				p.MaxTotal = h.Total
			}
			p.TotalDistribution[bucket(h.Total)]++
		}
	}

	if p.Hands > 0 {
		p.AvgTotal = float64(pSum) / float64(p.Hands)
	}
	if d.Hands > 0 {
		d.AvgTotal = float64(dSum) / float64(d.Hands)
	}
	if pMin == 99 {
		pMin = 0
	}
	if d.MinTotal == 99 {
		d.MinTotal = 0
	}
	p.MinTotal = pMin
	return p, d
}

func aggregateBetting(records []RoundRecord, start float64, ruined bool) BettingStats {
	s := BettingStats{
		StartBankroll:   start,
		MinBankroll:     start,
		Mix:             map[string]int{},
		ProfitByOption:  map[string]float64{},
		RoundsByOption:  map[string]int{},
		WinsByOption:    map[string]int{},
		WinRateByOption: map[string]float64{},
	}

	peak := start
	var edgeSum, edgeSumSq float64
	edgeN := 0

	for _, r := range records {
		s.Rounds++
		s.TotalWagered += r.Bet
		s.Mix[r.BetOption]++
		s.RoundsByOption[r.BetOption]++
		s.ProfitByOption[r.BetOption] += r.Net
		if r.Net > 0 {
			s.WinsByOption[r.BetOption]++
		}
		if r.BetOption == "all_in" {
			s.AllInRounds++
			if r.Net > 0 {
				s.AllInWins++
			}
		}
		if r.Bet > 0 {
			x := -r.Net / r.Bet
			edgeSum += x
			edgeSumSq += x * x
			edgeN++
		}
		if r.BankrollAfter > peak {
			peak = r.BankrollAfter
		}
		if r.BankrollAfter < s.MinBankroll {
			s.MinBankroll = r.BankrollAfter
		}
		if dd := peak - r.BankrollAfter; dd > s.MaxDrawdown {
			s.MaxDrawdown = dd
		}
	}

	if len(records) > 0 {
		s.EndBankroll = records[len(records)-1].BankrollAfter
	} else {
		s.EndBankroll = start
	}
	s.PeakBankroll = peak
	s.Profit = s.EndBankroll - start
	s.Ruined = ruined
	if s.TotalWagered > 0 {
		s.RTP = (s.TotalWagered + s.Profit) / s.TotalWagered
		s.HouseEdge = -s.Profit / s.TotalWagered
	}
	if s.Rounds > 0 {
		s.AvgBet = s.TotalWagered / float64(s.Rounds)
	}
	if edgeN > 0 {
		mean := edgeSum / float64(edgeN)
		variance := edgeSumSq/float64(edgeN) - mean*mean
		if variance < 0 {
			variance = 0
		}
		se := math.Sqrt(variance) / math.Sqrt(float64(edgeN))
		s.EdgeStdErr = se
		s.EdgeCI95 = [2]float64{mean - 1.96*se, mean + 1.96*se}
	}
	for opt, n := range s.RoundsByOption {
		if n > 0 {
			s.WinRateByOption[opt] = float64(s.WinsByOption[opt]) / float64(n)
		}
	}
	return s
}

func aggregateDecisions(records []RoundRecord) DecisionStats {
	s := DecisionStats{ByKind: map[string]int{}, ByAction: map[string]int{}}
	var confSum float64
	var confN int
	var brierSum float64
	var latencySum int64

	for _, r := range records {
		for _, d := range r.Decisions {
			s.Total++
			s.ByKind[d.Kind]++
			s.ByAction[d.Action]++
			latencySum += d.LatencyMs
			if d.Fallback {
				s.Fallbacks++
			}
			if d.Error != "" {
				s.Errors++
			}
			if d.Confidence > 0 {
				confSum += d.Confidence
				confN++
			}
			if d.Agrees != nil {
				s.AgreementChecked++
				if *d.Agrees {
					s.AgreementCount++
				}
				if d.Confidence > 0 {
					correct := 0.0
					if *d.Agrees {
						correct = 1
					}
					brierSum += (d.Confidence - correct) * (d.Confidence - correct)
					s.BrierN++
				}
			}
		}
	}

	if confN > 0 {
		s.AvgConfidence = confSum / float64(confN)
	}
	if s.BrierN > 0 {
		s.BrierScore = brierSum / float64(s.BrierN)
	}
	if s.AgreementChecked > 0 {
		s.AgreementRate = float64(s.AgreementCount) / float64(s.AgreementChecked)
	}
	if s.Total > 0 {
		s.AvgLatencyMs = float64(latencySum) / float64(s.Total)
	}
	return s
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
