# jev-plays-blackjack

[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev)

A Go client for the [OpenRouter Decisions API](https://openrouter.ai) plus a rule-accurate
blackjack simulator in which the Jev model plays every hand and chooses its own bet size.

Repository: **https://github.com/VPpexis/jev-plays-blackjack**

The model is asked three kinds of questions through the Decisions API:

- **Betting**: how much of the current bankroll to wager (5%, 10%, 50% or all-in).
- **Action**: what to do with the hand (hit, stand, double, split or surrender).
- **Insurance**: whether to take insurance when the dealer shows an ace (optional, off by default).

Every decision, its probabilities, the resulting cards, and the bankroll outcome are recorded
to a JSON report with full statistics.

---

## Features

- **Rule-accurate blackjack engine**: multi-deck shoe with penetration reshuffle, S17/H17,
  configurable blackjack payout, double (any two / 9–11 / off), double after split, splits to
  N hands, split aces, late surrender, and insurance.
- **Money management**: the model bets a percentage of its bankroll each round and can go broke.
- **Basic-strategy benchmark**: a built-in chart drives the fallback on API failure and measures
  how often the model agrees with optimal play.
- **Statistics**: per-side win/loss/push, blackjack and bust rates, total distributions, win rate
  by dealer up-card, RTP, house edge with 95% confidence interval, drawdown, bet mix, decision
  calibration (Brier score) and latency.
- **Resilient API usage**: retries with backoff, per-attempt timeout, token/cost accounting for
  every call.
- **No hidden state**: the full run is reproducible from a seed and is written to a schema-versioned
  JSON report.

---

## Requirements

- Go 1.27 or newer
- An OpenRouter API key with access to the Jev decisions endpoint

## Setup

Clone the repository and fetch dependencies:

```bash
git clone https://github.com/VPpexis/jev-plays-blackjack.git
cd jev-plays-blackjack
go mod download
```

Create a `.env` file in the project root (it is git-ignored):

```
OPENROUTER_API_KEY=your_key_here
```

Alternatively export `OPENROUTER_API_KEY` in the environment; the file is optional.

## Quick start

```bash
go run . -games 100
```

Each round prints the bet, every action the model takes with its probabilities, the final hands,
and the bankroll change. At the end you get player/dealer, bankroll, decision and usage summaries,
and a full report is written to `blackjack_results.json`.

A short example:

```
Round 1 | bet 10.00 (bet_10) | bankroll 100.00 -> 110.00
  bet: bet_10  [bet_5=42% bet_10=52% bet_50=2% all_in=4%]
  action: 3S 5S (8) vs 8D -> HIT  [hit=88% stand=2% double=10%]
  action: 3S 5S 2S (10) vs 8D -> HIT  [hit=70% stand=30%]
  action: 3S 5S 2S 10H (20) vs 8D -> STAND  [hit=2% stand=98%]
  player: 3S 5S 2S 10H (20) -> WIN
  dealer: 8D KH (18)
  net +10.00 | WIN
```

---

## CLI flags

### Run and model

| Flag | Default | Description |
|---|---|---|
| `-model` | `~typesafe/jev-latest` | Jev model that plays the game. |
| `-games` | `100` (env `BLACKJACK_GAMES`) | Number of rounds to play. Stops early if the bankroll is exhausted. |
| `-seed` | current time | RNG seed for the shoe; the same seed reproduces the same shoe. |
| `-timeout` | `1m0s` | Timeout per API attempt. Each decision retries up to three times with backoff. |
| `-delay` | `0` | Pause between rounds (for example `500ms` or `2s`). |
| `-results` | `blackjack_results.json` | Path of the JSON report. |

### Rules

| Flag | Default | Description |
|---|---|---|
| `-decks` | `6` | Number of decks in the shoe (1–8). |
| `-penetration` | `0.75` | Fraction of the shoe dealt before reshuffling; reshuffles happen between rounds. |
| `-bjpay` | `3:2` | Blackjack payout: `3:2`, `6:5` or `1:1`. |
| `-h17` | `false` | Dealer hits soft 17. Default is S17 (dealer stands on soft 17). |
| `-double` | `true` | Allow doubling down. |
| `-double-any` | `true` | Allow doubling on any two cards; `false` restricts doubling to hard 9–11. |
| `-das` | `true` | Allow double after split. |
| `-max-hands` | `4` | Maximum number of hands after splits (4 means up to three splits). |
| `-surrender` | `false` | Allow late surrender. |
| `-insurance` | `false` | Offer insurance when the dealer shows an ace. |

Split aces always receive one card and cannot be resplit.

### Money

| Flag | Default | Description |
|---|---|---|
| `-bankroll` | `100` | Starting bankroll in units. |
| `-unit` | `1` | Bet granularity; all bets are floored to a multiple of this. |
| `-min-bet` | `1` | Minimum bet. If the bankroll falls below it, the run ends in ruin. |

### Model behaviour

| Flag | Default | Description |
|---|---|---|
| `-fallback` | `basic` | What to play when the API fails or returns an invalid action: `basic` uses the basic-strategy chart, `stand` always stands. Bet fallbacks always use `bet_5`. |
| `-show-count` | `false` | Include the Hi-Lo running count and remaining shoe composition in prompts. |

### Output

| Flag | Default | Description |
|---|---|---|
| `-quiet` | `false` | Suppress per-round output; the report and end-of-run summary are still produced. |

Boolean flags accept `-h17`, `-h17=true` and `-h17=false`.

### Examples

```bash
# Standard 6-deck game, 200 rounds, reproducible
go run . -games 200 -seed 42

# Double-deck H17 game with 6:5 blackjack, surrender and insurance
go run . -games 300 -decks 2 -h17 -bjpay 6:5 -surrender -insurance

# High-roller run with card-count information and no per-round spam
go run . -games 500 -bankroll 1000 -unit 5 -show-count -quiet

# Conservative model with a stand-only fallback
go run . -games 100 -fallback stand
```

---

## How the game works

### Shoe

Cards are drawn from a shoe of `-decks` decks. When at least `-penetration` of the shoe has been
dealt, it is reshuffled at the start of the next round. Aces count as 11 or 1, whichever keeps the
hand at or below 21.

### Actions

On each hand the engine computes the legal action set:

- **hit** and **stand** are always available;
- **double** requires two cards and enough bankroll, honours `-double-any` and `-das`;
- **split** requires a pair, enough bankroll, and fewer than `-max-hands` hands;
- **surrender** requires `-surrender`, two cards, and no dealer blackjack.

The model is only shown the actions that are actually legal. If it answers with anything else the
decision is recorded as invalid and the fallback is used.

### Dealer

The dealer draws to 17, standing on soft 17 unless `-h17` is set. The dealer does not draw when
every player hand has busted, when the player has a natural blackjack, or when the dealer has a
blackjack.

### Payouts

- Win pays 1:1, push returns the stake, loss pays nothing.
- Natural blackjack pays `-bjpay` (3:2 by default). A 21 after a split is not a natural.
- Surrender returns half the bet.
- Insurance costs half the bet and pays 2:1 when the dealer has blackjack.

### Betting

Before the cards are dealt the model chooses **5%**, **10%**, **50%** or **all-in** of its current
bankroll. The amount is floored to the bet unit and clamped to `[min-bet, bankroll]`. Doubles and
splits are financed from the bankroll and are only legal while funds remain. If the bankroll drops
below the minimum bet the run ends and the report is marked `ruined`.

### Fallback

When the API call fails after its retries, or the model returns an action that is not legal, the
engine falls back to the basic-strategy chart (or to standing, with `-fallback stand`) and records
`fallback: true` plus the error on that decision. Failed bet calls fall back to `bet_5`.

### Model context

Prompts include the rules in force, the hand with its soft/hard total, the dealer up-card, the
legal actions with plain-English definitions, the bankroll, the current split hand index, and the
last few round results. With `-show-count` the Hi-Lo running count and remaining rank counts are
added. The stated objective is to maximise the final bankroll.

---

## Metrics

The end-of-run summary and report include:

- **Player vs dealer**: hands, win/loss/push rates, blackjack and bust rates, average/min/max final
  totals and the full distribution of final totals.
- **Bankroll**: starting and ending bankroll, profit, total wagered, RTP, house edge with standard
  error and 95% confidence interval, average bet, peak, low, maximum drawdown and ruin flag.
- **Betting mix**: rounds, profit and win rate for each bet option, including all-in rounds.
- **Decisions**: counts by kind and action, fallbacks, errors, average confidence, Brier score
  (calibration against basic strategy), basic-strategy agreement rate and average latency.
- **Usage**: API calls, input/output tokens, cost and wall-clock time.

---

## Results report

`blackjack_results.json` uses schema version 2. Top-level fields:

| Field | Description |
|---|---|
| `schema_version` | Always `2`. |
| `model` | Model that played. |
| `rules` | Full rule set used for the run. |
| `config` | Games requested, rounds played, seed, bankroll settings, fallback and count flags. |
| `started_at` / `finished_at` | Run timestamps. |
| `player`, `dealer` | Aggregated side statistics. |
| `betting` | Bankroll and betting statistics. |
| `decisions` | Decision quality statistics. |
| `summary` | Rounds, net, final bankroll, ruined. |
| `usage` | Totals for calls, tokens, cost and latency. |
| `records` | One entry per round. |

Each round record contains the bet option and amount, bankroll before/after, the final hands, the
dealer cards and total, insurance bet, net result and round outcome, every decision made, the shoe
position and the running count.

Each hand record contains the cards, total, soft flag, bet and total wagered, whether it was
doubled, split or surrendered, whether it was a natural blackjack or a bust, its outcome, payout
and net.

Each decision record contains its kind (`bet`, `action` or `insurance`), the hand and dealer
context, the legal actions, the chosen action and probabilities, confidence, the basic-strategy
action and whether the model agreed with it, fallback and error flags, latency and token usage.

---

## Project layout

```
.
├── main.go            CLI, round loop, console output and report assembly
├── cards.go           Card, Hand and Shoe (decks, penetration, Hi-Lo count)
├── rules.go           Rules, validation and the basic-strategy chart
├── engine.go          Round state machine, legal actions, splits, payouts, bankroll
├── model.go           Prompt building, Decisions API calls, retries and fallback
├── stats.go           Aggregation into player/dealer/betting/decision statistics
├── *_test.go          Unit tests for every module
├── jev/
│   └── client.go      Reusable OpenRouter Decisions API client and answer parsing
├── cmd/
│   └── jevtest/
│       └── main.go    Minimal decisions-API demo (urgency classification example)
├── go.mod / go.sum    Module definition and dependency checksums
├── .gitignore         Ignores the API key file, binaries and generated reports
└── README.md          This file
```

---

## Development

```bash
go build ./...     # build everything
go vet ./...       # static checks
go test ./...      # unit tests (no API calls; the model is mocked with httptest)
gofmt -l .         # formatting check
```

The tests cover hand totals, shoe penetration and composition, the basic-strategy chart, payout
math, bet sizing, legal-action rules, bankroll conservation, ruin, split flow, the mocked model
end-to-end path, fallback behaviour and statistics aggregation.

## Decisions API demo

`cmd/jevtest` contains a small standalone example of calling the Decisions API directly:

```bash
go run ./cmd/jevtest
```

It classifies a support message (`noul` question) and writes the parsed result to
`jev_decision.json`.

## Cost and performance

Each round costs one betting call plus roughly one to two action calls. In a typical 100-round run
that is a few hundred API calls, a few cents of usage, and a few minutes of wall-clock time at
roughly 0.8 seconds per call. Use `-quiet` for long runs and `-seed` when you need to reproduce a
particular shoe.

## Notes and limitations

- The model cannot split, double or surrender unless the rules and bankroll allow it, and it is
  never shown an illegal option.
- The basic-strategy chart assumes a multi-deck game; single-deck play with `-decks 1` uses the
  same chart and will differ slightly from composition-dependent optimal play.
- All decisions are independent API calls; the model has no memory between rounds other than the
  recent-results summary included in the prompt.
