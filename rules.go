package main

import "fmt"

type Action string

const (
	ActionHit       Action = "hit"
	ActionStand     Action = "stand"
	ActionDouble    Action = "double"
	ActionSplit     Action = "split"
	ActionSurrender Action = "surrender"
)

type Rules struct {
	Decks            int     `json:"decks"`
	Penetration      float64 `json:"penetration"`
	BlackjackPays    float64 `json:"blackjack_pays"`
	DealerHitsSoft17 bool    `json:"dealer_hits_soft_17"`
	DealerPeeks      bool    `json:"dealer_peeks"`
	DoubleAllowed    bool    `json:"double_allowed"`
	DoubleAnyTwo     bool    `json:"double_any_two"`
	DoubleAfterSplit bool    `json:"double_after_split"`
	MaxHands         int     `json:"max_hands"`
	ResplitAces      bool    `json:"resplit_aces"`
	SplitAcesOneCard bool    `json:"split_aces_one_card"`
	LateSurrender    bool    `json:"late_surrender"`
	Insurance        bool    `json:"insurance"`
}

func (r Rules) String() string {
	pay := "3:2"
	switch {
	case r.BlackjackPays == 1.2:
		pay = "6:5"
	case r.BlackjackPays == 1.0:
		pay = "1:1"
	}
	dealer := "stands on soft 17"
	if r.DealerHitsSoft17 {
		dealer = "hits soft 17"
	}
	dbl := "double on any two cards"
	switch {
	case !r.DoubleAllowed:
		dbl = "no doubling"
	case !r.DoubleAnyTwo:
		dbl = "double on 9-11 only"
	}
	das := "double after split allowed"
	if !r.DoubleAfterSplit {
		das = "no double after split"
	}
	peek := "dealer peeks for blackjack"
	if !r.DealerPeeks {
		peek = "no dealer peek"
	}
	return fmt.Sprintf("%d-deck shoe, dealer %s, blackjack pays %s, %s, %s, %s, up to %d hands after splits", r.Decks, dealer, pay, dbl, das, peek, r.MaxHands)
}

func (r Rules) Validate() error {
	if r.Decks < 1 || r.Decks > 8 {
		return fmt.Errorf("decks must be between 1 and 8, got %d", r.Decks)
	}
	if r.Penetration <= 0 || r.Penetration > 1 {
		return fmt.Errorf("penetration must be in (0,1], got %v", r.Penetration)
	}
	if r.BlackjackPays <= 0 {
		return fmt.Errorf("blackjack payout must be positive, got %v", r.BlackjackPays)
	}
	if r.MaxHands < 1 {
		return fmt.Errorf("max hands must be at least 1, got %d", r.MaxHands)
	}
	return nil
}

func ParseBlackjackPayout(s string) (float64, error) {
	switch s {
	case "3:2":
		return 1.5, nil
	case "6:5":
		return 1.2, nil
	case "1:1":
		return 1.0, nil
	}
	return 0, fmt.Errorf("invalid blackjack payout %q (use 3:2, 6:5 or 1:1)", s)
}

func dblOr(canDouble bool, fallback Action) Action {
	if canDouble {
		return ActionDouble
	}
	return fallback
}

func BasicStrategy(r Rules, hand Hand, up Card, canDouble, canSplit bool) Action {
	upVal := up.Value()
	total := hand.Total()

	if r.LateSurrender && len(hand) == 2 {
		if total == 16 && (upVal == 9 || upVal == 10 || upVal == 11) {
			return ActionSurrender
		}
		if total == 15 && upVal == 10 {
			return ActionSurrender
		}
	}

	if canSplit && hand.Pair() {
		if a := pairStrategy(hand[0].Rank, upVal, r); a != "" {
			return a
		}
	}
	if hand.Soft() {
		return softStrategy(total, upVal, r, canDouble)
	}
	return hardStrategy(total, upVal, r, canDouble)
}

func pairStrategy(rank string, up int, r Rules) Action {
	switch rank {
	case "A":
		return ActionSplit
	case "10", "J", "Q", "K":
		return ""
	case "9":
		if (up >= 2 && up <= 6) || up == 8 || up == 9 {
			return ActionSplit
		}
	case "8":
		return ActionSplit
	case "7":
		if up >= 2 && up <= 7 {
			return ActionSplit
		}
	case "6":
		if r.DoubleAfterSplit {
			if up >= 2 && up <= 6 {
				return ActionSplit
			}
		} else if up >= 3 && up <= 6 {
			return ActionSplit
		}
	case "5":
		return ""
	case "4":
		if r.DoubleAfterSplit && (up == 5 || up == 6) {
			return ActionSplit
		}
	case "3", "2":
		if r.DoubleAfterSplit {
			if up >= 2 && up <= 7 {
				return ActionSplit
			}
		} else if up >= 4 && up <= 7 {
			return ActionSplit
		}
	}
	return ""
}

func softStrategy(total, up int, r Rules, canDouble bool) Action {
	switch {
	case total >= 20:
		return ActionStand
	case total == 19:
		if r.DealerHitsSoft17 && up >= 3 && up <= 6 {
			return dblOr(canDouble, ActionStand)
		}
		return ActionStand
	case total == 18:
		if r.DealerHitsSoft17 {
			if up >= 2 && up <= 6 {
				return dblOr(canDouble, ActionStand)
			}
		} else {
			if up == 2 {
				return ActionStand
			}
			if up >= 3 && up <= 6 {
				return dblOr(canDouble, ActionStand)
			}
		}
		if up == 7 || up == 8 {
			return ActionStand
		}
		return ActionHit
	case total == 17:
		if up >= 3 && up <= 6 {
			return dblOr(canDouble, ActionHit)
		}
		return ActionHit
	case total == 15 || total == 16:
		if up >= 4 && up <= 6 {
			return dblOr(canDouble, ActionHit)
		}
		return ActionHit
	case total == 13 || total == 14:
		if up >= 5 && up <= 6 {
			return dblOr(canDouble, ActionHit)
		}
		return ActionHit
	}
	return ActionHit
}

func hardStrategy(total, up int, r Rules, canDouble bool) Action {
	switch {
	case total >= 17:
		return ActionStand
	case total >= 13:
		if up >= 2 && up <= 6 {
			return ActionStand
		}
		return ActionHit
	case total == 12:
		if up >= 4 && up <= 6 {
			return ActionStand
		}
		return ActionHit
	case total == 11:
		if r.DealerHitsSoft17 {
			return dblOr(canDouble, ActionHit)
		}
		if up >= 2 && up <= 10 {
			return dblOr(canDouble, ActionHit)
		}
		return ActionHit
	case total == 10:
		if up >= 2 && up <= 9 {
			return dblOr(canDouble, ActionHit)
		}
		return ActionHit
	case total == 9:
		if up >= 3 && up <= 6 {
			return dblOr(canDouble, ActionHit)
		}
		return ActionHit
	}
	return ActionHit
}

func containsAction(list []Action, a Action) bool {
	for _, x := range list {
		if x == a {
			return true
		}
	}
	return false
}

func actionDescription(a Action) string {
	switch a {
	case ActionHit:
		return "Draw another card"
	case ActionStand:
		return "Keep the current total and end your turn"
	case ActionDouble:
		return "Double your bet, draw exactly one card, then end your turn"
	case ActionSplit:
		return "Split the pair into two hands, matching your bet on the new hand"
	case ActionSurrender:
		return "Surrender and lose half your bet"
	}
	return string(a)
}
