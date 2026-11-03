package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type Statistics struct {
	mu     sync.Mutex
	counts map[EventType]int
}

func NewStatistics() *Statistics {
	return &Statistics{counts: make(map[EventType]int)}
}

func (s *Statistics) Notify(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[e.Type()]++
}

func (s *Statistics) Count(t EventType) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[t]
}

type FrontDesk struct {
	mu       sync.Mutex
	messages []string
}

func NewFrontDesk() *FrontDesk {
	return &FrontDesk{messages: []string{}}
}

func (f *FrontDesk) Notify(e Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, string(e.Type())+": "+e.Message())
}

func (f *FrontDesk) Messages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]string, len(f.messages))
	copy(copied, f.messages)
	return copied
}

type GuestApp struct {
	guest     string
	source    Subject[Event]
	confirmed atomic.Bool
}

func NewGuestApp(guest string, source Subject[Event]) *GuestApp {
	return &GuestApp{guest: guest, source: source}
}

func (g *GuestApp) Notify(e Event) {
	if e.Message() != g.guest {
		return
	}
	if !g.confirmed.CompareAndSwap(false, true) {
		return
	}
	g.source.Unsubscribe(BookingCreated, g)
}

func (g *GuestApp) Confirmed() bool { return g.confirmed.Load() }

type RevenueManager struct {
	*Registry[Event]
	lock      sync.Mutex
	batchSize int
	inBatch   int
	batchSum  float64
	total     float64
}

func NewRevenueManager(batchSize int) *RevenueManager {
	return &RevenueManager{Registry: NewRegistry[Event](), batchSize: batchSize}
}

func (r *RevenueManager) Notify(e Event) {
	r.lock.Lock()
	r.total += e.Value()
	r.batchSum += e.Value()
	r.inBatch++
	full := r.inBatch == r.batchSize
	sum := r.batchSum
	if full {
		r.inBatch = 0
		r.batchSum = 0
	}
	r.lock.Unlock()
	if full {
		msg := fmt.Sprintf("revenue of last %d bookings: %.2f", r.batchSize, sum)
		r.Publish(NewEvent(RevenueSummary, "", "", sum, msg))
	}
}

func (r *RevenueManager) Total() float64 {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.total
}

type Housekeeping struct {
	alerts    *Registry[Event]
	batchSize int
	lock      sync.Mutex
	counts    map[string]int
}

func NewHousekeeping(batchSize int, alerts *Registry[Event]) *Housekeeping {
	return &Housekeeping{alerts: alerts, batchSize: batchSize, counts: make(map[string]int)}
}

func (h *Housekeeping) Notify(e Event) {
	floor := e.FloorID()

	h.lock.Lock()
	h.counts[floor]++
	full := h.counts[floor] == h.batchSize
	if full {
		h.counts[floor] = 0
	}
	h.lock.Unlock()

	if full {
		msg := fmt.Sprintf("%d check-outs on floor %s", h.batchSize, floor)
		h.alerts.Publish(NewEvent(CleaningNeeded, floor, "", float64(h.batchSize), msg))
	}
}

type CleaningPlanner struct {
	floors map[string]*Floor
	alerts *Registry[Event]
}

func NewCleaningPlanner(floors map[string]*Floor, alerts *Registry[Event]) *CleaningPlanner {
	return &CleaningPlanner{floors: floors, alerts: alerts}
}

func (p *CleaningPlanner) Notify(e Event) {
	floor, ok := p.floors[e.FloorID()]
	if !ok {
		return
	}
	plan := Run[[]string](floor, NewCleaningPlanVisitor())
	msg := fmt.Sprintf("floor %s cleaning plan: %v", e.FloorID(), plan)
	p.alerts.Publish(NewEvent(CleaningPlan, e.FloorID(), "", float64(len(plan)), msg))
}
