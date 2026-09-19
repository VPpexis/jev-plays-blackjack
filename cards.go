package main

import (
	"math/rand"
	"strconv"
	"strings"
)

var ranks = []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
var suits = []string{"S", "H", "D", "C"}

type Card struct {
	Rank string `json:"rank"`
	Suit string `json:"suit"`
}

func (c Card) String() string { return c.Rank + c.Suit }

func (c Card) Value() int {
	switch c.Rank {
	case "A":
		return 11
	case "K", "Q", "J", "10":
		return 10
	default:
		n, _ := strconv.Atoi(c.Rank)
		return n
	}
}

type Hand []Card

func (h Hand) String() string {
	parts := make([]string, len(h))
	for i, c := range h {
		parts[i] = c.String()
	}
	return strings.Join(parts, " ")
}

func (h Hand) CardStrings() []string {
	out := make([]string, len(h))
	for i, c := range h {
		out[i] = c.String()
	}
	return out
}

func (h Hand) Total() int {
	total, aces := 0, 0
	for _, c := range h {
		total += c.Value()
		if c.Rank == "A" {
			aces++
		}
	}
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	return total
}

func (h Hand) Soft() bool {
	total, aces := 0, 0
	for _, c := range h {
		total += c.Value()
		if c.Rank == "A" {
			aces++
		}
	}
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	return aces > 0 && total <= 21
}

func (h Hand) Blackjack() bool { return len(h) == 2 && h.Total() == 21 }
func (h Hand) Bust() bool      { return h.Total() > 21 }
func (h Hand) Pair() bool      { return len(h) == 2 && h[0].Rank == h[1].Rank }

type Shoe struct {
	cards       []Card
	next        int
	decks       int
	penetration float64
	rng         *rand.Rand
	seen        []Card
}

func NewShoe(decks int, penetration float64, rng *rand.Rand) *Shoe {
	s := &Shoe{decks: decks, penetration: penetration, rng: rng}
	s.build()
	s.Shuffle()
	return s
}

func (s *Shoe) build() {
	s.cards = make([]Card, 0, s.decks*len(ranks)*len(suits))
	for d := 0; d < s.decks; d++ {
		for _, su := range suits {
			for _, r := range ranks {
				s.cards = append(s.cards, Card{Rank: r, Suit: su})
			}
		}
	}
	s.next = 0
	s.seen = nil
}

func (s *Shoe) Shuffle() {
	s.rng.Shuffle(len(s.cards), func(i, j int) { s.cards[i], s.cards[j] = s.cards[j], s.cards[i] })
	s.next = 0
	s.seen = nil
}

func (s *Shoe) NeedsShuffle() bool {
	return float64(s.next) >= float64(len(s.cards))*s.penetration
}

func (s *Shoe) Remaining() int { return len(s.cards) - s.next }

func (s *Shoe) RemainingCards() []Card { return s.cards[s.next:] }

func (s *Shoe) Draw() Card {
	if s.next >= len(s.cards) {
		s.Shuffle()
	}
	c := s.cards[s.next]
	s.next++
	s.seen = append(s.seen, c)
	return c
}

func (s *Shoe) RunningCount() int {
	count := 0
	for _, c := range s.seen {
		count += hiLo(c)
	}
	return count
}

func hiLo(c Card) int {
	switch c.Rank {
	case "2", "3", "4", "5", "6":
		return 1
	case "10", "J", "Q", "K", "A":
		return -1
	default:
		return 0
	}
}

func (s *Shoe) Composition() map[string]int {
	counts := make(map[string]int, len(ranks))
	for _, r := range ranks {
		counts[r] = 0
	}
	for i := s.next; i < len(s.cards); i++ {
		counts[s.cards[i].Rank]++
	}
	return counts
}
