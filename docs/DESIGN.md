# overcut — Product Design

## Principles

1. **One decision per screen.** The player has one job on a race weekend:
   decide what to do with the team. The home screen answers that job first
   and shows everything else second.
2. **Show the range, not the guess.** Every projection is a distribution.
   The UI shows P10, P50, and P90 on every asset, so the player sees risk,
   not a single number.
3. **Say what the model knows.** A projection made before qualifying and a
   projection made after the grid is set are different products. The
   status of the weekend is visible on every screen.
4. **Earn trust with numbers.** The model's past accuracy is one tap away,
   with baselines, so the player can calibrate how much to lean on it.
5. **Calm under a deadline.** Dark surface, low-noise chrome, one accent
   color for the action, and no motion that delays a read.

## Primary flow

Enter the current team → set what is known about the weekend → read the
recommended move → check the range and the alternatives → decide.

The team persists in the browser. On the next visit the player lands on
the recommendation directly.

## Views

| View | Purpose | Elements in priority order |
| --- | --- | --- |
| Decide | The recommendation | Move card (out → in, points delta, transfers, penalty), current team, alternatives, weekend status |
| Projections | Every asset's distribution | Range table sorted by mean, filters, value column, asset detail |
| Prices | Next price changes | Predicted change per asset, predictor accuracy |
| Trust | Model accuracy | Summary tiles, per-round table, model vs baselines chart |
| Hindsight | Best team for a past round | Round picker, top teams with official points |
| Rules | Scoring reference | Tables rendered from the rules engine |

Navigation: a left rail on desktop, a bottom bar on mobile. The round
selector and the weekend-status chip sit in the top bar on every view.

## Decide view

```
┌──────────────────────────────────────────────────────────────────────┐
│ R13 Italian GP · Model knows: grid penalties, FP3          [Sync]     │
├────────────────────────────┬─────────────────────────────────────────┤
│ RECOMMENDED                │ YOUR TEAM                    $120.2M    │
│ +24.7 pts vs keep          │ [ANT★] [HUL] [BOR] [COL] [LIN]          │
│                            │ [Mercedes] [McLaren]                    │
│ OUT  McLaren   → IN Ferrari│ Free transfers [3]  Chip [none ▾]       │
│ OUT  Bortoleto → IN Gasly  │ Risk [mean | safe | ceiling]            │
│                            ├─────────────────────────────────────────┤
│ 2 transfers · no penalty   │ WEEKEND                                 │
│ Boost: Antonelli           │ Quali order  [ … ]                      │
│ Projected 229.9 ▮▮▮▮▮▮▮    │ Grid / back of grid [ANT, ALB]          │
│ P10 141 ── P90 318         │ FP3 order [ … ]                         │
├────────────────────────────┴─────────────────────────────────────────┤
│ ALTERNATIVES                                                         │
│ #2 229.0  Gasly, Colapinto, Antonelli★, Lindblad, Albon …  3 tr      │
│ #3 226.3  …                                                          │
└──────────────────────────────────────────────────────────────────────┘
```

Mobile: the move card first, then the team editor, then the weekend
panel, then alternatives. The team editor opens as a sheet.

## Projections view

One row per asset. The range bar spans P10 to P90 on a shared axis; a
tick marks the mean. Rows sort by mean by default. Columns: asset, price,
mean, range bar, points per million, ownership. A row tap opens a detail
panel with the gameday history (points and price).

## Visual system

- **Surfaces:** background `#0B0D10`, panel `#14171C`, raised `#1B1F26`,
  border `#262B33`.
- **Text:** primary `#F2F4F7`, secondary `#A7AFBC`, muted `#6B7280`.
- **Accent:** `#3ED0FF` (action, selection, mean tick).
- **Gain / loss:** `#4ADE80` / `#FB7185` — distinguishable under
  deuteranopia and protanopia by lightness as well as hue; every gain or
  loss also carries a sign or an arrow.
- **Warning:** `#FBBF24` (unknown weekend state, stale data).
- **Team colors** appear only as a 3-pixel left edge on asset chips, never
  as chrome.
- **Type:** Space Grotesk for display, Inter for body, JetBrains Mono for
  every number in a table.
- **Radius:** 10px cards, 6px chips. **Spacing:** 4px grid.
- **Motion:** 120ms fades on panel changes; none on data.

## Charts

- Ranges: horizontal bar from P10 to P90, mean as a vertical tick, P50
  as a small dot. Shared axis per table, labeled at the header.
- Comparisons (model vs baselines): grouped bars per round; model in the
  accent, baselines in neutral greys.
- Deltas: signed number with color and arrow; never color alone.

## Copy

- Empty team: "Add five drivers and two constructors to see your move."
- Unknown weekend: "Before qualifying — projections sample the grid."
- Known grid: "Grid set — projections use the official starting order."
- Loading: "Running 20,000 race simulations…"
- Confidence: "Ranked 22 drivers with 0.59 correlation over the last 9
  rounds. Driver error ±11 points."

## Accessibility

Contrast ≥ 4.5:1 for all text. Keyboard reachable controls with visible
focus. Live region announces a new recommendation. Every color-coded
value carries text or a symbol.

## Build order

1. Decide, Projections, weekend panel.
2. Trust, Prices.
3. Hindsight, Rules, asset detail.
