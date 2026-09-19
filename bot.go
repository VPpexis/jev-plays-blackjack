package main

import "context"

type basicBot struct {
	rules   Rules
	flatBet float64
}

func NewBasicBot(rules Rules, flatBet float64) *basicBot {
	return &basicBot{rules: rules, flatBet: flatBet}
}

func (b *basicBot) ChooseBet(ctx context.Context, req BetRequest) BetResponse {
	flat := b.flatBet
	if flat <= 0 {
		flat = req.MinBet
	}
	return BetResponse{Option: "flat", Flat: flat}
}

func (b *basicBot) ChooseAction(ctx context.Context, req ActionRequest) ActionResponse {
	act := BasicStrategy(b.rules, req.Hand, req.DealerUp, containsAction(req.Legal, ActionDouble), containsAction(req.Legal, ActionSplit))
	if !containsAction(req.Legal, act) {
		act = ActionStand
	}
	return ActionResponse{Action: act}
}

func (b *basicBot) ChooseInsurance(ctx context.Context, req InsuranceRequest) InsuranceResponse {
	return InsuranceResponse{Take: false}
}
