package price

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/zkrebbekx/overcut/internal/dataset"
)

// priceSeason builds a season where price changes track recent form
// perfectly: the asset with the best last-three points always gains.
func priceSeason() dataset.Data {
	d := dataset.Data{Season: 2026}
	for i := 0; i < 6; i++ {
		a := dataset.Asset{
			ID: fmt.Sprintf("d%d", i), Kind: dataset.KindDriver,
			Name: fmt.Sprintf("D%d", i), TLA: fmt.Sprintf("D%dX", i), TeamID: "t",
		}
		p := 10.0
		for g := 1; g <= 8; g++ {
			pts := float64(30 - i*5) // fixed form order
			change := 0.0
			if g > 1 {
				change = 0.3 - float64(i)*0.12 // best gains, worst falls
			}
			old := p
			p += change
			a.History = append(a.History, dataset.AssetRound{
				Gameday: g, Active: true, Price: p, OldPrice: old,
				Points: pts, Ownership: float64(60 - i*10),
			})
		}
		d.Assets = append(d.Assets, a)
	}
	return d
}

func TestClip(t *testing.T) {
	Convey("Given the tier bounds", t, func() {
		Convey("When a cheap asset predicts a large rise", func() {
			Convey("Then the change clips to the low-tier limit", func() {
				So(clip(1.5, 10.0), ShouldEqual, TierBMax)
				So(clip(-1.5, 10.0), ShouldEqual, -TierBMax)
			})
		})
		Convey("When an expensive asset predicts a large rise", func() {
			Convey("Then the change clips to the high-tier limit", func() {
				So(clip(0.5, 25.0), ShouldEqual, TierAMax)
				So(clip(-0.5, 25.0), ShouldEqual, -TierAMax)
			})
		})
		Convey("When the prediction sits inside the bounds", func() {
			Convey("Then the change passes through", func() {
				So(clip(0.1, 25.0), ShouldEqual, 0.1)
			})
		})
	})
}

func TestFitAndPredict(t *testing.T) {
	Convey("Given a season where form drives every price move", t, func() {
		d := priceSeason()

		Convey("When the examples are extracted", func() {
			ex := Examples(d)

			Convey("Then every gameday after the first produces one example per asset", func() {
				So(len(ex), ShouldEqual, 6*7)
			})

			Convey("Then the recorded change matches the constructed movement", func() {
				for _, e := range ex {
					So(e.Change, ShouldAlmostEqual, 0.3-float64(e.AssetID[1]-'0')*0.12, 1e-9)
				}
			})
		})

		Convey("When the model fits and predicts the next change", func() {
			m := Fit(Examples(d))
			preds := Predict(d, m)

			Convey("Then the in-form asset predicts a rise and the out-of-form asset predicts a fall", func() {
				byID := map[string]float64{}
				for _, p := range preds {
					byID[p.AssetID] = p.Change
				}
				So(byID["d0"], ShouldBeGreaterThan, 0.15)
				So(byID["d5"], ShouldBeLessThan, -0.15)
			})
		})

		Convey("When the walk-forward backtest runs", func() {
			rep := Backtest(d)

			Convey("Then the model beats the always-zero baseline on this constructed season", func() {
				So(rep.Examples, ShouldBeGreaterThan, 0)
				So(rep.MAE, ShouldBeLessThan, rep.NaiveMAE)
			})

			Convey("Then the direction hit rate is perfect on this constructed season", func() {
				So(rep.Direction, ShouldAlmostEqual, 1.0, 0.01)
			})
		})
	})
}
