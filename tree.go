package main

type Element interface {
	Accept(v Visitor)
}

type Visitor interface {
	VisitHotel(h *Hotel)
	VisitFloor(f *Floor)
	VisitRoom(r *Room)
	VisitSuite(s *Suite)
	VisitHall(h *Hall)
	VisitRestaurant(r *Restaurant)
	VisitSpa(s *Spa)
}

type ResultVisitor[R any] interface {
	Visitor
	Result() R
}

func Run[R any](root Element, v ResultVisitor[R]) R {
	root.Accept(v)
	return v.Result()
}

func visitChildren(children []Element, v Visitor) {
	for _, c := range children {
		c.Accept(v)
	}
}

type Hotel struct {
	Name     string
	Children []Element
}

type Floor struct {
	ID       string
	Children []Element
}

type Room struct {
	Number       string
	RatePerNight float64
	Occupied     bool
	Dirty        bool
}

type Suite struct {
	Number       string
	RatePerNight float64
	Occupied     bool
	Dirty        bool
}

type Hall struct {
	Name        string
	RatePerDay  float64
	BookedToday bool
}

type Restaurant struct {
	Name            string
	AvgDailyRevenue float64
}

type Spa struct {
	Name            string
	AvgDailyRevenue float64
}

func (h *Hotel) Accept(v Visitor)      { v.VisitHotel(h) }
func (f *Floor) Accept(v Visitor)      { v.VisitFloor(f) }
func (r *Room) Accept(v Visitor)       { v.VisitRoom(r) }
func (s *Suite) Accept(v Visitor)      { v.VisitSuite(s) }
func (h *Hall) Accept(v Visitor)       { v.VisitHall(h) }
func (r *Restaurant) Accept(v Visitor) { v.VisitRestaurant(r) }
func (s *Spa) Accept(v Visitor)        { v.VisitSpa(s) }
