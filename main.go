package main

import (
	"flag"
	"fmt"
	"strings"
	"sync"
)

type Pool struct {
	tasks chan func()
	wg    sync.WaitGroup
}

func NewPool(size int) *Pool {
	p := &Pool{tasks: make(chan func())}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go p.work()
	}
	return p
}

func (p *Pool) work() {
	defer p.wg.Done()
	for task := range p.tasks {
		task()
	}
}

func (p *Pool) Submit(task func()) { p.tasks <- task }

func (p *Pool) Shutdown() {
	close(p.tasks)
	p.wg.Wait()
}

type System struct {
	Hotel     *Hotel
	Bookings  *BookingSystem
	Feed      *RoomStatusFeed
	Alerts    *Registry[Event]
	Stats     *Statistics
	FrontDesk *FrontDesk
	Revenue   *RevenueManager
}

func NewSystem() *System {
	hotel, floors := buildHotel()

	bookings := NewBookingSystem()
	feed := NewRoomStatusFeed()
	alerts := NewRegistry[Event]()

	stats := NewStatistics()
	frontDesk := NewFrontDesk()
	guestApp := NewGuestApp("Alice", bookings)
	revenue := NewRevenueManager(10)
	housekeeping := NewHousekeeping(3, alerts)
	planner := NewCleaningPlanner(floors, alerts)

	bookings.Subscribe(BookingCreated, stats)
	bookings.Subscribe(BookingCreated, guestApp)
	bookings.Subscribe(BookingCreated, revenue)
	feed.Subscribe(RoomCheckedOut, stats)
	feed.Subscribe(RoomCheckedOut, housekeeping)
	revenue.Subscribe(RevenueSummary, frontDesk)
	alerts.Subscribe(CleaningNeeded, stats)
	alerts.Subscribe(CleaningNeeded, planner)
	alerts.Subscribe(CleaningNeeded, frontDesk)
	alerts.Subscribe(CleaningPlan, stats)
	alerts.Subscribe(CleaningPlan, frontDesk)

	return &System{
		Hotel:     hotel,
		Bookings:  bookings,
		Feed:      feed,
		Alerts:    alerts,
		Stats:     stats,
		FrontDesk: frontDesk,
		Revenue:   revenue,
	}
}
func buildHotel() (*Hotel, map[string]*Floor) {
	f1 := &Floor{ID: "F1", Children: []Element{
		&Room{Number: "101", RatePerNight: 100, Occupied: true, Dirty: false},
		&Room{Number: "102", RatePerNight: 100, Occupied: false, Dirty: true},
		&Hall{Name: "Ballroom", RatePerDay: 500, BookedToday: true},
		&Restaurant{Name: "Bistro", AvgDailyRevenue: 1000},
	}}
	f2 := &Floor{ID: "F2", Children: []Element{
		&Room{Number: "201", RatePerNight: 120, Occupied: true, Dirty: false},
		&Room{Number: "202", RatePerNight: 120, Occupied: false, Dirty: true},
		&Suite{Number: "210", RatePerNight: 300, Occupied: true, Dirty: false},
		&Spa{Name: "Wellness", AvgDailyRevenue: 400},
	}}
	f3 := &Floor{ID: "F3", Children: []Element{
		&Suite{Number: "301", RatePerNight: 350, Occupied: false, Dirty: true},
		&Suite{Number: "302", RatePerNight: 350, Occupied: true, Dirty: false},
		&Hall{Name: "Boardroom", RatePerDay: 200, BookedToday: false},
	}}

	hotel := &Hotel{Name: "Grand Plaza", Children: []Element{f1, f2, f3}}
	floors := map[string]*Floor{"F1": f1, "F2": f2, "F3": f3}
	return hotel, floors
}

func produce(sys *System, floorID, roomID string, count int) {
	for i := 0; i < count; i++ {
		sys.Bookings.Book(floorID, roomID, "walk-in", 100)
		sys.Feed.CheckOut(floorID, roomID)
	}
}

func main() {
	workers := flag.Int("workers", 4, "number of producer goroutines")
	events := flag.Int("events", 1000, "bookings and check-outs per producer")
	flag.Parse()

	sys := NewSystem()
	floorIDs := []string{"F1", "F2", "F3"}
	roomIDs := []string{"102", "202", "301"}

	pool := NewPool(*workers)
	for w := 0; w < *workers; w++ {
		floorID := floorIDs[w%len(floorIDs)]
		roomID := roomIDs[w%len(roomIDs)]
		pool.Submit(func() { produce(sys, floorID, roomID, *events) })
	}
	pool.Shutdown()

	fmt.Println("bookings:        ", sys.Stats.Count(BookingCreated))
	fmt.Println("check-outs:      ", sys.Stats.Count(RoomCheckedOut))
	fmt.Println("cleaning alerts: ", sys.Stats.Count(CleaningNeeded))
	fmt.Println("cleaning plans:  ", sys.Stats.Count(CleaningPlan))
	fmt.Printf("booked revenue:   %.2f\n", sys.Revenue.Total())
	fmt.Println("front desk msgs: ", len(sys.FrontDesk.Messages()))

	fmt.Println("daily revenue:   ", Run[float64](sys.Hotel, &RevenueVisitor{}))
	report := Run[OccupancyReport](sys.Hotel, &OccupancyVisitor{})
	fmt.Printf("occupancy:        %d of %d rooms and suites\n", report.Occupied, report.Total)
	fmt.Println("cleaning plan:   ", strings.Join(Run[[]string](sys.Hotel, NewCleaningPlanVisitor()), ", "))
	fmt.Println("highest rate:    ", Run[float64](sys.Hotel, &MaxRateVisitor{}))
	fmt.Println(Run[string](sys.Hotel, &JSONExportVisitor{}))
}
