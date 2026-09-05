# overcut

An F1 Fantasy analysis toolkit in Go. One binary. No accounts. No
paywall.

The toolkit gives you the same tools as the popular fantasy sites, and it
adds three properties that they do not give you:

1. **Exact optimization.** The optimizer enumerates every legal team —
   about 1.4 million driver and constructor combinations — and scores each
   one. The result is the provable optimum for the given projections, not
   a heuristic.
2. **Measured accuracy.** The `backtest` command replays the season. For
   each past round, it fits the model only on earlier rounds, projects the
   round, and scores the projection against the official points. The
   model's edge over naive strategies is a printed number, not a claim.
3. **Self-calibrating scoring.** The scoring tables live in data and match
   the official 2026 rules. The components that public timing data cannot
   derive — overtake points, driver-of-the-day votes, and constructor
   pit-stop points — come from the official fantasy feed itself.

## Backtest result (2026 season, rounds 4–12)

| Metric | Model | Last-round baseline | Season-mean baseline |
| --- | --- | --- | --- |
| Driver points MAE | **11.6** | 15.5 | 12.1 |
| Mean driver rank correlation | **0.57** | — | — |
| Mean points of the pre-race optimal team | **192** | 139 (naive) | 288 (hindsight limit) |

The price predictor reports a mean absolute error of $0.28M per change and
an 80% direction hit rate, walk-forward on 318 real price moves.

## Install

```bash
go install github.com/zkrebbekx/overcut/cmd/overcut@latest
```

## Use

```bash
overcut sync                          # download the season (two public sources)
overcut project                       # projected points for the next round
overcut optimize                      # the best teams for your budget
overcut optimize -team "VER,NOR,HAD,GAS,BOR,McLaren,Ferrari" -free 2
overcut optimize -chip limitless      # chips: wildcard | limitless | 3x
overcut prices                        # predicted price changes
overcut backtest                      # measure the model on past rounds
overcut hindsight -round 12           # the best possible team for a past round
```

`sync` stores the dataset at `~/.overcut/season2026.json`. Every other
command works offline from that file and is deterministic for a given
seed.

## Data sources

- [Jolpica F1](https://api.jolpi.ca) — schedule, qualifying, sprint, and
  race classifications.
- The public F1 Fantasy feed — prices, ownership, and official fantasy
  points per asset per gameday.

Both sources are public and need no authentication.

## How the projection works

- **Pace.** Each driver gets a qualifying and a race pace estimate: an
  exponentially weighted average of past positions with a four-round
  half-life, plus a spread fitted from the same data.
- **Risk.** Each driver gets a retirement probability, shrunk toward the
  field rate.
- **Simulation.** The model simulates the weekend many times: it samples a
  grid, retirements, a finish order, overtake counts, the fastest lap, and
  the driver-of-the-day vote, then scores every sample with the official
  rules. Sprint weekends simulate the sprint legs too.
- **Output.** Each asset gets a full points distribution: mean, P10, P50,
  and P90. Optimize with `-risk p10` for a safe team or `-risk p90` for a
  ceiling play.
- **Constructors.** A constructor scores its two drivers plus the
  qualifying progression bonus, plus a pit-stop component fitted from the
  residual between official constructor points and rule-derived points.

## Scoring rules

The 2026 tables are built in. Pass `-rules myrules.json` to override any
value; the file only needs the fields that change. See `rules.Default` in
`rules/rules.go` for the field list.

## Packages

| Package | Purpose |
| --- | --- |
| `rules` | Official scoring tables and the points engine |
| `model` | Pace fit and Monte Carlo projection |
| `optimize` | Exact team optimizer with transfers and chips |
| `price` | Price-change predictor with walk-forward validation |
| `backtest` | Model accuracy measurement |
| `internal/jolpica`, `internal/feed`, `internal/dataset` | Data ingestion |

## License

MIT
