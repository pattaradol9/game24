package room

import (
	"encoding/json"
	"fmt"
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
	rm, hostKey, err := NewHub(nil).Create(Config{Mode: mode, Rounds: 12, HintQuota: 3, ExtendQuota: 2, RegenQuota: 2})
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

func TestEverySolverScoresAndRoundWaits(t *testing.T) {
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
	if len(sols) == 0 {
		t.Fatalf("no solution for %v", numbers)
	}

	// the first solve only marks its seat: the round keeps running
	if err := rm.Submit("p2", sols[0].Trace); err != nil {
		t.Fatalf("p2 submit: %v", err)
	}
	rm.mu.Lock()
	stillOpen := rm.state == StateRound && !rm.roundOver
	rm.mu.Unlock()
	if !stillOpen {
		t.Fatal("round closed before every seat had solved")
	}
	// the roster flags the solved seat, first to answer in the round
	for _, pv := range rm.snapshotForTest().Players {
		want := pv.ID == "p2"
		if pv.Solved != want {
			t.Fatalf("seat %s solved = %v, want %v", pv.ID, pv.Solved, want)
		}
		if pv.Solved && pv.SolveOrder != 1 {
			t.Fatalf("seat %s solveOrder = %d, want 1", pv.ID, pv.SolveOrder)
		}
	}
	// a solved seat cannot submit twice in the same round
	if err := rm.Submit("p2", sols[0].Trace); err == nil {
		t.Fatal("double submit accepted")
	}

	// the last solve closes the round immediately
	if err := rm.Submit("h", sols[0].Trace); err != nil {
		t.Fatalf("host submit: %v", err)
	}
	rm.mu.Lock()
	over := rm.roundOver && rm.state == StateSummary
	rm.mu.Unlock()
	if !over {
		t.Fatal("round did not close after the last seat solved")
	}
	// the second solver is order #2 in the roster
	for _, pv := range rm.snapshotForTest().Players {
		wantOrder := map[string]int{"h": 2, "p2": 1}
		if pv.SolveOrder != wantOrder[pv.ID] {
			t.Fatalf("seat %s solveOrder = %d, want %d", pv.ID, pv.SolveOrder, wantOrder[pv.ID])
		}
	}
	if host.count("round_result") != 1 || p2.count("round_result") != 1 {
		t.Fatalf("round_result counts: host=%d p2=%d", host.count("round_result"), p2.count("round_result"))
	}

	// both seats scored their own solves; in round one every cumulative
	// score equals its gain, and the summary carries the solution
	res, _ := p2.first("round_result")
	st, ok := res["standings"].([]any)
	if !ok || len(st) != 2 {
		t.Fatalf("standings = %v", res["standings"])
	}
	total := 0.0
	for _, raw := range st {
		row, _ := raw.(map[string]any)
		gained, _ := row["gained"].(float64)
		score, _ := row["score"].(float64)
		total += gained
		if gained <= 0 || score != gained {
			t.Fatalf("first-round row = %v, score must equal its positive gain", row)
		}
	}
	if total <= 0 {
		t.Fatal("no points awarded")
	}
	if sol, _ := res["solution"].(string); sol == "" {
		t.Fatal("summary missing the solution")
	}
}

func TestQuotasAreIdenticalForEveryone(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	host, p2 := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, host)
	rm.Join(Info{SessionID: "p2", Name: "Two"}, p2)

	snap := rm.snapshotForTest()
	for _, pv := range snap.Players {
		if pv.HintsLeft != 3 || pv.ExtendsLeft != 2 || pv.RegensLeft != 2 {
			t.Fatalf("quota not equal: %+v", pv)
		}
		// the lobby must not flag anyone as solved (roundNo is still 0)
		if pv.Solved {
			t.Fatalf("lobby seat flagged solved: %+v", pv)
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
	advance := func() {
		rm.mu.Lock()
		rm.roundNo++
		rm.mu.Unlock()
	}

	// one use per helper per round: a second press in the same round refuses
	// even with quota left (the reveal stays on the board anyway)
	if err := rm.Hint("s1"); err != nil {
		t.Fatal(err)
	}
	if err := rm.Hint("s1"); err == nil {
		t.Fatal("second solution in one round accepted")
	}
	if err := rm.Regen("s1"); err != nil {
		t.Fatal(err)
	}
	if err := rm.Regen("s1"); err == nil {
		t.Fatal("second new hand in one round accepted")
	}

	// the next round frees both helpers; the per-match quota keeps counting
	// down across rounds (3 solutions, 2 new hands in the test config)
	advance()
	if err := rm.Hint("s1"); err != nil {
		t.Fatal(err)
	}
	if err := rm.Regen("s1"); err != nil {
		t.Fatal(err)
	}
	advance()
	if err := rm.Hint("s1"); err != nil {
		t.Fatal(err)
	}
	advance()
	if err := rm.Hint("s1"); err == nil {
		t.Fatal("solution quota exceeded")
	}
	if err := rm.Regen("s1"); err == nil {
		t.Fatal("new-hand quota exceeded")
	}
	if conn.count("hint") != 3 || conn.count("regen") != 2 {
		t.Fatalf("personal messages: hint=%d regen=%d", conn.count("hint"), conn.count("regen"))
	}
}

func TestMatchFinishesAfterAllRounds(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
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
		switch {
		case state == StateSummary:
			// the host gates every transition: the next round — and the
			// final podium — only ever start on their call
			if err := rm.Next("s1"); err != nil {
				t.Fatalf("host next after round %d: %v", roundNo, err)
			}
		case state == StateRound && !over:
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

func TestHostGatesTheNextRound(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	hc, pc := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, hc)
	rm.Join(Info{SessionID: "p", Name: "Two"}, pc)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	sol := game.Solve(rm.sharedNumbers)[0]
	rm.mu.Unlock()
	if err := rm.Submit("h", sol.Trace); err != nil {
		t.Fatal(err)
	}
	if err := rm.Submit("p", sol.Trace); err != nil {
		t.Fatal(err)
	}

	// in the summary a member cannot drag the room forward
	if err := rm.Next("p"); err == nil {
		t.Fatal("member started the next round")
	}
	rm.mu.Lock()
	still := rm.state == StateSummary
	rm.mu.Unlock()
	if !still {
		t.Fatal("summary left before the host called it")
	}
	// the host's call starts round 2
	if err := rm.Next("h"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	round2 := rm.state == StateRound && rm.roundNo == 2
	rm.mu.Unlock()
	if !round2 {
		t.Fatal("host next did not start round 2")
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

func TestRoomExtendSpendsSeatQuota(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	conn := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, conn)
	rm.Start("s1")

	// helpers are room-provided: a guest spends the same room quota as a
	// signed-in player, no inventory involved
	guest := &fakeConn{}
	rm.Join(Info{SessionID: "g1", Name: "Guest"}, guest)
	if err := rm.Extend("g1"); err != nil {
		t.Fatalf("guest extend: %v", err)
	}
	if ext, _ := guest.first("extended"); ext == nil {
		t.Fatal("no extended message for the guest seat")
	}

	if err := rm.Extend("s1"); err != nil {
		t.Fatal(err)
	}
	ext, ok := conn.first("extended")
	if !ok {
		t.Fatal("no extended message for the seat")
	}
	endsAt, _ := ext["endsAt"].(float64)
	rm.mu.Lock()
	shared := rm.deadline.UnixMilli()
	rm.mu.Unlock()
	if int64(endsAt) <= shared {
		t.Fatalf("extended deadline %d not past the shared clock %d", int64(endsAt), shared)
	}

	// one add-time per round: a same-round second press refuses even with
	// quota left
	if err := rm.Extend("s1"); err == nil {
		t.Fatal("second extend in one round accepted")
	}

	// the next round frees the helper; the seat's second (and last) quota
	// unit is spent there, so the round after refuses outright
	rm.mu.Lock()
	rm.roundNo++
	rm.mu.Unlock()
	if err := rm.Extend("s1"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	rm.roundNo++
	rm.mu.Unlock()
	if err := rm.Extend("s1"); err == nil {
		t.Fatal("extend past the seat quota accepted")
	}

	// the extended seat solves after the shared clock has run out: its own
	// window is still open and the round pays off the widened limit
	rm.mu.Lock()
	numbers := rm.numbers["s1"]
	rm.deadline = time.Now().Add(-time.Second)
	rm.mu.Unlock()
	sols := game.Solve(numbers)
	if len(sols) == 0 {
		t.Fatal("no solution for the dealt hand")
	}
	if err := rm.Submit("s1", sols[0].Trace); err != nil {
		t.Fatalf("extended seat submit after shared deadline: %v", err)
	}
	// the round keeps waiting — the guest seat has not answered yet
	rm.mu.Lock()
	open := rm.state == StateRound && !rm.roundOver
	gen := rm.gen
	rm.mu.Unlock()
	if !open {
		t.Fatal("round closed while the guest was still playing")
	}
	// the clock runs out: every private clock dies with it, expire closes
	// the round
	rm.mu.Lock()
	for _, p := range rm.players {
		p.extendEndsAt = time.Time{}
	}
	rm.mu.Unlock()
	rm.expire(gen)
	if conn.count("round_result") != 1 {
		t.Fatal("no round_result after the clock ran out")
	}
}

func TestRoundWaitsForTheSlowestSolver(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	rm.mu.Lock()
	rm.mu.Unlock()
	a, b, c := &fakeConn{}, &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, a)
	rm.Join(Info{SessionID: "p2", Name: "Two"}, b)
	rm.Join(Info{SessionID: "p3", Name: "Three"}, c)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	sol := game.Solve(rm.sharedNumbers)[0]
	rm.mu.Unlock()

	// p2 solves early: a fat remaining-time bonus
	if err := rm.Submit("p2", sol.Trace); err != nil {
		t.Fatal(err)
	}
	// h solves late: most of the clock is gone, so its solve pays little.
	// The round must NOT close for either of them — p3 is still playing.
	rm.mu.Lock()
	rm.deadline = time.Now().Add(2 * time.Second)
	rm.mu.Unlock()
	if err := rm.Submit("h", sol.Trace); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	open := rm.state == StateRound && !rm.roundOver
	rm.mu.Unlock()
	if !open {
		t.Fatal("round closed while the slowest seat was still playing")
	}

	// p3 never solves: the shared clock dies and expire closes the round
	rm.mu.Lock()
	rm.deadline = time.Now().Add(-time.Second)
	gen := rm.gen
	for _, p := range rm.players {
		p.extendEndsAt = time.Time{}
	}
	rm.mu.Unlock()
	rm.expire(gen)
	if c.count("round_result") != 1 {
		t.Fatal("no round_result after the clock ran out")
	}

	// the summary judges the round: the fast solver out-earns the slow one,
	// the seat that never finished gains nothing
	res, _ := b.first("round_result")
	st, ok := res["standings"].([]any)
	if !ok || len(st) != 3 {
		t.Fatalf("standings = %v", res["standings"])
	}
	gains := map[string]float64{}
	for _, raw := range st {
		row, _ := raw.(map[string]any)
		id, _ := row["id"].(string)
		gains[id], _ = row["gained"].(float64)
	}
	if gains["p2"] <= gains["h"] {
		t.Fatalf("fast solver must outscore the slow one: p2=%v h=%v", gains["p2"], gains["h"])
	}
	if gains["p3"] != 0 {
		t.Fatalf("unsolved seat gained %v, want 0", gains["p3"])
	}
}

// Private clocks die at different times: a seat that ran out counts as
// finished, so the round closes the moment the last seat SOLVES — a solved
// seat's extension must never hold it open.
func TestRoundEndsWhenEveryonesClockIsDone(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	hc, pc := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, hc)
	rm.Join(Info{SessionID: "p", Name: "Two"}, pc)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	sol := game.Solve(rm.sharedNumbers)[0]
	rm.mu.Unlock()

	// p's clock dies with the shared deadline; h extends (+30s on its own
	// clock) and keeps playing
	rm.mu.Lock()
	rm.deadline = time.Now().Add(-time.Second)
	rm.mu.Unlock()
	if err := rm.Extend("h"); err != nil {
		t.Fatal(err)
	}

	// h solves while its extension is still live: p is already out of
	// time, so the round must close RIGHT NOW — not when the extension dies
	rm.mu.Lock()
	var live time.Time
	for _, p := range rm.players {
		if p.id == "h" {
			live = p.extendEndsAt
		}
	}
	rm.mu.Unlock()
	if time.Until(live) < 20*time.Second {
		t.Fatal("extension not active for the test")
	}
	if err := rm.Submit("h", sol.Trace); err != nil {
		t.Fatal(err)
	}
	snap := rm.snapshotForTest()
	rm.mu.Lock()
	over := rm.roundOver && rm.state == StateSummary
	rm.mu.Unlock()
	pTimedOut := false
	for _, pv := range snap.Players {
		if pv.ID == "p" {
			pTimedOut = pv.TimedOut
		}
	}
	if !over {
		t.Fatal("round did not close right after the last live seat solved")
	}
	if !pTimedOut {
		t.Fatal("expired seat not flagged timed out")
	}
	if hc.count("round_result") != 1 || pc.count("round_result") != 1 {
		t.Fatal("round_result not broadcast at the close")
	}
}

// The stall guard: a silent host never freezes the room — the summary
// advances itself once the window elapses, and a host press racing the
// timer still moves the match exactly once.
func TestSummaryAutoAdvancesWithoutHost(t *testing.T) {
	old := autoNextDelay
	autoNextDelay = 40 * time.Millisecond
	defer func() { autoNextDelay = old }()

	rm, hostKey := newTestRoom(t, game.Queen)
	hc, pc := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, hc)
	rm.Join(Info{SessionID: "p", Name: "Two"}, pc)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	sol := game.Solve(rm.sharedNumbers)[0]
	rm.mu.Unlock()
	if err := rm.Submit("h", sol.Trace); err != nil {
		t.Fatal(err)
	}
	if err := rm.Submit("p", sol.Trace); err != nil {
		t.Fatal(err)
	}

	// nobody presses: the summary advances itself into round 2
	waitFor := func(check func() bool, what string) {
		t.Helper()
		wait := time.Now().Add(3 * time.Second)
		for {
			if check() {
				return
			}
			if time.Now().After(wait) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitFor(func() bool {
		rm.mu.Lock()
		defer rm.mu.Unlock()
		return rm.state == StateRound && rm.roundNo == 2
	}, "the auto-advanced round 2")

	// round 2 dealt a fresh hand: solve it, land in the summary, then the
	// HOST presses before the window elapses — the pending timer must not
	// double-skip
	rm.mu.Lock()
	sol2 := game.Solve(rm.sharedNumbers)[0]
	rm.mu.Unlock()
	if err := rm.Submit("h", sol2.Trace); err != nil {
		t.Fatal(err)
	}
	if err := rm.Submit("p", sol2.Trace); err != nil {
		t.Fatal(err)
	}
	if err := rm.Next("h"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	rm.mu.Lock()
	round3, stillRound := rm.roundNo, rm.state == StateRound
	rm.mu.Unlock()
	if !stillRound || round3 != 3 {
		t.Fatalf("state round = %v round %d, want round 3 (auto timer must not re-advance)", stillRound, round3)
	}
}

func TestRoomFullAtTenPlayers(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	if _, err := rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, &fakeConn{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < maxPlayers; i++ {
		if _, err := rm.Join(Info{SessionID: fmt.Sprintf("s%d", i), Name: "P"}, &fakeConn{}); err != nil {
			t.Fatalf("join %d: %v", i, err)
		}
	}
	if _, err := rm.Join(Info{SessionID: "extra", Name: "Late"}, &fakeConn{}); err == nil {
		t.Fatal("join past the room capacity accepted")
	}
	if rm.PlayerCount() != maxPlayers {
		t.Fatalf("players = %d, want %d", rm.PlayerCount(), maxPlayers)
	}
}

func TestRoomSubmitRefusedPastOwnDeadline(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	conn := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey}, conn)
	rm.Start("s1")

	rm.mu.Lock()
	numbers := rm.numbers["s1"]
	rm.deadline = time.Now().Add(-time.Second)
	rm.mu.Unlock()
	sols := game.Solve(numbers)
	if err := rm.Submit("s1", sols[0].Trace); err == nil {
		t.Fatal("submit past the deadline accepted")
	}
}

func TestRoomExpireHoldsRoundForLiveExtension(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	rm.mu.Lock()
	rm.mu.Unlock()
	conn := &fakeConn{}
	rm.Join(Info{SessionID: "s1", Name: "Solo", HostKey: hostKey, DBID: "db1"}, conn)
	rm.Start("s1")
	if err := rm.Extend("s1"); err != nil {
		t.Fatal(err)
	}

	// force the shared clock to run out while the private extension stays
	// ahead: the expiring tick must hold the round open, then the
	// rescheduled fire closes it once the extension dies
	rm.mu.Lock()
	gen := rm.gen
	rm.deadline = time.Now().Add(30 * time.Millisecond)
	for _, p := range rm.players {
		p.extendEndsAt = time.Now().Add(280 * time.Millisecond)
	}
	rm.mu.Unlock()

	rm.expire(gen)
	rm.mu.Lock()
	stillOpen := rm.state == StateRound && !rm.roundOver
	rm.mu.Unlock()
	if !stillOpen {
		t.Fatal("round closed while a private extension was live")
	}

	wait := time.Now().Add(3 * time.Second)
	for {
		rm.mu.Lock()
		state := rm.state
		rm.mu.Unlock()
		if state == StateSummary {
			break
		}
		if time.Now().After(wait) {
			t.Fatal("round never closed after the extension ran out")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn.count("round_result") == 0 {
		t.Fatal("no round_result after the extension died")
	}
}

// The host's explicit goodbye ends the match for every seat at once: the
// standings freeze into the final podium (match_end, reason host_left) —
// a member's goodbye never does.
func TestHostLeaveEndsTheMatchWithTheFinalPodium(t *testing.T) {
	rm, hostKey := newTestRoom(t, game.Queen)
	hc, m1, m2 := &fakeConn{}, &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, hc)
	rm.Join(Info{SessionID: "m1", Name: "One"}, m1)
	rm.Join(Info{SessionID: "m2", Name: "Two"}, m2)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}
	rm.mu.Lock()
	sol := game.Solve(rm.sharedNumbers)[0]
	rm.mu.Unlock()
	if err := rm.Submit("m1", sol.Trace); err != nil {
		t.Fatal(err)
	}

	// a member's goodbye keeps the warm-seat flow — nothing ends
	if rm.HostLeft("m1") {
		t.Fatal("member leave was taken for the host's")
	}
	rm.mu.Lock()
	still := rm.state == StateRound
	rm.mu.Unlock()
	if !still {
		t.Fatal("a member's leave ended the match")
	}

	// the host leaves mid-round: the match is over for everyone, and the
	// podium carries the ranked frozen standings
	if !rm.HostLeft("h") {
		t.Fatal("host leave not recognized")
	}
	rm.mu.Lock()
	finished := rm.state == StateFinished && rm.roundOver
	rm.mu.Unlock()
	if !finished {
		t.Fatal("host leave did not finish the match")
	}
	for _, c := range []*fakeConn{hc, m1, m2} {
		if n := c.count("match_end"); n != 1 {
			t.Fatalf("match_end = %d, want 1", n)
		}
		if n := c.count("room_closed"); n != 0 {
			t.Fatalf("room_closed = %d, want 0 (the podium replaces it)", n)
		}
	}
	res, _ := m2.first("match_end")
	if res["reason"] != "host_left" {
		t.Fatalf("reason = %v, want host_left", res["reason"])
	}
	st, ok := res["standings"].([]any)
	if !ok || len(st) != 3 {
		t.Fatalf("standings = %v", res["standings"])
	}
	prev := -1.0
	for i, raw := range st {
		row, _ := raw.(map[string]any)
		score, _ := row["score"].(float64)
		rank, _ := row["rank"].(float64)
		if int(rank) != i+1 {
			t.Fatalf("row %d rank = %v, want %d", i, row["rank"], i+1)
		}
		if prev >= 0 && score > prev {
			t.Fatalf("standings not score-descending: %v", st)
		}
		prev = score
	}
}

// A host whose socket simply never comes back ends the match the same way
// once the reconnect grace expires: members get the final podium, not a
// raw socket cut. A bare lobby still closes with the plain notice.
func TestHostGraceExpiryEndsStandingMatchWithPodium(t *testing.T) {
	old := hostGrace
	hostGrace = 40 * time.Millisecond
	defer func() { hostGrace = old }()

	hub := NewHub(nil)
	rm, hostKey, err := hub.Create(Config{Mode: game.Queen, Rounds: 12, HintQuota: 3, RegenQuota: 2})
	if err != nil {
		t.Fatal(err)
	}
	host, guest := &fakeConn{}, &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey}, host)
	rm.Join(Info{SessionID: "g", Name: "Guest"}, guest)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}

	// the host's socket drops and never returns
	rm.Leave("h", host)
	wait := time.Now().Add(3 * time.Second)
	for guest.count("match_end") == 0 {
		if time.Now().After(wait) {
			t.Fatal("grace expiry never delivered the final podium")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := guest.count("room_closed"); n != 0 {
		t.Fatalf("room_closed = %d, want 0 (match_end replaces it)", n)
	}
	res, _ := guest.first("match_end")
	if res["reason"] != "host_left" {
		t.Fatalf("reason = %v, want host_left", res["reason"])
	}
	if _, err := hub.Get(rm.Code()); err == nil {
		t.Fatal("room still open after the grace expired")
	}
}

func TestAwardCallbackCarriesGuestFlag(t *testing.T) {
	// every round winner reports its seat to the award callback: signed-in
	// seats with guest=false, anonymous seats with guest=true (score-only
	// banking) — both carrying their dbID
	type awardCall struct {
		dbID  string
		guest bool
	}
	calls := make(chan awardCall, 4)
	hub := NewHub(func(dbPlayerID string, guest bool, _ game.Mode, _ int64) []Unlock {
		calls <- awardCall{dbID: dbPlayerID, guest: guest}
		return nil
	})
	rm, hostKey, err := hub.Create(Config{Mode: game.Queen, Rounds: 12, HintQuota: 3, RegenQuota: 2})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeConn{}
	rm.Join(Info{SessionID: "h", Name: "Host", HostKey: hostKey, DBID: "db-host"}, host)
	guest := &fakeConn{}
	rm.Join(Info{SessionID: "g", Name: "Guest", DBID: "db-guest", Guest: true}, guest)
	if err := rm.Start("h"); err != nil {
		t.Fatal(err)
	}

	rm.mu.Lock()
	numbers := rm.sharedNumbers
	rm.mu.Unlock()
	sols := game.Solve(numbers)
	if len(sols) == 0 {
		t.Fatalf("no solution for %v", numbers)
	}
	if err := rm.Submit("g", sols[0].Trace); err != nil {
		t.Fatalf("guest submit: %v", err)
	}
	if err := rm.Submit("h", sols[0].Trace); err != nil {
		t.Fatalf("host submit: %v", err)
	}

	got := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case c := <-calls:
			got[c.dbID] = c.guest
		case <-time.After(2 * time.Second):
			t.Fatalf("award callback fired %d times, want 2", i)
		}
	}
	if got["db-host"] != false {
		t.Fatalf("signed-in seat guest flag = %v, want false", got["db-host"])
	}
	if got["db-guest"] != true {
		t.Fatalf("guest seat guest flag = %v, want true", got["db-guest"])
	}
}
