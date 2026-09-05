package dataset

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

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
