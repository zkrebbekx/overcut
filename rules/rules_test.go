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

		Convey("When a driver makes three overtakes and then retires", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 22, GridPos: 22, FinishPos: 19, DNF: true, Overtakes: 3,
			})
			Convey("Then the overtakes still score and no position points apply: -20 + 3 = -17", func() {
				So(pts, ShouldEqual, -17)
			})
		})

		Convey("When a driver is disqualified from qualifying", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 1, QualiDSQ: true, GridPos: 22, FinishPos: 12,
			})
			Convey("Then qualifying scores the -5 penalty and the race scores 10 gained", func() {
				So(pts, ShouldEqual, -5+10)
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

		Convey("When a sprint weekend driver wins the sprint from the second grid slot with the sprint fastest lap", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 1, GridPos: 1, FinishPos: 1,
				HasSprint: true, SprintGrid: 2, SprintPos: 1, SprintFL: true,
			})
			Convey("Then sprint qualifying scores nothing and the sprint adds 8 + 1 gained + 5 FL = 14 over the 35-point race leg", func() {
				So(pts, ShouldEqual, 35+14)
			})
		})

		Convey("When a driver retires from the sprint after two overtakes", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 5, GridPos: 5, FinishPos: 5,
				HasSprint: true, SprintGrid: 5, SprintDNF: true, SprintOvers: 2,
			})
			Convey("Then the sprint leg scores -10 + 2 and the race leg adds 6 + 10", func() {
				So(pts, ShouldEqual, -10+2+6+10)
			})
		})

		Convey("When a driver drops from P1 to P15 in the sprint", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 1, GridPos: 1, FinishPos: 1,
				HasSprint: true, SprintGrid: 1, SprintPos: 15,
			})
			Convey("Then the sprint positions lost cap at -10", func() {
				So(pts, ShouldEqual, 35-10)
			})
		})

		Convey("When a driver drops from P1 to P15 in the race", func() {
			pts := cfg.DriverPoints(DriverWeekend{
				QualiPos: 1, GridPos: 1, FinishPos: 15,
			})
			Convey("Then the race positions lost are not capped: 10 - 14 = -4", func() {
				So(pts, ShouldEqual, -4)
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

		Convey("When a driver is disqualified from the race", func() {
			a := DriverWeekend{QualiPos: 1, GridPos: 1, FinishPos: 1, RaceDSQ: true}
			b := DriverWeekend{QualiPos: 2, GridPos: 2, FinishPos: 1}
			pts := cfg.ConstructorPoints(a, b, 0)
			Convey("Then the constructor takes the driver's -20 and an extra -20 of its own", func() {
				So(pts, ShouldEqual, (10-20)+(9+25+1)+10-20)
			})
		})

		Convey("When the pit-stop table is queried", func() {
			Convey("Then each band returns the official points", func() {
				So(cfg.PitStopPointsFor(1.95), ShouldEqual, 20)
				So(cfg.PitStopPointsFor(2.10), ShouldEqual, 10)
				So(cfg.PitStopPointsFor(2.30), ShouldEqual, 5)
				So(cfg.PitStopPointsFor(2.75), ShouldEqual, 2)
				So(cfg.PitStopPointsFor(3.10), ShouldEqual, 0)
			})
		})
	})
}
