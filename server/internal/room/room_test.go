package room

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/pattaradol9/game24/server/internal/game"
)

// fakeConn records delivered messages.
type fakeConn struct {
	mu   sync.Mutex
	msgs []sent
}

type sent struct {
	typ  string
	data map[string]any
}

func (c *fakeConn) Deliver(raw []byte) {
	var env struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(raw, &env)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, sent{typ: env.Type, data: env.Data})
}

func (c *fakeConn) CloseConn() {}

func (c *fakeConn) count(typ string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, m := range c.msgs {
		if m.typ == typ {
			n++
		}
	}
	return n
}

func (r *Room) snapshotForTest() stateView {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotLocked()
}

func newTestRoom(t *testing.T, mode game.Mode) (*Room, string) {
	t.Helper()
	rm, hostKey, err := NewHub(nil).Create(Config{Mode: mode, Rounds: 12, HintQuota: 3, RegenQuota: 2})
	if err != nil {
		t.Fatal(err)
	}
	return rm, hostKey
}

func TestJoinAndHostKey(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	host := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Host", HostKey: hostKey}, host)
	guest := &fakeConn{}
	rm.Join(Info{SessionID: "s2", Name: "Guest"}, guest)

	if got := rm.snapshotForTest().Host; got != "s1" {
		t.Fatalf("host = %s, want s1", got)
	}
	if rm.PlayerCount() != 2 {
		t.Fatalf("players = %d", rm.PlayerCount())
	}
	if host.count("room_state") == 0 || guest.count("room_state") == 0 {
		t.Fatal("room_state not broadcast on join")
	}
}

func TestFirstCorrectSolverWins(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	host, p2 := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, host)
	rm.Join(Info{SessionID: "p2", Name: "Two"}, p2)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}

	rm.mu.Lock()
	numbers := rm.sharedNumbers
	rm.mu.Unlock()
	if numbers == nil {
		t.Fatal("no hand dealt")
	}
	sols := game.Solve(numbers)
	if len(sols) < 2 {
		t.Fatalf("want >=2 solutions to test racing, got %d for %v", len(sols), numbers)
	}

	if err := rm.Submit("p2", sols[0].Trace); err != nil {
		t.Fatalf("p2 submit: %v", err)
	}
	if err := rm.Submit("h", sols[1].Trace); err == nil {
		t.Fatal("late submit accepted")
	}
	if host.count("round_result") != 1 || p2.count("round_result") != 1 {
		t.Fatalf("round_result counts: host=%d p2=%d", host.count("round_result"), p2.count("round_result"))
	}
	rm.mu.Lock()
	if rm.solvedBy != "p2" {
		t.Fatalf("solvedBy = %s", rm.solvedBy)
	}
	rm.mu.Unlock()
}

func TestQuotasAreIdenticalForEveryone(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	host, p2 := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, host)
	rm.Join(Info{SessionID: "p2", Name: "Two"}, p2)

	snap := rm.snapshotForTest()
	for _, pv := range snap.Players {
		if pv.HintsLeft != 3 || pv.RegensLeft != 2 {
			t.Fatalf("quota not equal: %+v", pv)
		}
	}
}

func TestHintAndRegenQuota(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	conn := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, conn)
	if err := rm.Start("s1"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := rm.Hint("s1"); err != nil {
			t.Fatalf("hint %d: %v", i, err)
		}
	}
	if err := rm.Hint("s1"); err == nil {
		t.Fatal("hint quota exceeded")
	}
	for i := 0; i < 2; i++ {
		if err := rm.Regen("s1"); err != nil {
			t.Fatalf("regen %d: %v", i, err)
		}
	}
	if err := rm.Regen("s1"); err == nil {
		t.Fatal("regen quota exceeded")
	}
	if conn.count("hint") != 3 || conn.count("regen") != 2 {
		t.Fatalf("personal messages: hint=%d regen=%d", conn.count("hint"), conn.count("regen"))
	}
}

func TestMatchFinishesAfterAllRounds(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	rm.mu.Lock()
	rm.summaryDur = 5 * time.Millisecond
	rm.mu.Unlock()
	conn := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, conn)
	rm.Start("s1")

	deadline := time.Now().Add(30 * time.Second)
	for {
		rm.mu.Lock()
		state, roundNo := rm.state, rm.roundNo
		over := rm.roundOver
		rm.mu.Unlock()

		if state == StateFinished {
			break
		}
		if state == StateRound && !over {
			rm.mu.Lock()
			numbers := rm.numbers["s1"]
			rm.mu.Unlock()
			sols := game.Solve(numbers)
			if len(sols) > 0 {
				if err := rm.Submit("s1", sols[0].Trace); err != nil {
					t.Fatalf("submit round %d: %v", roundNo, err)
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("match did not finish in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if conn.count("round_result") != 12 {
		t.Fatalf("round_result = %d, want 12", conn.count("round_result"))
	}
	if conn.count("match_end") != 1 {
		t.Fatalf("match_end = %d", conn.count("match_end"))
	}
}

func TestLeaveCleansUp(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	c := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, c)
	rm.Leave("s1")
	time.Sleep(10 * time.Millisecond)
	if rm.PlayerCount() != 0 {
		t.Fatalf("players after leave = %d", rm.PlayerCount())
	}
}
