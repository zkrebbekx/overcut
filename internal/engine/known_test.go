package engine

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/zkrebbekx/overcut/internal/dataset"
)

func TestWithKnownWeekendSprintGrid(t *testing.T) {
	Convey("Given a sprint round with a stored sprint qualifying order and no sprint result", t, func() {
		r := dataset.Round{Round: 17, HasSprint: true, SprintQuali: map[string]int{"VER": 1, "RUS": 2}}

		Convey("When the caller gives no sprint grid", func() {
			cond, k := withKnownWeekend(r, Conditions{})
			Convey("Then the sprint grid comes from the data", func() {
				So(cond.SprintGrid, ShouldResemble, []string{"VER", "RUS"})
				So(k.sprintGrid, ShouldBeTrue)
			})
		})

		Convey("When the caller gives a sprint grid", func() {
			cond, k := withKnownWeekend(r, Conditions{SprintGrid: []string{"RUS", "VER"}})
			Convey("Then the caller's grid is kept", func() {
				So(cond.SprintGrid, ShouldResemble, []string{"RUS", "VER"})
				So(k.sprintGrid, ShouldBeFalse)
			})
		})
	})
}
