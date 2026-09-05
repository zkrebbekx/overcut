package model

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/rules"
)

// syntheticSeason builds a small consistent season: six drivers on three
// teams, five completed rounds, with team 0 fastest and team 2 slowest.
func syntheticSeason() dataset.Data {
	d := dataset.Data{Season: 2026}
	tlas := []string{"AAA", "AAB", "BBA", "BBB", "CCA", "CCB"}

	for r := 1; r <= 5; r++ {
		round := dataset.Round{
			Round: r, Name: fmt.Sprintf("R%d", r), HasResults: true,
			Quali: map[string]int{}, Race: map[string]dataset.RaceRow{},
		}
		for i, tla := range tlas {
			pos := i + 1
			round.Quali[tla] = pos
			round.Race[tla] = dataset.RaceRow{Grid: pos, Pos: pos}
		}
		d.Rounds = append(d.Rounds, round)
	}

	for i, tla := range tlas {
		a := dataset.Asset{
			ID: fmt.Sprintf("d%d", i), Kind: dataset.KindDriver,
			Name: tla, TLA: tla, TeamID: fmt.Sprintf("t%d", i/2),
		}
		for r := 1; r <= 5; r++ {
			a.History = append(a.History, dataset.AssetRound{
				Gameday: r, Active: true, Price: 10, OldPrice: 10,
				Points: float64(30 - i*5),
			})
		}
		d.Assets = append(d.Assets, a)
	}
	for t := 0; t < 3; t++ {
		a := dataset.Asset{
			ID: fmt.Sprintf("c%d", t), Kind: dataset.KindConstructor,
			Name: fmt.Sprintf("Team%d", t), TeamID: fmt.Sprintf("t%d", t),
		}
		for r := 1; r <= 5; r++ {
			a.History = append(a.History, dataset.AssetRound{
				Gameday: r, Active: true, Price: 20, OldPrice: 20,
				Points: float64(60 - t*15),
			})
		}
		d.Assets = append(d.Assets, a)
	}
	return d
}

func TestFit(t *testing.T) {
	Convey("Given a synthetic five-round season with a fixed pecking order", t, func() {
		d := syntheticSeason()
		cfg := rules.Default()

		Convey("When the model fits on the full season", func() {
			m := Fit(d, cfg, 5)

			Convey("Then every selectable driver and constructor gets a model", func() {
				So(m.Drivers, ShouldHaveLength, 6)
				So(m.Constructors, ShouldHaveLength, 3)
			})

			Convey("Then the fastest driver carries the lowest race mean", func() {
				fast, _ := m.DriverByTLA("AAA")
				slow, _ := m.DriverByTLA("CCB")
				So(fast.RaceMu, ShouldBeLessThan, slow.RaceMu)
				So(fast.RaceMu, ShouldAlmostEqual, 1, 0.01)
			})

			Convey("Then a driver with no retirement keeps a small shrunk DNF rate", func() {
				dm, _ := m.DriverByTLA("AAA")
				So(dm.DNFProb, ShouldBeGreaterThan, 0)
				So(dm.DNFProb, ShouldBeLessThan, 0.2)
			})
		})

		Convey("When one driver retires in every round", func() {
			for i := range d.Rounds {
				row := d.Rounds[i].Race["CCB"]
				row.DNF = true
				d.Rounds[i].Race["CCB"] = row
			}
			m := Fit(d, cfg, 5)

			Convey("Then that driver's DNF rate rises far above the field", func() {
				crash, _ := m.DriverByTLA("CCB")
				clean, _ := m.DriverByTLA("AAA")
				So(crash.DNFProb, ShouldBeGreaterThan, 0.4)
				So(crash.DNFProb, ShouldBeGreaterThan, clean.DNFProb*3)
			})
		})
	})
}

func TestSimulate(t *testing.T) {
	Convey("Given a model fitted on the synthetic season", t, func() {
		d := syntheticSeason()
		cfg := rules.Default()
		m := Fit(d, cfg, 5)

		Convey("When the model simulates the next round twice with one seed", func() {
			a := m.Simulate(6, false, 2000, 42)
			b := m.Simulate(6, false, 2000, 42)

			Convey("Then the projections repeat exactly", func() {
				So(a.Assets, ShouldHaveLength, len(b.Assets))
				for i := range a.Assets {
					So(a.Assets[i].Mean, ShouldEqual, b.Assets[i].Mean)
				}
			})
		})

		Convey("When the model simulates the next round", func() {
			sim := m.Simulate(6, false, 4000, 1)

			Convey("Then every asset receives a projection", func() {
				So(sim.Assets, ShouldHaveLength, 9)
			})

			Convey("Then the fastest driver projects above the slowest", func() {
				fast, ok := sim.ByID("d0")
				So(ok, ShouldBeTrue)
				slow, _ := sim.ByID("d5")
				So(fast.Mean, ShouldBeGreaterThan, slow.Mean)
			})

			Convey("Then the percentiles come in order", func() {
				for _, p := range sim.Assets {
					So(p.P10, ShouldBeLessThanOrEqualTo, p.P50)
					So(p.P50, ShouldBeLessThanOrEqualTo, p.P90)
				}
			})

			Convey("Then the top constructor projects above the bottom constructor", func() {
				top, _ := sim.ByID("c0")
				bottom, _ := sim.ByID("c2")
				So(top.Mean, ShouldBeGreaterThan, bottom.Mean)
			})
		})

		Convey("When the round has a sprint", func() {
			plain := m.Simulate(6, false, 4000, 1)
			sprint := m.Simulate(6, true, 4000, 1)

			Convey("Then the front-runner projects more points than on a plain weekend", func() {
				a, _ := sprint.ByID("d0")
				b, _ := plain.ByID("d0")
				So(a.Mean, ShouldBeGreaterThan, b.Mean)
			})
		})
	})
}
