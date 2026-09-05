// The scoring rules the engine uses, rendered from the running config.

import { useEffect, useState } from "react";
import { api } from "../api";
import { Card, ErrorBox, Spinner } from "../components/ui";

interface Rules {
  quali_points: number[];
  quali_no_time: number;
  race_points: number[];
  position_gained: number;
  position_lost: number;
  overtake: number;
  fastest_lap: number;
  driver_of_the_day: number;
  race_dnf: number;
  sprint_points: number[];
  sprint_max_lost: number;
  sprint_fastest_lap: number;
  sprint_dnf: number;
  constructor_quali_bonus: { none_in_q2: number; one_in_q2: number; both_in_q2: number; one_in_q3: number; both_in_q3: number };
  constructor_dsq: { quali: number; sprint: number; race: number };
  pit_stop_points: { under_seconds: number; points: number }[];
  fastest_pit_stop: number;
  record_pit_stop: number;
  record_pit_stop_time: number;
  price_floor: number;
  price_cap: number;
  price_form_rounds: number;
  transfer_penalty: number;
  free_transfers: number;
  max_carry_over: number;
  boost_multiplier: number;
  extra_boost_multiplier: number;
  budget: number;
  team_drivers: number;
  team_constructors: number;
}

export function RulesView() {
  const [r, setR] = useState<Rules | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    api.rules().then((x) => setR(x as unknown as Rules)).catch((e) => setError(String(e.message ?? e)));
  }, []);
  if (error) return <ErrorBox error={error} />;
  if (!r) return <Spinner label="Loading rules…" />;

  const Table = ({ rows }: { rows: [string, string | number][] }) => (
    <table className="w-full text-sm">
      <tbody>
        {rows.map(([k, v]) => (
          <tr key={k} className="border-t border-line/60 first:border-0">
            <td className="py-1 pr-3 text-ink-2">{k}</td>
            <td className="num py-1 text-right text-ink">{v}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
  const positional = (t: number[]) => t.map((p, i) => [`P${i + 1}`, p] as [string, number]);

  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <Card title="Team">
        <Table
          rows={[
            ["Drivers", r.team_drivers],
            ["Constructors", r.team_constructors],
            ["Cost cap", `$${r.budget}M`],
            ["Free transfers per round", r.free_transfers],
            ["Carry-over (max)", r.max_carry_over],
            ["Extra transfer", `${r.transfer_penalty} pts`],
            ["Boost", `×${r.boost_multiplier} one driver, every round`],
            ["x3 chip", `×${r.extra_boost_multiplier} one driver; Boost moves to another`],
          ]}
        />
      </Card>
      <Card title="Qualifying (drivers)">
        <Table rows={[...positional(r.quali_points), ["No time / DSQ", r.quali_no_time]]} />
      </Card>
      <Card title="Race (drivers)">
        <Table
          rows={[
            ...positional(r.race_points),
            ["Position gained", `+${r.position_gained} each`],
            ["Position lost", `${r.position_lost} each, uncapped`],
            ["Overtake", `+${r.overtake} each (scores even after a DNF)`],
            ["Fastest lap", r.fastest_lap],
            ["Driver of the day", `${r.driver_of_the_day} (driver only)`],
            ["DNF / NC / DSQ", `${r.race_dnf}, no position points`],
          ]}
        />
      </Card>
      <Card title="Sprint (drivers)">
        <Table
          rows={[
            ...positional(r.sprint_points),
            ["Sprint qualifying", "no points"],
            ["Position lost", `${r.position_lost} each, capped at ${r.sprint_max_lost}`],
            ["Fastest lap", r.sprint_fastest_lap],
            ["DNF / NC / DSQ", r.sprint_dnf],
          ]}
        />
      </Card>
      <Card title="Constructors">
        <Table
          rows={[
            ["Base", "both drivers' points, minus driver of the day"],
            ["Neither driver in Q2", r.constructor_quali_bonus.none_in_q2],
            ["One driver in Q2", r.constructor_quali_bonus.one_in_q2],
            ["Both drivers in Q2", r.constructor_quali_bonus.both_in_q2],
            ["One driver in Q3", r.constructor_quali_bonus.one_in_q3],
            ["Both drivers in Q3", r.constructor_quali_bonus.both_in_q3],
            ["Driver DSQ in qualifying", r.constructor_dsq.quali],
            ["Driver DSQ in sprint", r.constructor_dsq.sprint],
            ["Driver DSQ in race", r.constructor_dsq.race],
          ]}
        />
      </Card>
      <Card title="Pit stops (constructors)">
        <Table
          rows={[
            ...r.pit_stop_points.map((b) => [`Under ${b.under_seconds.toFixed(2)}s`, b.points] as [string, number]),
            ["Fastest stop of the race", `+${r.fastest_pit_stop}`],
            ["New record (< " + r.record_pit_stop_time.toFixed(2) + "s)", `+${r.record_pit_stop}`],
          ]}
        />
      </Card>
      <Card title="Prices">
        <Table
          rows={[
            ["Floor", `$${r.price_floor}M`],
            ["Cap", `$${r.price_cap}M`],
            ["Driven by", `average of the last ${r.price_form_rounds} rounds`],
          ]}
        />
      </Card>
      <Card title="Chips">
        <ul className="space-y-2 text-sm text-ink-2">
          <li>
            <b className="text-ink">Autopilot</b> — Boost moves to your top scorer after the race.
          </li>
          <li>
            <b className="text-ink">x3 Boost</b> — one driver scores ×3; the regular ×2 Boost goes to another.
          </li>
          <li>
            <b className="text-ink">No Negative</b> — each negative scoring category floors at zero, per asset.
          </li>
          <li>
            <b className="text-ink">Wildcard</b> — unlimited transfers within the cap.
          </li>
          <li>
            <b className="text-ink">Limitless</b> — unlimited transfers and no cap for one round; team restores after.
          </li>
          <li>
            <b className="text-ink">Final Fix</b> — one driver swap after qualifying; the new driver scores from the next session.
          </li>
        </ul>
      </Card>
    </div>
  );
}
