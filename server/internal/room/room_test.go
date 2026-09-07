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

// first returns the data payload of the first message of the given type.
func (c *fakeConn) first(typ string) (map[string]any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if m.typ == typ {
			return m.data, true
		}
	}
	return nil, false
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
	old := playerGrace
	playerGrace = 40 * time.Millisecond
	defer func() { playerGrace = old }()

	rm, hostKey := newTestRoom(t, game.Queen)
	c := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, c)
	rm.Leave("s1", c)

	// the seat is held warm for a refresh before it is reaped
	if rm.PlayerCount() != 1 {
		t.Fatalf("seat dropped before grace: %d players", rm.PlayerCount())
	}
	time.Sleep(80 * time.Millisecond)
	if rm.PlayerCount() != 0 {
		t.Fatalf("absent seat not reaped after grace: %d players", rm.PlayerCount())
	}
}

func TestHubRemovalGraceLetsRefreshRejoin(t *testing.T) {
	old := removeGrace
	removeGrace = 40 * time.Millisecond
	defer func() { removeGrace = old }()

	hub := NewHub(nil)
	rm, hostKey, err := hub.Create(Config{Mode: game.Queen, Rounds: 12, HintQuota: 3, RegenQuota: 2})
	if err != nil {
		t.Fatal(err)
	}
	c := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, c)

	// the refresh: leave empties the room, but it must linger for rejoin
	rm.Leave("s1", c)
	if _, err := hub.Get(rm.Code()); err != nil {
		t.Fatalf("room dropped before grace expired: %v", err)
	}
	rm.Join(Info{SessionID: "s2", Name: "Solo"}, c)

	// the pending removal was cancelled by the rejoin
	time.Sleep(80 * time.Millisecond)
	if _, err := hub.Get(rm.Code()); err != nil {
		t.Fatalf("room dropped while occupied: %v", err)
	}

	// without a rejoin the room is removed once the grace elapses
	rm.Leave("s2", c)
	time.Sleep(80 * time.Millisecond)
	if _, err := hub.Get(rm.Code()); err == nil {
		t.Fatal("room still present after grace expired")
	}
}

func TestHostIsFixedAndRoomClosesWhenHostLeaves(t *testing.T) {
	old := hostGrace
	hostGrace = 40 * time.Millisecond
	defer func() { hostGrace = old }()

	hub := NewHub(nil)
	rm, hostKey, err := hub.Create(Config{Mode: game.Queen, Rounds: 12, HintQuota: 3, RegenQuota: 2})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Host", HostKey: hostKey}, host)
	guest := &fakeConn{}
	rm.Join(Info{SessionID: "s2", Name: "Guest"}, guest)

	// the host seat never migrates when the host disconnects
	rm.Leave("s1", host)
	if got := rm.snapshotForTest().Host; got != "s1" {
		t.Fatalf("host migrated on disconnect: %s", got)
	}
	// the room lingers so the host can refresh back in
	if _, err := hub.Get(rm.Code()); err != nil {
		t.Fatalf("room dropped during host grace: %v", err)
	}
	// the host rejoins (refresh): seat reclaimed, pending close cancelled
	rm.Join(Info{SessionID: "s3", Name: "Host", HostKey: hostKey}, host)
	if got := rm.snapshotForTest().Host; got != "s3" {
		t.Fatalf("host reclaim failed: %s", got)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := hub.Get(rm.Code()); err != nil {
		t.Fatal("room closed even though the host returned in time")
	}

	// when the host leaves for good the room closes and members are told
	rm.Leave("s3", host)
	time.Sleep(80 * time.Millisecond)
	if _, err := hub.Get(rm.Code()); err == nil {
		t.Fatal("room still open after host left for good")
	}
	if n := guest.count("room_closed"); n == 0 {
		t.Fatal("members were not notified with room_closed")
	}
}

func TestHostListedFirstAndReconnectNotice(t *testing.T) {
	hub := NewHub(nil)
	rm, hostKey, err := hub.Create(Config{Mode: game.Queen, Rounds: 12, HintQuota: 3, RegenQuota: 2})
	if err != nil {
		t.Fatal(err)
	}
	guest := &fakeConn{}
	hostConn := &fakeConn{}

	// the member joins before the host, yet the host must top the list
	rm.Join(Info{SessionID: "g1", Name: "Member"}, guest)
	rm.Join(Info{SessionID: "s1", Name: "Host", HostKey: hostKey}, hostConn)
	players := rm.snapshotForTest().Players
	if len(players) != 2 || players[0].Name != "Host" || !players[0].Host {
		t.Fatalf("host not first: %+v", players)
	}

	// host drops: members learn the reconnect deadline
	rm.Leave("s1", hostConn)
	if n := guest.count("host_reconnecting"); n == 0 {
		t.Fatal("host_reconnecting not broadcast")
	}
	// a member joining during the window inherits the countdown
	late := &fakeConn{}
	rm.Join(Info{SessionID: "g2", Name: "Late"}, late)
	welcome, ok := late.first("welcome")
	if !ok {
		t.Fatal("late joiner got no welcome")
	}
	if _, ok := welcome["hostReconnecting"]; !ok {
		t.Fatal("welcome missing hostReconnecting deadline")
	}
	// host returns: countdown lifted for everyone, seat back on top.
	// The rejoining host gets a fresh welcome — it must not re-arm the
	// reconnect countdown, or the wait dialog would flash on the host's
	// own screen right after host_back cleared it.
	hostBack := &fakeConn{}
	rm.Join(Info{SessionID: "s2", Name: "Host", HostKey: hostKey}, hostBack)
	if n := guest.count("host_back"); n == 0 {
		t.Fatal("host_back not broadcast")
	}
	rejoin, ok := hostBack.first("welcome")
	if !ok {
		t.Fatal("rejoining host got no welcome")
	}
	if _, ok := rejoin["hostReconnecting"]; ok {
		t.Fatal("rejoining host welcome must not carry hostReconnecting")
	}
	if !rm.snapshotForTest().Players[0].Host {
		t.Fatal("host not first after rejoin")
	}
}

func TestRenameBroadcastsNewName(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	c := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "OldName", HostKey: hostKey}, c)
	before := c.count("room_state")

	if err := rm.Rename("s1", "NewName"); err != nil {
		t.Fatal(err)
	}
	if c.count("room_state") != before+1 {
		t.Fatal("room_state not broadcast after rename")
	}
	for _, p := range rm.snapshotForTest().Players {
		if p.Name != "NewName" {
			t.Fatalf("player name = %s, want NewName", p.Name)
		}
	}
	if err := rm.Rename("ghost", "Nobody"); err == nil {
		t.Fatal("rename for unknown session should fail")
	}
}

func TestMemberRefreshKeepsSeatAndResumeReattaches(t *testing.T) {
	old := playerGrace
	playerGrace = 60 * time.Millisecond
	defer func() { playerGrace = old }()

	rm, hostKey := newTestRoom(t, game.Queen)
	host, member := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, host)
	rm.Join(Info{SessionID: "m1", Name: "Member"}, member)

	// the refresh: the seat is kept warm and flagged absent, not deleted
	rm.Leave("m1", member)
	if rm.PlayerCount() != 2 {
		t.Fatalf("seat not kept during grace: %d players", rm.PlayerCount())
	}
	absent := false
	for _, pv := range rm.snapshotForTest().Players {
		if pv.ID == "m1" {
			absent = pv.Absent
		}
	}
	if !absent {
		t.Fatal("refreshed member not flagged absent")
	}

	// the rejoin: the resume secret reattaches the same seat
	welcome, _ := member.first("welcome")
	key, _ := welcome["resume"].(string)
	if key == "" {
		t.Fatal("welcome missing resume secret")
	}
	back := &fakeConn{}
	rm.Join(Info{SessionID: "m2", Name: "Member", Resume: key}, back)
	rejoined, _ := back.first("welcome")
	if rejoined["you"] != "m1" {
		t.Fatalf("resume created a new seat: %v", rejoined["you"])
	}
	for _, pv := range rm.snapshotForTest().Players {
		if pv.ID == "m1" && pv.Absent {
			t.Fatal("resumed seat still flagged absent")
		}
	}

	// the dropped socket's late leave must not re-flag the resumed seat
	rm.Leave("m1", member)
	for _, pv := range rm.snapshotForTest().Players {
		if pv.ID == "m1" && pv.Absent {
			t.Fatal("stale leave re-flagged the resumed seat")
		}
	}

	// a real goodbye drops the seat once the grace elapses
	rm.Leave("m1", back)
	time.Sleep(150 * time.Millisecond)
	for _, pv := range rm.snapshotForTest().Players {
		if pv.ID == "m1" {
			t.Fatal("absent seat not dropped after grace")
		}
	}
	if host.count("room_state") == 0 {
		t.Fatal("host never told about the seat changes")
	}
}
