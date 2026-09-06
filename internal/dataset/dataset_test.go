package dataset

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDue(t *testing.T) {
	Convey("Given a calendar with a qualifying session at 14:00 and a race at 13:00 the next day", t, func() {
		quali := time.Date(2026, 9, 5, 14, 0, 0, 0, time.UTC)
		race := time.Date(2026, 9, 6, 13, 0, 0, 0, time.UTC)
		d := Data{
			SyncedAt: quali.Add(-2 * time.Hour),
			Rounds: []Round{{
				Round: 13, Sessions: map[string]time.Time{"Qualifying": quali, "Race": race},
			}},
		}

		Convey("When the clock is inside qualifying", func() {
			_, due := d.Due(quali.Add(30 * time.Minute))
			Convey("Then a sync is not due", func() {
				So(due, ShouldBeFalse)
			})
		})

		Convey("When qualifying ended twenty minutes ago", func() {
			reason, due := d.Due(quali.Add(80 * time.Minute))
			Convey("Then a sync is due and names the session", func() {
				So(due, ShouldBeTrue)
				So(reason, ShouldContainSubstring, "Qualifying")
			})
		})

		Convey("When qualifying ended seven hours ago and the data is nine hours old", func() {
			_, due := d.Due(quali.Add(8 * time.Hour))
			Convey("Then a sync is not due", func() {
				So(due, ShouldBeFalse)
			})
		})

		Convey("When the race ended an hour ago", func() {
			reason, due := d.Due(race.Add(4 * time.Hour))
			Convey("Then a sync is due for the race", func() {
				So(due, ShouldBeTrue)
				So(reason, ShouldContainSubstring, "Race")
			})
		})

		Convey("When nothing ended recently but the data is two days old", func() {
			reason, due := d.Due(race.Add(3 * 24 * time.Hour))
			Convey("Then a sync is due because of age", func() {
				So(due, ShouldBeTrue)
				So(reason, ShouldContainSubstring, "old")
			})
		})

		Convey("When the next session is asked for before qualifying", func() {
			round, name, start, ok := d.NextSession(quali.Add(-time.Hour))
			Convey("Then it is round 13 qualifying", func() {
				So(ok, ShouldBeTrue)
				So(round, ShouldEqual, 13)
				So(name, ShouldEqual, "Qualifying")
				So(start, ShouldEqual, quali)
			})
		})
	})
}

func TestQualiOrder(t *testing.T) {
	Convey("Given a round with a qualifying classification", t, func() {
		r := Round{Quali: map[string]int{"HAM": 3, "GAS": 1, "RUS": 2}}
		Convey("When the order is requested", func() {
			Convey("Then the codes come from P1 in order", func() {
				So(r.QualiOrder(), ShouldResemble, []string{"GAS", "RUS", "HAM"})
			})
		})
	})
	Convey("Given a round without qualifying", t, func() {
		Convey("Then the order is nil", func() {
			So(Round{}.QualiOrder(), ShouldBeNil)
		})
	})
}

func TestClassified(t *testing.T) {
	Convey("Given the Jolpica positionText encoding", t, func() {
		Convey("When the car finishes on the lead lap or lapped", func() {
			Convey("Then a numeric positionText counts as classified", func() {
				So(classified("1"), ShouldBeTrue)
				So(classified("16"), ShouldBeTrue)
			})
		})
		Convey("When the car retires, is disqualified, or withdraws", func() {
			Convey("Then a letter positionText counts as a DNF", func() {
				So(classified("R"), ShouldBeFalse)
				So(classified("D"), ShouldBeFalse)
				So(classified("W"), ShouldBeFalse)
				So(classified("E"), ShouldBeFalse)
			})
		})
		Convey("When the field is empty", func() {
			Convey("Then the row counts as a DNF", func() {
				So(classified(""), ShouldBeFalse)
			})
		})
	})
}

func TestSelectable(t *testing.T) {
	Convey("Given a dataset with a mid-season seat swap", t, func() {
		d := Data{
			Assets: []Asset{
				{ID: "old", Kind: KindDriver, History: []AssetRound{
					{Gameday: 1, Active: true},
					{Gameday: 5, Active: false},
				}},
				{ID: "new", Kind: KindDriver, History: []AssetRound{
					{Gameday: 5, Active: true},
				}},
				{ID: "gone", Kind: KindDriver, History: []AssetRound{
					{Gameday: 3, Active: true},
				}},
			},
		}

		Convey("When the selectable filter runs", func() {
			Convey("Then the replacement driver is selectable", func() {
				So(d.Selectable(d.Assets[1]), ShouldBeTrue)
			})
			Convey("Then the inactive driver is not selectable", func() {
				So(d.Selectable(d.Assets[0]), ShouldBeFalse)
			})
			Convey("Then a driver missing from the latest gameday is not selectable", func() {
				So(d.Selectable(d.Assets[2]), ShouldBeFalse)
			})
		})
	})
}
