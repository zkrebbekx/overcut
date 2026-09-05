package rules

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDriverPoints(t *testing.T) {
	cfg := Default()

	Convey("Given the default 2026 scoring rules", t, func() {
		Convey("When a driver takes pole and wins from pole with the fastest lap and the DOTD award", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 1, GridPos: 1, FinishPos: 1,
				FastestLap: true, DOTD: true,
			})
			Convey("Then the driver scores 10 quali + 25 race + 10 FL + 10 DOTD = 55", func() {
				So(pts, ShouldEqual, 55)
			})
		})

		Convey("When a driver qualifies P15, gains five places to P10, and makes three overtakes", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 15, GridPos: 15, FinishPos: 10, Overtakes: 3,
			})
			Convey("Then the driver scores 0 quali + 1 race + 5 gained + 3 overtakes = 9", func() {
				So(pts, ShouldEqual, 9)
			})
		})

		Convey("When a driver qualifies P2 and retires from the race", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 2, GridPos: 2, DNF: true,
			})
			Convey("Then the driver scores 9 quali - 20 DNF = -11", func() {
				So(pts, ShouldEqual, -11)
			})
		})

		Convey("When a driver sets no qualifying time and finishes P12 from P22", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 0, GridPos: 22, FinishPos: 12,
			})
			Convey("Then the driver scores -5 quali + 0 race + 10 gained = 5", func() {
				So(pts, ShouldEqual, 5)
			})
		})

		Convey("When a driver loses four places from P3 to P7", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 3, GridPos: 3, FinishPos: 7,
			})
			Convey("Then the driver scores 8 quali + 6 race - 4 lost = 10", func() {
				So(pts, ShouldEqual, 10)
			})
		})

		Convey("When a sprint weekend driver wins the sprint from SQ2 with the sprint fastest lap", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 1, GridPos: 1, FinishPos: 1,
				HasSprint: true, SprintQPos: 2, SprintGrid: 2, SprintPos: 1, SprintFL: true,
			})
			Convey("Then the sprint leg adds 7 SQ + 8 sprint + 1 gained + 5 FL = 21 over the 35-point race leg", func() {
				So(pts, ShouldEqual, 35+21)
			})
		})

		Convey("When a driver retires from the sprint", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 5, GridPos: 5, FinishPos: 5,
				HasSprint: true, SprintQPos: 5, SprintGrid: 5, SprintDNF: true,
			})
			Convey("Then the sprint leg adds 4 SQ - 10 DNF and the race leg adds 6 + 10", func() {
				So(pts, ShouldEqual, 4-10+6+10)
			})
		})
	})
}

func TestConstructorPoints(t *testing.T) {
	cfg := Default()

	Convey("Given the default 2026 scoring rules", t, func() {
		Convey("When both drivers reach Q3 and finish P1 and P2 with 4 pit-stop points", func() {
			a := DriverWeekend{QualiPos: 1, GridPos: 1, FinishPos: 1}
			b := DriverWeekend{QualiPos: 2, GridPos: 2, FinishPos: 2}
			pts := cfg.ConstructorPoints(a, b, 4)
			Convey("Then the constructor scores both drivers + 10 bonus + 4 pit stops", func() {
				So(pts, ShouldEqual, (10+25)+(9+18)+10+4)
			})
		})

		Convey("When a driver holds the DOTD award", func() {
			a := DriverWeekend{QualiPos: 1, GridPos: 1, FinishPos: 1, DOTD: true}
			b := DriverWeekend{QualiPos: 2, GridPos: 2, FinishPos: 2}
			pts := cfg.ConstructorPoints(a, b, 0)
			Convey("Then the constructor does not receive the DOTD points", func() {
				So(pts, ShouldEqual, (10+25)+(9+18)+10)
			})
		})

		Convey("When both drivers fall in Q1 at P17 and P20", func() {
			a := DriverWeekend{QualiPos: 17, GridPos: 17, FinishPos: 15}
			b := DriverWeekend{QualiPos: 20, GridPos: 20, FinishPos: 18}
			pts := cfg.ConstructorPoints(a, b, 0)
			Convey("Then the constructor takes the -1 progression penalty plus position points", func() {
				So(pts, ShouldEqual, 2+2-1)
			})
		})

		Convey("When one driver reaches Q3 at P10 and one falls at P16", func() {
			bonus := cfg.ConstructorQualiBonusFor(2, 1)
			Convey("Then the progression bonus is the one-in-Q3 value", func() {
				So(bonus, ShouldEqual, 5)
			})
		})
	})
}
