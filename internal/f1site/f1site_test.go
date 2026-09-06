package f1site

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const indexPage = `
<a href="/en/results/2026/races/1292/netherlands/race-result">Netherlands</a>
<a href="/en/results/2026/races/1293/italy/race-result">Italy</a>
<a href="/en/results/2026/races/1293/italy/qualifying">Italy</a>
<a href="/en/results/2025/races/1276/abu-dhabi/race-result">old</a>
<a href="/en/results/2026/races/1308/bahrain/race-result">Bahrain</a>
`

const gridPage = `
<table>
<tr><th>Pos.</th><th>No.</th><th>Driver</th><th>Team</th><th>Time</th></tr>
<tr><td>1</td><td>10</td><td><span>Pierre</span> <span>Gasly</span> <span>GAS</span></td><td>Alpine</td><td>1:21.786</td></tr>
<tr><td>2</td><td>63</td><td>George Russell RUS</td><td>Mercedes</td><td>1:21.846</td></tr>
<tr><td>20</td><td>12</td><td>Kimi Antonelli ANT</td><td>Mercedes</td><td>1:22.093</td></tr>
</table>
`

func TestParseIndex(t *testing.T) {
	Convey("Given a season results index page", t, func() {
		Convey("When the links are parsed for 2026", func() {
			races := ParseIndex(indexPage, 2026)
			Convey("Then each 2026 race appears once with its id and slug", func() {
				So(races, ShouldResemble, []Race{{1292, "netherlands"}, {1293, "italy"}, {1308, "bahrain"}})
			})
		})
		Convey("When a race name is matched", func() {
			r, ok := MatchRace(ParseIndex(indexPage, 2026), "Italian Grand Prix")
			Convey("Then the Italian slug resolves", func() {
				So(ok, ShouldBeTrue)
				So(r.ID, ShouldEqual, 1293)
			})
			_, ok = MatchRace(ParseIndex(indexPage, 2026), "Monaco Grand Prix")
			Convey("And an absent race does not match", func() {
				So(ok, ShouldBeFalse)
			})
		})
	})
}

func TestParseGrid(t *testing.T) {
	Convey("Given a starting-grid page", t, func() {
		Convey("When the table is parsed", func() {
			rows := ParseGrid(gridPage)
			Convey("Then every car row yields position, number, code, and team", func() {
				So(rows, ShouldHaveLength, 3)
				So(rows[0], ShouldResemble, GridRow{Position: 1, Number: 10, Code: "GAS", Team: "Alpine"})
				So(rows[2], ShouldResemble, GridRow{Position: 20, Number: 12, Code: "ANT", Team: "Mercedes"})
			})
		})
		Convey("When the page has no table", func() {
			Convey("Then the result is nil", func() {
				So(ParseGrid("<html><body>Not yet</body></html>"), ShouldBeNil)
			})
		})
	})
}
