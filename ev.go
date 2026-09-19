package main

import (
	"math"
	"math/rand"
)

type evEstimator struct {
	rules    Rules
	rng      *rand.Rand
	rollouts int
}

type simHandResult struct {
	cards       Hand
	wagered     float64
	fromSplit   bool
	surrendered bool
}

func (e *evEstimator) estimate(player Hand, up Card, remaining []Card, action Action) float64 {
	if e == nil || e.rollouts <= 0 || len(remaining) < 20 {
		return math.NaN()
	}
	sum := 0.0
	n := 0
	for i := 0; i < e.rollouts; i++ {
		v, ok := simulateRound(e.rules, player, up, remaining, e.rng, action)
		if !ok {
			continue
		}
		sum += v
		n++
	}
	if n == 0 {
		return math.NaN()
	}
	return sum / float64(n)
}

func simulateRound(rules Rules, player Hand, up Card, remaining []Card, rng *rand.Rand, action Action) (float64, bool) {
	if action == ActionSurrender {
		return -0.5, true
	}

	deck := make([]Card, len(remaining))
	copy(deck, remaining)
	rng.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	pos := 0
	ok := true
	draw := func() Card {
		if pos >= len(deck) {
			ok = false
			return Card{Rank: "10", Suit: "S"}
		}
		c := deck[pos]
		pos++
		return c
	}

	dealer := Hand{up}
	for {
		dealer = Hand{up, draw()}
		if !ok {
			return 0, false
		}
		if !rules.DealerPeeks {
			break
		}
		if up.Value() != 10 && up.Value() != 11 {
			break
		}
		if !dealer.Blackjack() {
			break
		}
	}
	hands := make([]simHandResult, 0, 4)
	splitsLeft := rules.MaxHands - 1

	switch action {
	case ActionStand:
		hands = append(hands, simHandResult{cards: player, wagered: 1})
	case ActionHit:
		cards := append(Hand{}, player...)
		cards = append(cards, draw())
		simPlayHand(rules, cards, up, 1, false, false, splitsLeft, draw, &hands)
	case ActionDouble:
		cards := append(Hand{}, player...)
		cards = append(cards, draw())
		hands = append(hands, simHandResult{cards: cards, wagered: 2})
	case ActionSplit:
		if len(player) != 2 {
			return 0, false
		}
		c1, c2 := player[0], player[1]
		ace := c1.Rank == "A"
		h1 := Hand{c1, draw()}
		h2 := Hand{c2, draw()}
		simPlayHand(rules, h1, up, 1, true, ace, splitsLeft-1, draw, &hands)
		simPlayHand(rules, h2, up, 1, true, ace, splitsLeft-1, draw, &hands)
	default:
		hands = append(hands, simHandResult{cards: player, wagered: 1})
	}
	if !ok {
		return 0, false
	}

	dealerPlays := false
	if !dealer.Blackjack() {
		for _, h := range hands {
			if h.surrendered || h.cards.Bust() {
				continue
			}
			if len(h.cards) == 2 && h.cards.Total() == 21 && !h.fromSplit {
				continue
			}
			dealerPlays = true
			break
		}
	}
	if dealerPlays {
		for dealerShouldHit(dealer, rules) {
			dealer = append(dealer, draw())
		}
	}
	if !ok {
		return 0, false
	}

	net := 0.0
	for _, h := range hands {
		ph := &playerHand{cards: h.cards, bet: h.wagered, wagered: h.wagered, fromSplit: h.fromSplit, surrendered: h.surrendered}
		net += resolveHand(ph, dealer, rules) - h.wagered
	}
	return net, true
}

func simPlayHand(rules Rules, cards Hand, up Card, wagered float64, fromSplit, splitAces bool, splitsLeft int, draw func() Card, out *[]simHandResult) {
	for {
		total := cards.Total()
		if total >= 21 {
			break
		}
		if splitAces && rules.SplitAcesOneCard {
			break
		}

		canDouble := len(cards) == 2 && rules.DoubleAllowed
		if canDouble && !rules.DoubleAnyTwo {
			canDouble = total >= 9 && total <= 11
		}
		if fromSplit && !rules.DoubleAfterSplit {
			canDouble = false
		}
		canSplit := len(cards) == 2 && cards.Pair() && splitsLeft > 0
		if splitAces && !rules.ResplitAces {
			canSplit = false
		}

		switch BasicStrategy(rules, cards, up, canDouble, canSplit) {
		case ActionHit:
			cards = append(cards, draw())
		case ActionDouble:
			wagered++
			cards = append(cards, draw())
			*out = append(*out, simHandResult{cards: cards, wagered: wagered, fromSplit: fromSplit})
			return
		case ActionSplit:
			c1, c2 := cards[0], cards[1]
			ace := c1.Rank == "A"
			h1 := Hand{c1, draw()}
			h2 := Hand{c2, draw()}
			simPlayHand(rules, h1, up, 1, true, ace, splitsLeft-1, draw, out)
			simPlayHand(rules, h2, up, 1, true, ace, splitsLeft-1, draw, out)
			return
		case ActionSurrender:
			*out = append(*out, simHandResult{cards: cards, wagered: wagered, fromSplit: fromSplit, surrendered: true})
			return
		default:
			*out = append(*out, simHandResult{cards: cards, wagered: wagered, fromSplit: fromSplit})
			return
		}
	}
	*out = append(*out, simHandResult{cards: cards, wagered: wagered, fromSplit: fromSplit})
}
