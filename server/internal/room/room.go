package room

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/pattaradol9/game24/server/internal/game"
)

// Conn is one live websocket, from the room's point of view.
type Conn interface {
	Deliver(raw []byte) // enqueue a pre-encoded message
	CloseConn()
}

// Info describes a player joining the room.
type Info struct {
	SessionID string // unique per connection
	Name      string
	DBID      string // store player id for signed-in users, "" for guests
	Level     int64
	Tier      string
	HostKey   string // matches the room secret to claim host
	Resume    string // matches a seat's secret to reattach after a refresh
}

type player struct {
	id              string
	name            string
	dbID            string
	level           int64
	tier            string
	score           int64
	wins            int
	hintsLeft       int
	extendsLeft     int
	regensLeft      int
	hintRoundNo     int // round each helper was last spent in: one use per
	extendRoundNo   int // round per seat, on top of the per-match quota
	regenRoundNo    int
	solvedRoundNo   int   // round the seat solved; 0 = not solved this round
	solveOrder      int   // which seat solved it: 1st, 2nd, … within the round
	timedOutRoundNo int   // round whose own clock ran out on the seat
	roundPoints     int64 // points the current round's solve paid the seat
	conn            Conn
	absent          bool      // socket dropped; seat held warm through playerGrace
	absentSeq       int       // bumped per absence so stale drop timers no-op
	resumeKey       string    // secret letting the same browser reattach this seat
	extendEndsAt    time.Time // private deadline while a time extension is live
}

type stateView struct {
	Code    string       `json:"code"`
	State   string       `json:"state"` // lobby | round | summary | finished
	Host    string       `json:"host"`
	Config  Config       `json:"config"`
	RoundNo int          `json:"roundNo,omitempty"`
	Players []playerView `json:"players"`
}

type playerView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Guest       bool   `json:"guest"`
	Level       int64  `json:"level"`
	Tier        string `json:"tier"`
	Score       int64  `json:"score"`
	Wins        int    `json:"wins"`
	HintsLeft   int    `json:"hintsLeft"`
	ExtendsLeft int    `json:"extendsLeft"`
	RegensLeft  int    `json:"regensLeft"`
	Solved      bool   `json:"solved"` // solved the current round
	SolveOrder  int    `json:"solveOrder,omitempty"`
	TimedOut    bool   `json:"timedOut"` // own clock ran out this round
	Gained      int64  `json:"gained,omitempty"`
	Rank        int    `json:"rank,omitempty"` // podium place, stamped on the final standings only
	Host        bool   `json:"host"`
	Absent      bool   `json:"absent"` // socket dropped, seat held warm
}

type Room struct {
	code    string
	cfg     Config
	award   AwardEXP
	hostKey string

	mu                   sync.Mutex
	players              map[string]*player
	joinOrder            []string
	hostID               string
	hostAbsentSince      time.Time // zero while the host seat is connected
	state                string
	roundNo              int
	numbers              map[string][]int // session id -> active hand (shared or regenerated)
	sharedNumbers        []int
	deadline             time.Time
	roundOver            bool
	solvedCount          int // seats that solved the current round, so far
	gen                  int
	rnd                  *rand.Rand
	removedCallback      func(code string)
	repopulatedCallback  func(code string)
	hostLostCallback     func(code string)
	hostReturnedCallback func(code string)
}

const (
	StateLobby    = "lobby"
	StateRound    = "round"
	StateSummary  = "summary"
	StateFinished = "finished"

	extendSeconds = 30 // seconds one Add-time use adds to a seat's private clock

	// maxPlayers caps a room's seats. Checked on fresh joins only — resume
	// reattaches and reclaiming seats never grow the roster.
	maxPlayers = 10
)

func newRoom(code, hostKey string, cfg Config, award AwardEXP) *Room {
	return &Room{
		code:    code,
		cfg:     cfg,
		award:   award,
		hostKey: hostKey,
		players: map[string]*player{},
		numbers: map[string][]int{},
		state:   StateLobby,
		rnd:     rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 24)),
	}
}

// Attach hooks so the hub can drop finished/empty rooms and cancel the
// pending removal when someone joins again.
func (r *Room) OnEmpty(fn func(code string)) { r.removedCallback = fn }

func (r *Room) OnRepopulated(fn func(code string)) {
	if fn != nil {
		r.repopulatedCallback = fn
	}
}

// OnHostLost fires when the host disconnects while members remain; the
// hub starts the reconnect grace, after which the room is closed.
func (r *Room) OnHostLost(fn func(code string)) {
	if fn != nil {
		r.hostLostCallback = fn
	}
}

// OnHostReturned fires when a join reclaims the host seat, so the hub can
// abort a pending close.
func (r *Room) OnHostReturned(fn func(code string)) {
	if fn != nil {
		r.hostReturnedCallback = fn
	}
}

func (r *Room) Code() string { return r.code }

func (r *Room) PublicInfo() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return map[string]any{
		"code": r.code, "mode": r.cfg.Mode, "rounds": cfgRounds(r.cfg),
		"state": r.state, "players": len(r.players),
	}
}

func cfgRounds(c Config) int { return c.Rounds }

// Join registers a connection. The host seat belongs to the hostKey
// holder (the room creator) and is never handed to anyone else. A join
// carrying a valid resume secret reattaches the caller's existing seat —
// same id, score and quotas — so a page refresh never churns the room
// for everyone else. Returns the seat id that now owns the connection:
// the fresh session id, or the reattached seat's original one. Fresh
// joins are refused once the room holds maxPlayers seats.
func (r *Room) Join(info Info, conn Conn) (string, error) {
	r.mu.Lock()

	// a resume secret identifies the returning browser's own seat
	var p *player
	if info.Resume != "" {
		for _, q := range r.players {
			if q.resumeKey == info.Resume {
				p = q
				break
			}
		}
	}

	// the host returning on a brand-new session retires their lingering
	// absent seat first, so the freed slot counts against capacity
	if p == nil && r.hostID != "" && info.HostKey != "" && info.HostKey == r.hostKey {
		if old, ok := r.players[r.hostID]; ok && old.absent {
			delete(r.players, old.id)
			delete(r.numbers, old.id)
			r.removeFromOrderLocked(old.id)
		}
	}
	// fresh seats only: a full room refuses everyone else
	if p == nil && len(r.players) >= maxPlayers {
		r.mu.Unlock()
		return "", fmt.Errorf("room is full (max %d players)", maxPlayers)
	}

	first := r.connectedLocked() == 0
	isHostSeat := false
	var staleConn Conn
	if p != nil {
		// reattach: fresh socket into the same seat, everything kept
		staleConn = p.conn
		p.conn = conn
		p.absent = false
		p.absentSeq++
		p.name, p.dbID, p.level, p.tier = info.Name, info.DBID, info.Level, info.Tier
		isHostSeat = p.id == r.hostID
	} else {
		p = &player{
			id: info.SessionID, name: info.Name, dbID: info.DBID,
			level: info.Level, tier: info.Tier, conn: conn,
			hintsLeft: r.cfg.HintQuota, extendsLeft: r.cfg.ExtendQuota,
			regensLeft: r.cfg.RegenQuota,
			resumeKey:  newSecret(24),
		}
		r.players[p.id] = p
		r.joinOrder = append(r.joinOrder, p.id)
		if r.hostID == "" {
			r.hostID = p.id
			isHostSeat = true
		} else if info.HostKey != "" && info.HostKey == r.hostKey {
			r.hostID = p.id
			isHostSeat = true
		}
	}

	welcome := map[string]any{"you": p.id, "resume": p.resumeKey, "state": r.snapshotLocked()}
	// members surface the pending host countdown; the host seat itself
	// must not — host_back already lifted it, and a stale deadline sent
	// after it would re-open the wait dialog on the host's own screen
	if !r.hostAbsentSince.IsZero() && !isHostSeat {
		welcome["hostReconnecting"] = map[string]any{
			"endsAt": r.hostAbsentSince.Add(hostGrace).UnixMilli(),
		}
	}
	// (re)joining mid-match: hand over the live round data so a refresh
	// lands back on a playable board — a privately regenerated hand is
	// preserved when the seat survived the refresh
	hand := r.numbers[p.id]
	if r.state == StateRound && !r.roundOver && (hand != nil || r.sharedNumbers != nil) {
		if hand == nil {
			hand = r.sharedNumbers
			r.numbers[p.id] = hand
		}
		round := map[string]any{
			"roundNo":    r.roundNo,
			"total":      r.cfg.Rounds,
			"numbers":    hand,
			"endsAt":     r.deadline.UnixMilli(),
			"timeLimit":  game.MustConfig(r.cfg.Mode).TimeLimitSec,
			"hintUsed":   p.hintRoundNo == r.roundNo,
			"extendUsed": p.extendRoundNo == r.roundNo,
			"regenUsed":  p.regenRoundNo == r.roundNo,
		}
		// a live private extension survives the refresh: the seat re-arms
		// its own widened clock instead of the shared one
		if p.extendEndsAt.After(r.deadline) {
			round["extendEndsAt"] = p.extendEndsAt.UnixMilli()
			round["extraSeconds"] = int(p.extendEndsAt.Sub(r.deadline).Seconds())
		}
		welcome["round"] = round
	}
	r.mu.Unlock()

	if staleConn != nil {
		staleConn.CloseConn()
	}
	if first && r.repopulatedCallback != nil {
		r.repopulatedCallback(r.code)
	}
	if isHostSeat {
		r.mu.Lock()
		r.hostAbsentSince = time.Time{}
		r.mu.Unlock()
		if r.hostReturnedCallback != nil {
			r.hostReturnedCallback(r.code)
		}
		if !first {
			r.broadcast("host_back", nil)
		}
	}
	r.sendState()
	r.sendTo(p.id, "welcome", welcome)
	return p.id, nil
}

// Leave drops a connection. The seat stays warm through the reconnect
// grace, flagged absent so the room shows "connecting…" on that player's
// name only; dropAbsent reaps it if the player never returns. The host
// seat is never reaped — the hub's host grace decides the room's fate.
func (r *Room) Leave(sessionID string, conn Conn) {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok || p.absent || p.conn != conn {
		// unknown seat, already absent, or a stale leave from a socket
		// that lost the race against a resume reattach
		r.mu.Unlock()
		return
	}
	p.absent = true
	p.absentSeq++
	p.conn = nil
	seq := p.absentSeq
	wasHost := r.hostID == sessionID
	connected := r.connectedLocked()
	var reconnectEndsAt int64
	if wasHost && connected > 0 {
		r.hostAbsentSince = time.Now()
		reconnectEndsAt = r.hostAbsentSince.Add(hostGrace).UnixMilli()
	}
	r.mu.Unlock()

	time.AfterFunc(playerGrace, func() { r.dropAbsent(sessionID, seq) })

	if connected == 0 {
		// nobody is watching anymore; keep the room briefly for refreshes
		if r.removedCallback != nil {
			r.removedCallback(r.code)
		}
		return
	}
	if wasHost && r.hostLostCallback != nil {
		r.hostLostCallback(r.code)
	}
	if reconnectEndsAt != 0 {
		r.broadcast("host_reconnecting", map[string]any{"endsAt": reconnectEndsAt})
	}
	r.sendState()
}

// dropAbsent reaps a seat whose reconnect grace elapsed. The sequence
// check makes timers from earlier absences — or a resume that already
// landed — harmless no-ops.
func (r *Room) dropAbsent(sessionID string, seq int) {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok || !p.absent || p.absentSeq != seq {
		r.mu.Unlock()
		return
	}
	if sessionID == r.hostID && r.connectedLocked() > 0 {
		// while members remain the host seat stays put — the hub's host
		// grace decides the room's fate instead
		r.mu.Unlock()
		return
	}
	delete(r.players, sessionID)
	delete(r.numbers, sessionID)
	r.removeFromOrderLocked(sessionID)
	r.mu.Unlock()
	r.sendState()
}

// connectedLocked counts seats with a live socket.
func (r *Room) connectedLocked() int {
	n := 0
	for _, q := range r.players {
		if !q.absent {
			n++
		}
	}
	return n
}

func (r *Room) removeFromOrderLocked(id string) {
	for i, q := range r.joinOrder {
		if q == id {
			r.joinOrder = append(r.joinOrder[:i], r.joinOrder[i+1:]...)
			return
		}
	}
}

// Shutdown notifies everyone still inside that the room is over (the host
// is gone for good) and tears the connections down. The hub calls it once
// the reconnect grace has elapsed: a match still standing ends into the
// final podium, a bare lobby closes with the plain notice.
func (r *Room) Shutdown() {
	r.mu.Lock()
	typ, data := r.endByHostLeftLocked()
	conns := r.liveConnsLocked()
	r.mu.Unlock()
	r.seal(conns, typ, data)
}

// HostLeft is the host's explicit goodbye (the leave message — a refresh
// or a dropped socket never sends it): the match ends for every seat at
// once, with the same final podium a natural finish would hand out. A
// member's goodbye keeps the warm-seat flow and reports false. The caller
// must drop the room from the hub so joins fail from here on.
func (r *Room) HostLeft(sessionID string) bool {
	r.mu.Lock()
	if sessionID != r.hostID {
		r.mu.Unlock()
		return false
	}
	typ, data := r.endByHostLeftLocked()
	conns := r.liveConnsLocked()
	r.mu.Unlock()
	r.seal(conns, typ, data)
	return true
}

// endByHostLeftLocked freezes the match where it stands once the host is
// gone for good: a match still standing becomes the final podium
// (match_end, reason host_left), a bare lobby closes outright, and an
// already-finished match needs no second announcement. Returns the goodbye
// message to seal the room with. The lock must be held.
func (r *Room) endByHostLeftLocked() (string, map[string]any) {
	if r.roundNo == 0 || r.state == StateFinished {
		return "room_closed", map[string]any{"reason": "host left"}
	}
	r.state = StateFinished
	r.roundOver = true
	return "match_end", map[string]any{
		"standings": r.rankedStandingsLocked(),
		"reason":    "host_left",
	}
}

// liveConnsLocked snapshots every connected socket. The lock must be held.
func (r *Room) liveConnsLocked() []Conn {
	conns := make([]Conn, 0, len(r.players))
	for _, p := range r.players {
		if p.conn != nil {
			conns = append(conns, p.conn)
		}
	}
	return conns
}

// seal delivers one last message to the sockets and cuts them once it has
// flushed — the shared tail of every host-left teardown.
func (r *Room) seal(conns []Conn, typ string, data map[string]any) {
	raw, err := json.Marshal(map[string]any{"type": typ, "data": data})
	if err != nil {
		return
	}
	for _, c := range conns {
		c.Deliver(raw)
	}
	// give the writer pumps a beat to flush the notice before cutting
	time.AfterFunc(100*time.Millisecond, func() {
		for _, c := range conns {
			c.CloseConn()
		}
	})
}

// Start begins the match; only the host may call it.
func (r *Room) Start(sessionID string) error {
	r.mu.Lock()
	if sessionID != r.hostID {
		r.mu.Unlock()
		return fmt.Errorf("only the host can start")
	}
	if r.state != StateLobby {
		r.mu.Unlock()
		return fmt.Errorf("match already started")
	}
	r.roundNo = 0
	r.dealRoundLocked()
	return nil
}

// dealRoundLocked deals the next round of the match. The room lock must be
// held on entry and is released on the way out, where the round_start
// broadcast goes out and the round's expiry timer arms itself.
func (r *Room) dealRoundLocked() {
	r.gen++
	g := r.gen
	r.roundNo++
	hand, err := game.Deal(r.cfg.Mode, r.rnd)
	if err != nil {
		r.mu.Unlock()
		r.broadcast("error", map[string]any{"message": "deal failed: " + err.Error()})
		return
	}
	r.sharedNumbers = hand
	r.numbers = map[string][]int{}
	for id := range r.players {
		r.numbers[id] = hand
	}
	// an extension buys extra seconds for one round only; the per-round
	// solve marks and the per-match quota counters behave like the clock:
	// the deadline resets, everything else carries over
	for _, p := range r.players {
		p.extendEndsAt = time.Time{}
		p.roundPoints = 0
		p.solveOrder = 0
		p.timedOutRoundNo = 0
	}
	r.solvedCount = 0
	r.deadline = time.Now().Add(time.Duration(game.MustConfig(r.cfg.Mode).TimeLimitSec) * time.Second)
	r.roundOver = false
	r.state = StateRound
	start := map[string]any{
		"roundNo":   r.roundNo,
		"total":     r.cfg.Rounds,
		"numbers":   hand,
		"endsAt":    r.deadline.UnixMilli(),
		"timeLimit": game.MustConfig(r.cfg.Mode).TimeLimitSec,
	}
	r.mu.Unlock()
	r.broadcast("round_start", start)

	time.AfterFunc(time.Until(r.deadline), func() { r.expire(g) })
}

// expire closes the round once every seat is done — solved, or their own
// clock ran out. Private clocks die at different times (each seat's
// extension is its own), so each fire settles the seats whose deadline just
// passed and reschedules itself to the earliest still-live clock until the
// round is decided.
func (r *Room) expire(g int) {
	r.mu.Lock()
	if r.gen != g || r.roundOver || r.state != StateRound {
		r.mu.Unlock()
		return
	}
	if !r.settleRoundLocked() {
		next := r.nextDeadlineLocked()
		r.mu.Unlock()
		time.AfterFunc(time.Until(next), func() { r.expire(g) })
		return
	}
	r.roundOver = true
	result := r.roundResultLocked()
	r.state = StateSummary
	r.mu.Unlock()
	r.broadcast("round_result", result)
	r.scheduleAutoNext(g)
}

// settleRoundLocked flags every unsolved seat whose own clock has run out
// (the shared deadline or its private extension — whichever the seat holds)
// and reports whether the round is decided: everyone solved or out of time.
func (r *Room) settleRoundLocked() bool {
	now := time.Now()
	for _, p := range r.players {
		if p.solvedRoundNo == r.roundNo || p.timedOutRoundNo == r.roundNo {
			continue
		}
		if p.deadlineLocked(r.deadline).Before(now) {
			p.timedOutRoundNo = r.roundNo
		}
	}
	return r.allDoneLocked()
}

// allDoneLocked reports whether every seat finished the round one way or
// another — solved, or out of time.
func (r *Room) allDoneLocked() bool {
	for _, p := range r.players {
		if p.solvedRoundNo != r.roundNo && p.timedOutRoundNo != r.roundNo {
			return false
		}
	}
	return true
}

// deadlineLocked is the seat's own clock: the shared deadline, pushed out
// while a time extension of this round is still live.
func (p *player) deadlineLocked(shared time.Time) time.Time {
	if p.extendEndsAt.After(shared) {
		return p.extendEndsAt
	}
	return shared
}

// roundResultLocked builds the round_result payload: everyone's cumulative
// standings with this round's gains, the dealt hand's solution, and the
// moment the summary auto-advances should the host stay silent.
func (r *Room) roundResultLocked() map[string]any {
	sol := ""
	if sols := game.Solve(r.numbersForSolution()); len(sols) > 0 {
		sol = sols[0].Expr
	}
	return map[string]any{
		"roundNo":    r.roundNo,
		"solution":   sol,
		"standings":  r.standingsLocked(),
		"autoNextAt": time.Now().Add(autoNextDelay).UnixMilli(),
	}
}

// nextDeadlineLocked returns the earliest still-live clock among the seats
// that are neither solved nor timed out yet — the next moment the round can
// decide itself.
func (r *Room) nextDeadlineLocked() time.Time {
	next := time.Time{}
	for _, p := range r.players {
		if p.solvedRoundNo == r.roundNo || p.timedOutRoundNo == r.roundNo {
			continue
		}
		effective := p.deadlineLocked(r.deadline)
		if next.IsZero() || effective.Before(next) {
			next = effective
		}
	}
	if next.IsZero() {
		next = r.deadline
	}
	return next
}

func (r *Room) numbersForSolution() []int {
	if r.sharedNumbers != nil {
		return r.sharedNumbers
	}
	for _, h := range r.numbers {
		return h
	}
	return nil
}

// Next is the host's go signal between rounds: only they may start the next
// round — or, once the last round has been summarized, close out the match.
// Everyone else waits in the summary; if the host stays silent past the
// auto-advance window, the match moves on by itself (see scheduleAutoNext).
func (r *Room) Next(sessionID string) error {
	r.mu.Lock()
	if sessionID != r.hostID {
		r.mu.Unlock()
		return fmt.Errorf("only the host can continue")
	}
	if r.state != StateSummary {
		r.mu.Unlock()
		return fmt.Errorf("no round to continue from")
	}
	r.advanceLocked()
	return nil
}

// advanceLocked moves the match out of the summary — the next round, or the
// podium once every round is summarized. The lock must be held and is
// released on the way out; the state re-check inside keeps a host press
// racing the auto-advance timer from moving the match twice.
func (r *Room) advanceLocked() {
	if r.state != StateSummary {
		r.mu.Unlock()
		return
	}
	if r.roundNo >= r.cfg.Rounds {
		r.state = StateFinished
		r.roundOver = true
		standings := r.rankedStandingsLocked()
		r.mu.Unlock()
		r.broadcast("match_end", map[string]any{"standings": standings})
		return
	}
	r.dealRoundLocked()
}

// scheduleAutoNext is the stall guard: when a round's summary begins, a
// window opens for the host to continue at their own pace — past it, the
// summary advances itself so a silent host can never freeze the room.
func (r *Room) scheduleAutoNext(g int) {
	time.AfterFunc(autoNextDelay, func() {
		r.mu.Lock()
		// a stale timer from an older summary no-ops: only the live one
		// (same generation, still sitting in the summary) may advance
		if r.gen != g || r.state != StateSummary {
			r.mu.Unlock()
			return
		}
		r.advanceLocked()
	})
}

// Submit verifies a trace and scores the seat's own solve: every player
// races the same hand until they solve it or the clock dies, and the sooner
// a seat solves, the more remaining time it banks. A solve marks the seat
// (the roster shows the "solved" status) and the round ends only when every
// seat has solved — or when expire closes it on time.
func (r *Room) Submit(sessionID string, steps []game.Step) error {
	r.mu.Lock()
	if r.state != StateRound || r.roundOver {
		r.mu.Unlock()
		return fmt.Errorf("round is over")
	}
	p, ok := r.players[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("not in room")
	}
	if p.solvedRoundNo == r.roundNo {
		r.mu.Unlock()
		return fmt.Errorf("already solved this round")
	}
	// each seat submits against its own clock: the shared deadline, pushed
	// out while a time extension of this round is still live
	effective := p.deadlineLocked(r.deadline)
	if time.Now().After(effective) {
		r.mu.Unlock()
		return fmt.Errorf("time is up")
	}
	hand, ok := r.numbers[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no active hand")
	}
	if err := game.Verify(hand, steps); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("wrong: %v", err)
	}

	remaining := int64(time.Until(effective).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	cfg, _ := game.Config(r.cfg.Mode)
	points := (10 + remaining) * int64(cfg.Multiplier)
	p.score += points
	p.wins++
	p.roundPoints = points
	p.solvedRoundNo = r.roundNo
	r.solvedCount++
	p.solveOrder = r.solvedCount

	if p.dbID != "" && r.award != nil {
		dbID, winner, mode := p.dbID, p.id, r.cfg.Mode
		go func() {
			unlocks := r.award(dbID, mode, points)
			if len(unlocks) == 0 {
				return
			}
			payload := make([]map[string]any, 0, len(unlocks))
			for _, u := range unlocks {
				payload = append(payload, map[string]any{
					"id": u.ID, "tier": u.Tier,
					"title":     map[string]any{"en": u.TitleEN, "th": u.TitleTH},
					"expReward": u.ExpReward, "coinReward": u.CoinReward,
				})
			}
			r.sendTo(winner, "achievements", map[string]any{"unlocked": payload})
		}()
	}

	// the round is decided when everyone is done — this solve, or a clock
	// that already ran out on its own. Private clocks die at different
	// times, so a seat that timed out earlier counts as finished here.
	allDone := r.settleRoundLocked()
	var result map[string]any
	if allDone {
		// the last solver just banked: close the round at once — the match
		// moves on when the host calls the next round, or the auto-advance
		// window elapses on a silent host
		r.roundOver = true
		r.state = StateSummary
		result = r.roundResultLocked()
	}
	g := r.gen
	r.mu.Unlock()

	if allDone {
		r.broadcast("round_result", result)
		r.scheduleAutoNext(g)
	} else {
		// round keeps running: everyone sees the fresh roster with the
		// seat's solved status
		r.sendState()
	}
	return nil
}

// Rename updates a player's display name and rebroadcasts the state so
// everyone in the room sees the new name without a rejoin.
func (r *Room) Rename(sessionID, name string) error {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("not in room")
	}
	p.name = name
	r.mu.Unlock()
	r.sendState()
	return nil
}

// Hint spends a hint and privately reveals the opening move.
func (r *Room) Hint(sessionID string) error {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("not in room")
	}
	if r.state != StateRound || r.roundOver {
		r.mu.Unlock()
		return fmt.Errorf("round is over")
	}
	// one solution per round: the reveal stays on the board, so a second
	// press in the same hand could only waste quota
	if p.hintRoundNo == r.roundNo || p.solvedRoundNo == r.roundNo {
		r.mu.Unlock()
		return fmt.Errorf("solution already used this round")
	}
	if p.hintsLeft <= 0 {
		r.mu.Unlock()
		return fmt.Errorf("no hints left")
	}
	hand, ok := r.numbers[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no active hand")
	}
	h, ok := game.Hint(hand)
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no hint available")
	}
	p.hintsLeft--
	p.hintRoundNo = r.roundNo
	r.mu.Unlock()
	r.sendTo(sessionID, "hint", map[string]any{
		"leftCard": h.Step.LeftCard, "rightCard": h.Step.RightCard,
		"op": h.Step.Op, "result": h.Step.Result.String(),
		"expr":          h.Expr,
		"alternatives":  h.Alternatives,
		"solutionCount": h.Count,
	})
	return nil
}

// Regen deals the player a private replacement hand.
func (r *Room) Regen(sessionID string) error {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("not in room")
	}
	if r.state != StateRound || r.roundOver {
		r.mu.Unlock()
		return fmt.Errorf("round is over")
	}
	// one new hand per round — and none at all once the seat has solved
	if p.regenRoundNo == r.roundNo || p.solvedRoundNo == r.roundNo {
		r.mu.Unlock()
		return fmt.Errorf("new hand already used this round")
	}
	if p.regensLeft <= 0 {
		r.mu.Unlock()
		return fmt.Errorf("no regenerates left")
	}
	hand, err := game.Deal(r.cfg.Mode, r.rnd)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	p.regensLeft--
	p.regenRoundNo = r.roundNo
	r.numbers[sessionID] = hand
	r.mu.Unlock()
	r.sendTo(sessionID, "regen", map[string]any{"numbers": hand})
	r.sendState()
	return nil
}

// Extend is the multiplayer Add-time helper: the room's own quota — set at
// room creation and identical for every seat, inventory untouched — pushes
// the seat's private round deadline out by extendSeconds. One add-time per
// seat per round. The shared clock
// keeps running for everyone else; the round stays open until the last live
// private deadline passes (see expire), and the seat's own submit is graded
// against its widened window. Guests spend the same room allowance as
// signed-in players: helpers are room-provided, never personal.
func (r *Room) Extend(sessionID string) error {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("not in room")
	}
	if r.state != StateRound || r.roundOver {
		r.mu.Unlock()
		return fmt.Errorf("round is over")
	}
	// one add-time per round — and none at all once the seat has solved
	if p.extendRoundNo == r.roundNo || p.solvedRoundNo == r.roundNo {
		r.mu.Unlock()
		return fmt.Errorf("time extension already used this round")
	}
	if p.extendsLeft <= 0 {
		r.mu.Unlock()
		return fmt.Errorf("no time extensions left")
	}
	p.extendsLeft--
	p.extendRoundNo = r.roundNo
	p.extendEndsAt = r.deadline.Add(extendSeconds * time.Second)
	endsAt, extra := p.extendEndsAt, extendSeconds
	r.mu.Unlock()
	r.sendState()
	r.sendTo(sessionID, "extended", map[string]any{
		"endsAt": endsAt.UnixMilli(), "extraSeconds": extra,
	})
	return nil
}

func (r *Room) standingsLocked() []playerView {
	out := make([]playerView, 0, len(r.players))
	for _, p := range r.players {
		out = append(out, r.viewLocked(p))
	}
	// sort by score desc, wins desc, name
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			if a.Score < b.Score || (a.Score == b.Score && a.Wins < b.Wins) ||
				(a.Score == b.Score && a.Wins == b.Wins && a.Name > b.Name) {
				out[j-1], out[j] = out[j], out[j-1]
			}
		}
	}
	return out
}

// rankedStandingsLocked stamps the podium places onto the score-sorted
// roster — the final standings the podium renders. The lock must be held.
func (r *Room) rankedStandingsLocked() []playerView {
	out := r.standingsLocked()
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func (r *Room) viewLocked(p *player) playerView {
	// roundNo 0 = the lobby: nothing has been solved yet, ever
	solved := r.roundNo > 0 && p.solvedRoundNo == r.roundNo
	timedOut := r.roundNo > 0 && p.timedOutRoundNo == r.roundNo
	return playerView{
		ID: p.id, Name: p.name, Guest: p.dbID == "", Level: p.level, Tier: p.tier,
		Score: p.score, Wins: p.wins, HintsLeft: p.hintsLeft,
		ExtendsLeft: p.extendsLeft, RegensLeft: p.regensLeft,
		Solved: solved, SolveOrder: p.solveOrder, TimedOut: timedOut,
		Gained: p.roundPoints,
		Host:   p.id == r.hostID, Absent: p.absent,
	}
}

func (r *Room) snapshotLocked() stateView {
	players := make([]playerView, 0, len(r.players))
	// the host is always listed first, everyone else by join order
	if hp, ok := r.players[r.hostID]; ok {
		players = append(players, r.viewLocked(hp))
	}
	for _, id := range r.joinOrder {
		if id == r.hostID {
			continue
		}
		if p, ok := r.players[id]; ok {
			players = append(players, r.viewLocked(p))
		}
	}
	return stateView{
		Code: r.code, State: r.state, Host: r.hostID, Config: r.cfg,
		RoundNo: r.roundNo, Players: players,
	}
}

func (r *Room) sendState() {
	r.mu.Lock()
	snap := r.snapshotLocked()
	r.mu.Unlock()
	r.broadcast("room_state", snap)
}

func (r *Room) sendTo(sessionID, typ string, data any) {
	raw, err := json.Marshal(map[string]any{"type": typ, "data": data})
	if err != nil {
		return
	}
	r.mu.Lock()
	p, ok := r.players[sessionID]
	r.mu.Unlock()
	if ok && p.conn != nil {
		p.conn.Deliver(raw)
	}
}

func (r *Room) broadcast(typ string, data any) {
	raw, err := json.Marshal(map[string]any{"type": typ, "data": data})
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		if p.conn != nil {
			p.conn.Deliver(raw)
		}
	}
}

// LevelFromProgress refreshes a signed-in player's level/tier display
// (called by the ws layer when the player re-authenticates).
func (r *Room) PlayerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.players)
}
