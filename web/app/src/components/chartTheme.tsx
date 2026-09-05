// Chart tokens for the dark surface. Series colors are validated for
// colorblind separation on the panel surface; text stays in ink tokens.

export const chart = {
  s1: "#3987e5", // model / primary series
  s2: "#d95926", // naive baseline
  s3: "#199e70", // hindsight / third series
  grid: "#262b33",
  cursor: "rgba(255,255,255,0.04)",
  tick: { fill: "#a7afbc", fontSize: 11, fontFamily: "JetBrains Mono, monospace" },
  tooltip: {
    background: "#1b1f26",
    border: "1px solid #262b33",
    borderRadius: 8,
    color: "#f2f4f7",
    fontSize: 12,
  },
};
