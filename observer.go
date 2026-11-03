package main

import "sync"

type EventType string

const (
	BookingCreated EventType = "booking.created"
	RoomCheckedOut EventType = "room.checkedout"
	RevenueSummary EventType = "revenue.summary"
	CleaningNeeded EventType = "alert.cleaning"
	CleaningPlan   EventType = "task.cleaning"
)

type Event struct {
	eventType EventType
	floorID   string
	roomID    string
	value     float64
	message   string
}

func NewEvent(t EventType, floorID, roomID string, value float64, message string) Event {
	return Event{eventType: t, floorID: floorID, roomID: roomID, value: value, message: message}
}

func (e Event) Type() EventType { return e.eventType }
func (e Event) FloorID() string { return e.floorID }
func (e Event) RoomID() string  { return e.roomID }
func (e Event) Value() float64  { return e.value }
func (e Event) Message() string { return e.message }

type Typed interface {
	Type() EventType
}

type Observer[E Typed] interface {
	Notify(e E)
}

type Subject[E Typed] interface {
	Subscribe(t EventType, o Observer[E])
	Unsubscribe(t EventType, o Observer[E])
}

type Registry[E Typed] struct {
	mu        sync.RWMutex
	listeners map[EventType][]Observer[E]
}

func NewRegistry[E Typed]() *Registry[E] {
	return &Registry[E]{listeners: make(map[EventType][]Observer[E])}
}

func (r *Registry[E]) Subscribe(t EventType, o Observer[E]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.listeners[t]
	updated := make([]Observer[E], len(old), len(old)+1)
	copy(updated, old)
	r.listeners[t] = append(updated, o)
}

func (r *Registry[E]) Unsubscribe(t EventType, o Observer[E]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.listeners[t]
	updated := make([]Observer[E], 0, len(old))
	for _, existing := range old {
		if existing != o {
			updated = append(updated, existing)
		}
	}
	r.listeners[t] = updated
}

func (r *Registry[E]) Publish(e E) {
	r.mu.RLock()
	snapshot := r.listeners[e.Type()]
	r.mu.RUnlock()

	for _, o := range snapshot {
		o.Notify(e)
	}
}

type BookingSystem struct {
	*Registry[Event]
}

func NewBookingSystem() *BookingSystem {
	return &BookingSystem{Registry: NewRegistry[Event]()}
}

func (b *BookingSystem) Book(floorID, roomID, guest string, amount float64) {
	b.Publish(NewEvent(BookingCreated, floorID, roomID, amount, guest))
}

type RoomStatusFeed struct {
	*Registry[Event]
}

func NewRoomStatusFeed() *RoomStatusFeed {
	return &RoomStatusFeed{Registry: NewRegistry[Event]()}
}

func (f *RoomStatusFeed) CheckOut(floorID, roomID string) {
	f.Publish(NewEvent(RoomCheckedOut, floorID, roomID, 0, ""))
}

var _ Subject[Event] = (*BookingSystem)(nil)
var _ Subject[Event] = (*RoomStatusFeed)(nil)
