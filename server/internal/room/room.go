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
}

type Room struct {
	code    string
	cfg     Config
	award   AwardEXP
	hostKey string

	mu              sync.Mutex
	players         map[string]*player
	joinOrder       []string
	hostID          string
	state           string
	roundNo         int
	numbers         map[string][]int // session id -> active hand (shared or regenerated)
	sharedNumbers   []int
	deadline        time.Time
	summaryDur      time.Duration
	roundOver       bool
	solvedBy        string
	gen             int
	rnd             *rand.Rand
	removedCallback func(code string)
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

// Attach removes hook so the hub can drop finished/empty rooms.
func (r *Room) OnEmpty(fn func(code string)) { r.removedCallback = fn }

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

// Join registers a connection. Host is the first player.
func (r *Room) Join(info Info, conn Conn) {
	r.mu.Lock()
	p := &player{
		id: info.SessionID, name: info.Name, dbID: info.DBID,
		level: info.Level, tier: info.Tier, conn: conn,
		hintsLeft: r.cfg.HintQuota, regensLeft: r.cfg.RegenQuota,
	}
	r.players[p.id] = p
	r.joinOrder = append(r.joinOrder, p.id)
	if r.hostID == "" || (info.HostKey != "" && info.HostKey == r.hostKey) {
		r.hostID = p.id
	}
	// late joiner mid-match: hand them the shared hand
	if r.state == StateRound && !r.roundOver && r.sharedNumbers != nil {
		r.numbers[p.id] = r.sharedNumbers
	}
	r.mu.Unlock()
	r.sendState()
	r.sendTo(p.id, "welcome", map[string]any{"you": p.id, "state": r.snapshotLocked()})
}

// Leave drops a player; host duties move down the join order.
func (r *Room) Leave(sessionID string) {
	r.mu.Lock()
	p, ok := r.players[sessionID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.players, sessionID)
	delete(r.numbers, sessionID)
	for i, id := range r.joinOrder {
		if id == sessionID {
			r.joinOrder = append(r.joinOrder[:i], r.joinOrder[i+1:]...)
			break
		}
	}
	p.conn.CloseConn()
	if r.hostID == sessionID && len(r.joinOrder) > 0 {
		r.hostID = r.joinOrder[0]
	}
	empty := len(r.players) == 0
	r.mu.Unlock()
	if empty {
		if r.removedCallback != nil {
			r.removedCallback(r.code)
		}
		return
	}
	r.sendState()
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
		Host: p.id == r.hostID,
	}
}

func (r *Room) snapshotLocked() stateView {
	players := make([]playerView, 0, len(r.players))
	for _, id := range r.joinOrder {
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
	if ok {
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
		p.conn.Deliver(raw)
	}
}

// LevelFromProgress refreshes a signed-in player's level/tier display
// (called by the ws layer when the player re-authenticates).
func (r *Room) PlayerCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.players)
}
