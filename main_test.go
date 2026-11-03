package main

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type recorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *recorder) Notify(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

type selfLeaving struct {
	reg *Registry[Event]
	got int
}

func (s *selfLeaving) Notify(e Event) {
	s.got++
	s.reg.Unsubscribe(e.Type(), s)
}

type joiner struct {
	reg    *Registry[Event]
	late   *recorder
	joined bool
}

func (j *joiner) Notify(e Event) {
	if !j.joined {
		j.joined = true
		j.reg.Subscribe(e.Type(), j.late)
	}
}

type counter struct{ n atomic.Int64 }

func (c *counter) Notify(e Event) { c.n.Add(1) }

func bookingEvent(guest string, amount float64) Event {
	return NewEvent(BookingCreated, "F1", "101", amount, guest)
}

func TestEventKeepsItsData(t *testing.T) {
	e := NewEvent(CleaningNeeded, "F2", "202", 3, "dirty")
	if e.Type() != CleaningNeeded || e.FloorID() != "F2" || e.RoomID() != "202" || e.Value() != 3 || e.Message() != "dirty" {
		t.Fatalf("event lost data: %+v", e)
	}
}

func TestSubscribedListenerGetsEvent(t *testing.T) {
	reg := NewRegistry[Event]()
	rec := &recorder{}
	reg.Subscribe(BookingCreated, rec)
	reg.Publish(bookingEvent("Bob", 100))
	if rec.count() != 1 {
		t.Fatalf("want 1 event, got %d", rec.count())
	}
}

func TestListenerOnlyGetsItsOwnEventType(t *testing.T) {
	reg := NewRegistry[Event]()
	rec := &recorder{}
	reg.Subscribe(RoomCheckedOut, rec)
	reg.Publish(bookingEvent("Bob", 100))
	if rec.count() != 0 {
		t.Fatalf("listener got an event of another type")
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	reg := NewRegistry[Event]()
	rec := &recorder{}
	reg.Subscribe(BookingCreated, rec)
	reg.Publish(bookingEvent("Bob", 1))
	reg.Unsubscribe(BookingCreated, rec)
	reg.Publish(bookingEvent("Bob", 2))
	if rec.count() != 1 {
		t.Fatalf("want 1 event, got %d", rec.count())
	}
}

func TestListenerCanUnsubscribeInsideItsHandler(t *testing.T) {
	reg := NewRegistry[Event]()
	l := &selfLeaving{reg: reg}
	reg.Subscribe(BookingCreated, l)
	reg.Publish(bookingEvent("Bob", 1))
	reg.Publish(bookingEvent("Bob", 2))
	if l.got != 1 {
		t.Fatalf("listener should get exactly 1 event, got %d", l.got)
	}
}

func TestListenerCanSubscribeInsideItsHandler(t *testing.T) {
	reg := NewRegistry[Event]()
	late := &recorder{}
	reg.Subscribe(BookingCreated, &joiner{reg: reg, late: late})
	reg.Publish(bookingEvent("Bob", 1)) // joiner adds "late" here, late must not see this one
	reg.Publish(bookingEvent("Bob", 2))
	if late.count() != 1 {
		t.Fatalf("late listener should get only the second event, got %d", late.count())
	}
}

func TestPublishWithoutListenersIsFine(t *testing.T) {
	reg := NewRegistry[Event]()
	reg.Publish(bookingEvent("Bob", 1)) // must not panic
}

func TestRevenueManagerRepublishesBatchSum(t *testing.T) {
	manager := NewRevenueManager(2)
	rec := &recorder{}
	manager.Subscribe(RevenueSummary, rec)
	manager.Notify(bookingEvent("A", 100))
	manager.Notify(bookingEvent("B", 300))
	if rec.count() != 1 {
		t.Fatalf("want 1 summary, got %d", rec.count())
	}
	if rec.events[0].Value() != 400 {
		t.Fatalf("want sum 400, got %v", rec.events[0].Value())
	}
}

func TestGuestAppConfirmsOnceAndLeaves(t *testing.T) {
	bookings := NewBookingSystem()
	app := NewGuestApp("Alice", bookings)
	bookings.Subscribe(BookingCreated, app)

	bookings.Book("F1", "101", "Bob", 100)
	if app.Confirmed() {
		t.Fatal("app must ignore bookings of other guests")
	}
	bookings.Book("F1", "101", "Alice", 100)
	if !app.Confirmed() {
		t.Fatal("app should confirm Alice's booking")
	}
	if len(bookings.listeners[BookingCreated]) != 0 {
		t.Fatalf("guest app should have left the booking system")
	}
}

func TestRevenueWholeHotel(t *testing.T) {
	hotel, _ := buildHotel()
	// F1: 100 + 500 + 1000 = 1600, F2: 120 + 300 + 400 = 820, F3: 350 + 0 = 350
	if got := Run[float64](hotel, &RevenueVisitor{}); got != 2770 {
		t.Fatalf("want 2770, got %v", got)
	}
}

func TestRevenueOfOneFloorOnly(t *testing.T) {
	_, floors := buildHotel()
	if got := Run[float64](floors["F2"], &RevenueVisitor{}); got != 820 {
		t.Fatalf("want 820, got %v", got)
	}
}

func TestOccupancyReport(t *testing.T) {
	hotel, _ := buildHotel()
	// rooms and suites: 101 102 201 202 210 301 302 = 7, occupied: 101 201 210 302 = 4
	got := Run[OccupancyReport](hotel, &OccupancyVisitor{})
	if got.Total != 7 || got.Occupied != 4 {
		t.Fatalf("want 4 of 7, got %d of %d", got.Occupied, got.Total)
	}
}

func TestCleaningPlanWholeHotelKeepsTreeOrder(t *testing.T) {
	hotel, _ := buildHotel()
	got := Run[[]string](hotel, NewCleaningPlanVisitor())
	if strings.Join(got, ",") != "102,202,301" {
		t.Fatalf("want 102,202,301 got %v", got)
	}
}

func TestCleaningPlanOfOneFloorOnly(t *testing.T) {
	_, floors := buildHotel()
	got := Run[[]string](floors["F2"], NewCleaningPlanVisitor())
	if strings.Join(got, ",") != "202" {
		t.Fatalf("want 202, got %v", got)
	}
}

func TestJSONExactForSmallTree(t *testing.T) {
	floor := &Floor{ID: "X", Children: []Element{
		&Room{Number: "1", RatePerNight: 50, Occupied: true, Dirty: false},
	}}
	want := `{"type":"floor","id":"X","children":[{"type":"room","number":"1","ratePerNight":50,"occupied":true,"dirty":false}]}`
	if got := Run[string](floor, &JSONExportVisitor{}); got != want {
		t.Fatalf("\nwant %s\ngot  %s", want, got)
	}
}

func TestJSONIsValidAndNestedCorrectly(t *testing.T) {
	hotel, _ := buildHotel()
	out := Run[string](hotel, &JSONExportVisitor{})

	var root map[string]any
	if err := json.Unmarshal([]byte(out), &root); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	floors := root["children"].([]any)
	firstFloor := floors[0].(map[string]any)["children"].([]any)
	secondFloor := floors[1].(map[string]any)["children"].([]any)
	if len(floors) != 3 || len(firstFloor) != 4 || len(secondFloor) != 4 {
		t.Fatalf("wrong nesting: %d floors, %d and %d children", len(floors), len(firstFloor), len(secondFloor))
	}
}

func TestMaxRateVisitor(t *testing.T) {
	hotel, _ := buildHotel()
	if got := Run[float64](hotel, &MaxRateVisitor{}); got != 350 {
		t.Fatalf("want 350, got %v", got)
	}
}

// ---- Integration test ----

func TestCheckoutsTriggerCleaningPlanOfThatFloorOnly(t *testing.T) {
	sys := NewSystem()
	for i := 0; i < 3; i++ {
		sys.Feed.CheckOut("F2", "202")
	}

	plan := ""
	for _, m := range sys.FrontDesk.Messages() {
		if strings.HasPrefix(m, string(CleaningPlan)) {
			plan = m
		}
	}
	if plan == "" {
		t.Fatal("no cleaning plan reached the front desk")
	}
	if !strings.Contains(plan, "202") {
		t.Errorf("plan should contain room 202: %s", plan)
	}
	if strings.Contains(plan, "102") || strings.Contains(plan, "301") {
		t.Errorf("other floors must not be visited: %s", plan)
	}
}

func TestPoolRunsEveryTaskBeforeShutdownReturns(t *testing.T) {
	var done atomic.Int64
	pool := NewPool(3)
	for i := 0; i < 100; i++ {
		pool.Submit(func() { done.Add(1) })
	}
	pool.Shutdown()
	if done.Load() != 100 {
		t.Fatalf("want 100 finished tasks, got %d", done.Load())
	}
}

func TestStress4ProducersTimes1000Bookings(t *testing.T) {
	sys := NewSystem()
	pool := NewPool(4)
	for p := 0; p < 4; p++ {
		pool.Submit(func() {
			for i := 0; i < 1000; i++ {
				sys.Bookings.Book("F1", "101", "walk-in", 100)
			}
		})
	}
	pool.Shutdown()

	if got := sys.Stats.Count(BookingCreated); got != 4000 {
		t.Errorf("bookings: want 4000, got %d", got)
	}
	if got := sys.Revenue.Total(); got != 400000 {
		t.Errorf("revenue: want 400000, got %v", got)
	}
	// one summary per 10 bookings
	if got := len(sys.FrontDesk.Messages()); got != 400 {
		t.Errorf("summaries: want 400, got %d", got)
	}
}

func TestConcurrentCheckoutsGiveExactAlertCount(t *testing.T) {
	sys := NewSystem()
	pool := NewPool(4)
	for p := 0; p < 4; p++ {
		pool.Submit(func() {
			for i := 0; i < 900; i++ {
				sys.Feed.CheckOut("F2", "202")
			}
		})
	}
	pool.Shutdown()

	if got := sys.Stats.Count(RoomCheckedOut); got != 3600 {
		t.Errorf("check-outs: want 3600, got %d", got)
	}
	if got := sys.Stats.Count(CleaningNeeded); got != 1200 {
		t.Errorf("alerts: want 1200, got %d", got)
	}
	if got := sys.Stats.Count(CleaningPlan); got != 1200 {
		t.Errorf("plans: want 1200, got %d", got)
	}
	// the front desk gets every alert and every plan
	if got := len(sys.FrontDesk.Messages()); got != 2400 {
		t.Errorf("front desk messages: want 2400, got %d", got)
	}
}

func TestSubscribingWhilePublishingLosesNothing(t *testing.T) {
	sys := NewSystem()
	extra := &counter{}
	stop := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		defer close(finished)
		for {
			select {
			case <-stop:
				return
			default:
				sys.Bookings.Subscribe(BookingCreated, extra)
				sys.Bookings.Unsubscribe(BookingCreated, extra)
			}
		}
	}()

	pool := NewPool(4)
	for p := 0; p < 4; p++ {
		pool.Submit(func() {
			for i := 0; i < 1000; i++ {
				sys.Bookings.Book("F3", "301", "walk-in", 100)
			}
		})
	}
	pool.Shutdown()
	close(stop)
	<-finished

	if got := sys.Stats.Count(BookingCreated); got != 4000 {
		t.Fatalf("want 4000, got %d", got)
	}
}
