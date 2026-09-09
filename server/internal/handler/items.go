// Consumable boost-item endpoints. The catalog read is public (inventory and
// live windows only show up with a valid token); buying and using require a
// Google-signed-in player, mirroring the skins gates.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/items"
	"github.com/pattaradol9/game24/server/internal/store"
)

// maxItemCount bounds the units one buy/use request may move. The 24h window
// cap makes anything past ~96 (24h of the shortest item) pointless for use;
// 200 is a generous sanity bound over that.
const maxItemCount = 200

type itemJSON struct {
	ID          string   `json:"id"`
	Name        textJSON `json:"name"`
	Desc        textJSON `json:"desc"`
	Kind        string   `json:"kind"`
	Multiplier  float64  `json:"multiplier"`
	DurationMin int      `json:"durationMinutes"`
	Price       int64    `json:"price"`
	Rarity      string   `json:"rarity"`
}

func toItemJSON(d items.Def) itemJSON {
	return itemJSON{
		ID: d.ID, Name: textJSON{En: d.Name.En, Th: d.Name.Th},
		Desc: textJSON{En: d.Desc.En, Th: d.Desc.Th},
		Kind: d.Kind, Multiplier: d.Multiplier, DurationMin: d.DurationMin, Price: d.Price,
		Rarity: d.Rarity,
	}
}

// itemCatalog serves every item plus, for signed-in players, their inventory
// counts and the item windows running right now (item id → endsAt RFC3339),
// so the shop and inventory can badge what is already active.
func (a *API) itemCatalog(w http.ResponseWriter, r *http.Request) {
	catalog := make([]itemJSON, 0, len(items.Catalog))
	for _, d := range items.Catalog {
		catalog = append(catalog, toItemJSON(d))
	}
	resp := map[string]any{
		"items":  catalog,
		"owned":  map[string]int64{},
		"active": map[string]string{},
	}
	if p, ok := playerOf(r); ok && !p.IsGuest {
		stacks, err := a.Store.PlayerItems(p.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		owned := map[string]int64{}
		for _, it := range stacks {
			owned[it.ID] = it.Qty
		}
		resp["owned"] = owned
		active := map[string]string{}
		live := a.Store.ActivePlayerBoosts(p.ID)
		for _, d := range items.Catalog {
			if ab, ok := live[store.BoostKind(d.Kind)]; ok && ab.Multiplier == d.Multiplier {
				active[d.ID] = ab.EndsAt.UTC().Format(time.RFC3339)
			}
		}
		resp["active"] = active
	}
	ok(w, resp)
}

// buyItem exchanges coins for one or more units of an item — the body may
// name how many (`{"count": N}`, default 1, capped at maxItemCount), the
// batch debits price×count at once. Guests are refused up front; every other
// failure maps to its store error.
func (a *API) buyItem(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	if p.IsGuest {
		a.googleOnly(w)
		return
	}
	id := chi.URLParam(r, "id")
	count, valid, err := itemCount(r)
	if !valid {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.Store.BuyItem(p.ID, id, count); err != nil {
		a.itemError(w, err)
		return
	}
	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	// coins and the inventory changed behind the player's other tabs
	a.notifyPlayer(fresh)
	ok(w, map[string]any{"player": a.toPlayerJSON(fresh), "count": count})
}

// itemCount reads the optional `{"count": N}` unit count shared by the buy
// and use endpoints: absent → 1; anything non-integral or outside
// 1..maxItemCount → ok=false with the client-facing message.
func itemCount(r *http.Request) (int, bool, error) {
	var body struct {
		Count *float64 `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		return 0, false, errors.New("invalid body")
	}
	if body.Count == nil {
		return 1, true, nil
	}
	n := *body.Count
	if n != math.Trunc(n) || n < 1 || n > maxItemCount {
		return 0, false, fmt.Errorf("count must be an integer between 1 and %d", maxItemCount)
	}
	return int(n), true, nil
}

// useItem consumes units from the inventory and arms the item's personal
// boost window; the fresh player JSON carries both the new stock and the
// stacked boost view. The body may name how many units to spend at once —
// `{"count": N}` (default 1, capped at maxItemCount); every unit adds its own
// duration to the window, trimmed at the 24h cap. `action` tells what
// happened to the kind's window — "fresh", "extended" (same multiplier
// stacked time) or "replaced" (a different multiplier took over) — `replaced`
// mirrors it as a bool and `capped` flags a window end trimmed to the cap.
func (a *API) useItem(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	if p.IsGuest {
		a.googleOnly(w)
		return
	}
	id := chi.URLParam(r, "id")
	count, valid, err := itemCount(r)
	if !valid {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	boost, action, capped, err := a.Store.UseItem(p.ID, id, count)
	if err != nil {
		a.itemError(w, err)
		return
	}
	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	// other open tabs pick the new boost icon up live
	a.notifyPlayer(fresh)
	ok(w, map[string]any{
		"player":   a.toPlayerJSON(fresh),
		"boost":    a.boostView("item", boost),
		"action":   action,
		"replaced": action == "replaced",
		"count":    count,
		"capped":   capped,
	})
}

func (a *API) itemError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrGoogleRequired):
		a.googleOnly(w)
	case errors.Is(err, store.ErrInsufficientCoins):
		fail(w, http.StatusBadRequest, "insufficient coins")
	case errors.Is(err, store.ErrNoItems):
		fail(w, http.StatusBadRequest, "no items left in inventory")
	case errors.Is(err, store.ErrInvalidCount):
		fail(w, http.StatusBadRequest, "invalid count")
	case errors.Is(err, store.ErrBoostDurationCap):
		fail(w, http.StatusBadRequest, "boost duration cap exceeded (24h)")
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "item not found")
	default:
		fail(w, http.StatusInternalServerError, err.Error())
	}
}
