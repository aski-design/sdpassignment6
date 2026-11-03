package main

import (
	"fmt"
	"strings"
)

type noLeaves struct{}

func (noLeaves) VisitRoom(*Room)             {}
func (noLeaves) VisitSuite(*Suite)           {}
func (noLeaves) VisitHall(*Hall)             {}
func (noLeaves) VisitRestaurant(*Restaurant) {}
func (noLeaves) VisitSpa(*Spa)               {}

type RevenueVisitor struct {
	noLeaves
	total float64
}

func (v *RevenueVisitor) VisitHotel(h *Hotel) { visitChildren(h.Children, v) }
func (v *RevenueVisitor) VisitFloor(f *Floor) { visitChildren(f.Children, v) }
func (v *RevenueVisitor) VisitRoom(r *Room) {
	if r.Occupied {
		v.total += r.RatePerNight
	}
}
func (v *RevenueVisitor) VisitSuite(s *Suite) {
	if s.Occupied {
		v.total += s.RatePerNight
	}
}
func (v *RevenueVisitor) VisitHall(h *Hall) {
	if h.BookedToday {
		v.total += h.RatePerDay
	}
}
func (v *RevenueVisitor) VisitRestaurant(r *Restaurant) { v.total += r.AvgDailyRevenue }
func (v *RevenueVisitor) VisitSpa(s *Spa)               { v.total += s.AvgDailyRevenue }
func (v *RevenueVisitor) Result() float64               { return v.total }

type OccupancyReport struct {
	Total    int
	Occupied int
}

type OccupancyVisitor struct {
	noLeaves
	report OccupancyReport
}

func (v *OccupancyVisitor) count(occupied bool) {
	v.report.Total++
	if occupied {
		v.report.Occupied++
	}
}

func (v *OccupancyVisitor) VisitHotel(h *Hotel) { visitChildren(h.Children, v) }
func (v *OccupancyVisitor) VisitFloor(f *Floor) { visitChildren(f.Children, v) }
func (v *OccupancyVisitor) VisitRoom(r *Room)   { v.count(r.Occupied) }
func (v *OccupancyVisitor) VisitSuite(s *Suite) { v.count(s.Occupied) }
func (v *OccupancyVisitor) Result() OccupancyReport {
	return v.report
}

type CleaningPlanVisitor struct {
	noLeaves
	numbers []string
}

func NewCleaningPlanVisitor() *CleaningPlanVisitor {
	return &CleaningPlanVisitor{numbers: []string{}}
}

func (v *CleaningPlanVisitor) check(number string, occupied, dirty bool) {
	if !occupied && dirty {
		v.numbers = append(v.numbers, number)
	}
}

func (v *CleaningPlanVisitor) VisitHotel(h *Hotel) { visitChildren(h.Children, v) }
func (v *CleaningPlanVisitor) VisitFloor(f *Floor) { visitChildren(f.Children, v) }
func (v *CleaningPlanVisitor) VisitRoom(r *Room) {
	v.check(r.Number, r.Occupied, r.Dirty)
}
func (v *CleaningPlanVisitor) VisitSuite(s *Suite) {
	v.check(s.Number, s.Occupied, s.Dirty)
}
func (v *CleaningPlanVisitor) Result() []string { return v.numbers }

type JSONExportVisitor struct {
	out strings.Builder
}

func (j *JSONExportVisitor) writeList(children []Element) {
	for i, c := range children {
		if i > 0 {
			j.out.WriteString(",")
		}
		c.Accept(j)
	}
}

func (j *JSONExportVisitor) VisitHotel(h *Hotel) {
	fmt.Fprintf(&j.out, `{"type":"hotel","name":%q,"children":[`, h.Name)
	j.writeList(h.Children)
	j.out.WriteString("]}")
}

func (j *JSONExportVisitor) VisitFloor(f *Floor) {
	fmt.Fprintf(&j.out, `{"type":"floor","id":%q,"children":[`, f.ID)
	j.writeList(f.Children)
	j.out.WriteString("]}")
}

func (j *JSONExportVisitor) VisitRoom(r *Room) {
	fmt.Fprintf(&j.out, `{"type":"room","number":%q,"ratePerNight":%g,"occupied":%t,"dirty":%t}`,
		r.Number, r.RatePerNight, r.Occupied, r.Dirty)
}

func (j *JSONExportVisitor) VisitSuite(s *Suite) {
	fmt.Fprintf(&j.out, `{"type":"suite","number":%q,"ratePerNight":%g,"occupied":%t,"dirty":%t}`,
		s.Number, s.RatePerNight, s.Occupied, s.Dirty)
}

func (j *JSONExportVisitor) VisitHall(h *Hall) {
	fmt.Fprintf(&j.out, `{"type":"hall","name":%q,"ratePerDay":%g,"bookedToday":%t}`,
		h.Name, h.RatePerDay, h.BookedToday)
}

func (j *JSONExportVisitor) VisitRestaurant(r *Restaurant) {
	fmt.Fprintf(&j.out, `{"type":"restaurant","name":%q,"avgDailyRevenue":%g}`, r.Name, r.AvgDailyRevenue)
}

func (j *JSONExportVisitor) VisitSpa(s *Spa) {
	fmt.Fprintf(&j.out, `{"type":"spa","name":%q,"avgDailyRevenue":%g}`, s.Name, s.AvgDailyRevenue)
}

func (j *JSONExportVisitor) Result() string { return j.out.String() }

type MaxRateVisitor struct {
	noLeaves
	max float64
}

func (v *MaxRateVisitor) VisitHotel(h *Hotel) { visitChildren(h.Children, v) }
func (v *MaxRateVisitor) VisitFloor(f *Floor) { visitChildren(f.Children, v) }
func (v *MaxRateVisitor) VisitRoom(r *Room) {
	if r.RatePerNight > v.max {
		v.max = r.RatePerNight
	}
}
func (v *MaxRateVisitor) VisitSuite(s *Suite) {
	if s.RatePerNight > v.max {
		v.max = s.RatePerNight
	}
}
func (v *MaxRateVisitor) Result() float64 { return v.max }

var _ ResultVisitor[float64] = (*RevenueVisitor)(nil)
var _ ResultVisitor[OccupancyReport] = (*OccupancyVisitor)(nil)
var _ ResultVisitor[[]string] = (*CleaningPlanVisitor)(nil)
var _ ResultVisitor[string] = (*JSONExportVisitor)(nil)
var _ ResultVisitor[float64] = (*MaxRateVisitor)(nil)
