// Package presence tracks the live per-player event streams (one per open
// tab) and fans profile pushes out to every stream of the target player.
// It is what carries admin adjustments — EXP, coins, tier, achievements,
// renames, deletions — to a signed-in player without a page refresh.
package presence

import "sync"

// Event is one typed push to a player's streams. Name becomes the SSE event
// name; Data is the pre-marshaled JSON body.
type Event struct {
	Name string
	Data []byte
}

// sub is one live stream. The channel is buffered so a short burst of
// publishes never blocks the mutating handler; on overflow the sub is closed
// instead of stalled — its stream ends and the client reconnects, picking up
// a fresh snapshot on arrival.
type sub struct {
	ch   chan Event
	once sync.Once
}

// Sub is the publisher-side handle to one subscribed stream.
type Sub struct {
	s *sub
}

// C exposes the event channel; it closes once the sub is closed.
func (s Sub) C() <-chan Event { return s.s.ch }

const subBuffer = 16

// Broker holds the playerID -> live streams registry.
type Broker struct {
	mu   sync.Mutex
	subs map[string]map[*sub]struct{}
}

// NewBroker returns an empty registry.
func NewBroker() *Broker {
	return &Broker{subs: map[string]map[*sub]struct{}{}}
}

// Subscribe registers a new stream for the player.
func (b *Broker) Subscribe(playerID string) Sub {
	s := &sub{ch: make(chan Event, subBuffer)}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[playerID] == nil {
		b.subs[playerID] = map[*sub]struct{}{}
	}
	b.subs[playerID][s] = struct{}{}
	return Sub{s}
}

// Unsubscribe removes the stream. The channel closes so a blocked receiver
// wakes up; removal happens under the lock, so no later Publish reaches it.
func (b *Broker) Unsubscribe(playerID string, s Sub) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.drop(playerID, s.s)
}

// Count reports how many live streams the player has open (multiple tabs
// each hold one). Mainly for tests and diagnostics.
func (b *Broker) Count(playerID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs[playerID])
}

// Publish delivers the event to every live stream of the player. Delivery
// is best-effort and never blocks: a stream whose buffer is full is dropped
// and left to resync through a reconnect.
func (b *Broker) Publish(playerID string, ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs[playerID] {
		select {
		case s.ch <- ev:
		default:
			b.drop(playerID, s)
		}
	}
}

// Broadcast delivers the event to the live streams of every player — for
// server-wide notices (the global EXP boost) that are not tied to one
// profile. Same best-effort delivery as Publish.
func (b *Broker) Broadcast(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for playerID, set := range b.subs {
		for s := range set {
			select {
			case s.ch <- ev:
			default:
				b.drop(playerID, s)
			}
		}
	}
}

// drop removes and closes one stream. Callers must hold b.mu.
func (b *Broker) drop(playerID string, s *sub) {
	set, ok := b.subs[playerID]
	if !ok {
		return
	}
	if _, live := set[s]; !live {
		return
	}
	delete(set, s)
	if len(set) == 0 {
		delete(b.subs, playerID)
	}
	s.once.Do(func() { close(s.ch) })
}
