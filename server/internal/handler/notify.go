// Live profile pushes: after any server-side mutation a player did not
// perform in the current tab — admin EXP/coin/tier adjustments, achievement
// grants and revocations, renames, bans, account deletion — the fresh state
// is fanned out through the presence broker to every open /ws/player socket
// of that player, so open tabs update without a page refresh. A ban is its
// own terminal event (`banned`, reason included) so the client can throw a
// ban dialog instead of a silent profile swap. The transport relay itself
// lives in wsplayer.go.
package handler

import (
	"encoding/json"

	"github.com/pattaradol9/game24/server/internal/presence"
	"github.com/pattaradol9/game24/server/internal/store"
)

// notifyPlayer pushes a fresh profile snapshot to every live socket of the
// player. Best-effort: with no broker wired (or a player with no open
// socket) it is a no-op.
func (a *API) notifyPlayer(p store.Player) {
	if a.Presence == nil {
		return
	}
	raw, err := json.Marshal(map[string]any{"player": a.toPlayerJSON(p)})
	if err != nil {
		return
	}
	a.Presence.Publish(p.ID, presence.Event{Name: "player", Data: raw})
}

// notifyPlayerDeleted tells every live socket of the player that the account
// is gone, so its open tabs drop the dead session at once.
func (a *API) notifyPlayerDeleted(playerID string) {
	if a.Presence == nil {
		return
	}
	a.Presence.Publish(playerID, presence.Event{Name: "deleted", Data: []byte("{}")})
}

// notifyPlayerBanned announces a fresh ban to every live socket of the
// player, reason included, so the open tabs surface the ban dialog and drop
// the session at once instead of stumbling into 403s.
func (a *API) notifyPlayerBanned(playerID, reason string) {
	if a.Presence == nil {
		return
	}
	raw, err := json.Marshal(map[string]any{"reason": reason})
	if err != nil {
		raw = []byte("{}")
	}
	a.Presence.Publish(playerID, presence.Event{Name: "banned", Data: raw})
}

// notifyBoosts tells every connected player that a server-wide boost changed
// — armed, stopped or retuned. The payload carries the full active-boost
// view ({"boosts": {…}} with every running kind, empty when none is), so the
// client can swap its buff icons in one shot. Best-effort, like the profile
// pushes; anyone offline picks the state up with their next profile fetch.
func (a *API) notifyBoosts() {
	if a.Presence == nil {
		return
	}
	raw, err := json.Marshal(map[string]any{"boosts": a.boostViews()})
	if err != nil {
		return
	}
	a.Presence.Broadcast(presence.Event{Name: "boost", Data: raw})
}
