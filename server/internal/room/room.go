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
	id         string
	name       string
	dbID       string
	level      int64
	tier       string
	score      int64
	wins       int
	hintsLeft  int
	regensLeft int
	conn       Conn
	absent     bool   // socket dropped; seat held warm through playerGrace
	absentSeq  int    // bumped per absence so stale drop timers no-op
	resumeKey  string // secret letting the same browser reattach this seat
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
	ID         string `json:"id"`
	Name       string `json:"name"`
	Guest      bool   `json:"guest"`
	Level      int64  `json:"level"`
	Tier       string `json:"tier"`
	Score      int64  `json:"score"`
	Wins       int    `json:"wins"`
	HintsLeft  int    `json:"hintsLeft"`
	RegensLeft int    `json:"regensLeft"`
	Host       bool   `json:"host"`
	Absent     bool   `json:"absent"` // socket dropped, seat held warm
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
	summaryDur           time.Duration
	roundOver            bool
	solvedBy             string
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

	summarySeconds = 6 // default pause between rounds
)

func newRoom(code, hostKey string, cfg Config, award AwardEXP) *Room {
	return &Room{
		code:       code,
		cfg:        cfg,
		award:      award,
		hostKey:    hostKey,
		players:    map[string]*player{},
		numbers:    map[string][]int{},
		state:      StateLobby,
		rnd:        rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 24)),
		summaryDur: summarySeconds * time.Second,
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
// the fresh session id, or the reattached seat's original one.
func (r *Room) Join(info Info, conn Conn) string {
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
			hintsLeft: r.cfg.HintQuota, regensLeft: r.cfg.RegenQuota,
			resumeKey: newSecret(24),
		}
		r.players[p.id] = p
		r.joinOrder = append(r.joinOrder, p.id)
		claimsHost := r.hostID == "" || (info.HostKey != "" && info.HostKey == r.hostKey)
		if claimsHost {
			// the host is back on a brand-new session: retire their
			// lingering absent seat so no ghost chip stays behind
			if old, ok := r.players[r.hostID]; ok && old.id != p.id && old.absent {
				delete(r.players, old.id)
				delete(r.numbers, old.id)
				r.removeFromOrderLocked(old.id)
			}
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
		welcome["round"] = map[string]any{
			"roundNo":   r.roundNo,
			"total":     r.cfg.Rounds,
			"numbers":   hand,
			"endsAt":    r.deadline.UnixMilli(),
			"timeLimit": game.MustConfig(r.cfg.Mode).TimeLimitSec,
		}
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
	return p.id
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
// the reconnect grace has elapsed.
func (r *Room) Shutdown() {
	r.mu.Lock()
	conns := make([]Conn, 0, len(r.players))
	for _, p := range r.players {
		if p.conn != nil {
			conns = append(conns, p.conn)
		}
	}
	raw, _ := json.Marshal(map[string]any{
		"type": "room_closed",
		"data": map[string]any{"reason": "host left"},
	})
	for _, c := range conns {
		c.Deliver(raw)
	}
	r.mu.Unlock()
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
	r.state = StateRound
	r.roundNo = 0
	r.mu.Unlock()
	r.nextRound()
	return nil
}

// nextRound advances to the next round or finishes the match.
func (r *Room) nextRound() {
	r.mu.Lock()
	r.gen++
	g := r.gen
	r.roundNo++
	if r.roundNo > r.cfg.Rounds {
		r.state = StateFinished
		r.roundOver = true
		standings := r.standingsLocked()
		r.mu.Unlock()
		r.broadcast("match_end", map[string]any{"standings": standings})
		return
	}
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
	r.deadline = time.Now().Add(time.Duration(game.MustConfig(r.cfg.Mode).TimeLimitSec) * time.Second)
	r.roundOver = false
	r.solvedBy = ""
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

// expire closes the round with no winner.
func (r *Room) expire(g int) {
	r.mu.Lock()
	if r.gen != g || r.roundOver || r.state != StateRound {
		r.mu.Unlock()
		return
	}
	r.roundOver = true
	sol := ""
	if sols := game.Solve(r.numbersForSolution()); len(sols) > 0 {
		sol = sols[0].Expr
	}
	result := map[string]any{
		"roundNo": r.roundNo, "winner": nil, "solution": sol,
		"standings": r.standingsLocked(),
	}
	r.state = StateSummary
	r.mu.Unlock()
	r.broadcast("round_result", result)
	r.scheduleNext(g)
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

func (r *Room) scheduleNext(g int) {
	time.AfterFunc(r.summaryDur, func() {
		r.mu.Lock()
		same := r.gen == g
		r.mu.Unlock()
		if same {
			r.nextRound()
		}
	})
}

// Submit verifies a trace; the first correct one wins the round.
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
	hand, ok := r.numbers[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no active hand")
	}
	if err := game.Verify(hand, steps); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("wrong: %v", err)
	}

	remaining := int64(time.Until(r.deadline).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	cfg, _ := game.Config(r.cfg.Mode)
	points := (10 + remaining) * int64(cfg.Multiplier)
	p.score += points
	p.wins++
	r.roundOver = true
	r.solvedBy = sessionID
	expr := game.ExprFromSteps(hand, steps)
	r.state = StateSummary

	if p.dbID != "" && r.award != nil {
		go r.award(p.dbID, r.cfg.Mode, points)
	}

	result := map[string]any{
		"roundNo":    r.roundNo,
		"winner":     p.id,
		"winnerName": p.name,
		"expr":       expr,
		"points":     points,
		"standings":  r.standingsLocked(),
	}
	g := r.gen
	r.mu.Unlock()

	r.broadcast("round_result", result)
	r.scheduleNext(g)
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
	r.numbers[sessionID] = hand
	r.mu.Unlock()
	r.sendTo(sessionID, "regen", map[string]any{"numbers": hand})
	r.sendState()
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

func (r *Room) viewLocked(p *player) playerView {
	return playerView{
		ID: p.id, Name: p.name, Guest: p.dbID == "", Level: p.level, Tier: p.tier,
		Score: p.score, Wins: p.wins, HintsLeft: p.hintsLeft, RegensLeft: p.regensLeft,
		Host: p.id == r.hostID, Absent: p.absent,
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
