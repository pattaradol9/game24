// Achievement and card-skin endpoints. Catalog reads are public (progress
// only shows up with a valid token); purchases and equips require a
// Google-signed-in player.
package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/achv"
	"github.com/pattaradol9/game24/server/internal/skins"
	"github.com/pattaradol9/game24/server/internal/store"
)

type textJSON struct {
	En string `json:"en"`
	Th string `json:"th"`
}

type achievementJSON struct {
	ID         string   `json:"id"`
	Tier       string   `json:"tier"`
	ExpReward  int64    `json:"expReward"`
	CoinReward int64    `json:"coinReward"`
	Target     int64    `json:"target"`
	Title      textJSON `json:"title"`
	Desc       textJSON `json:"desc"`
}

func toAchievementJSON(d achv.Def) achievementJSON {
	return achievementJSON{
		ID: d.ID, Tier: string(d.Tier),
		ExpReward: d.ExpReward, CoinReward: d.CoinReward, Target: d.Target,
		Title: textJSON{En: d.Title.En, Th: d.Title.Th},
		Desc:  textJSON{En: d.Desc.En, Th: d.Desc.Th},
	}
}

// achievements serves the full catalog plus, for signed-in players, their
// unlock timestamps and per-achievement progress.
func (a *API) achievements(w http.ResponseWriter, r *http.Request) {
	out := make([]achievementJSON, 0, len(achv.Catalog))
	for _, d := range achv.Catalog {
		out = append(out, toAchievementJSON(d))
	}
	resp := map[string]any{
		"achievements": out,
		"unlocked":     map[string]string{},
		"progress":     map[string]any{},
	}
	if p, ok := playerOf(r); ok && !p.IsGuest {
		unlocked, snap, err := a.Store.AchievementState(p.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp["unlocked"] = unlocked
		progress := map[string]any{}
		for _, d := range achv.Catalog {
			progress[d.ID] = map[string]int64{
				"current": achv.Value(d, snap),
				"target":  d.Target,
			}
		}
		resp["progress"] = progress
	}
	ok(w, resp)
}

type skinJSON struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Price  int64    `json:"price"`
	Rarity string   `json:"rarity"`
	Desc   textJSON `json:"desc"`
}

func toSkinJSON(s skins.Def) skinJSON {
	return skinJSON{
		ID: s.ID, Name: s.Name, Price: s.Price, Rarity: s.Rarity,
		Desc: textJSON{En: s.Desc.En, Th: s.Desc.Th},
	}
}

// skinCatalog serves every skin plus, for signed-in players, ownership and
// the equipped id.
func (a *API) skinCatalog(w http.ResponseWriter, r *http.Request) {
	catalog := make([]skinJSON, 0, len(skins.Catalog))
	for _, s := range skins.Catalog {
		catalog = append(catalog, toSkinJSON(s))
	}
	resp := map[string]any{
		"skins":    catalog,
		"owned":    []string{},
		"equipped": "",
	}
	if p, ok := playerOf(r); ok && !p.IsGuest {
		owned, err := a.Store.OwnedSkins(p.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp["owned"] = owned
		resp["equipped"] = p.Skin
	}
	ok(w, resp)
}

// buySkin exchanges coins for a skin. Guests are refused up front; every
// other failure maps to its store error.
func (a *API) buySkin(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	if p.IsGuest {
		a.googleOnly(w)
		return
	}
	if err := a.Store.BuySkin(p.ID, chi.URLParam(r, "id")); err != nil {
		a.skinError(w, err)
		return
	}
	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	ok(w, map[string]any{"player": a.toPlayerJSON(fresh)})
}

// equipSkin makes an owned skin (or the free classic) the player's active one.
func (a *API) equipSkin(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	if p.IsGuest {
		a.googleOnly(w)
		return
	}
	if err := a.Store.EquipSkin(p.ID, chi.URLParam(r, "id")); err != nil {
		a.skinError(w, err)
		return
	}
	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	ok(w, map[string]any{"player": a.toPlayerJSON(fresh)})
}

func (a *API) googleOnly(w http.ResponseWriter) {
	fail(w, http.StatusForbidden, "google sign-in required")
}

func (a *API) skinError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrGoogleRequired):
		a.googleOnly(w)
	case errors.Is(err, store.ErrInsufficientCoins):
		fail(w, http.StatusBadRequest, "insufficient coins")
	case errors.Is(err, store.ErrAlreadyOwned):
		fail(w, http.StatusConflict, "skin already owned")
	case errors.Is(err, store.ErrNotOwned):
		fail(w, http.StatusBadRequest, "skin not owned")
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "skin not found")
	default:
		fail(w, http.StatusInternalServerError, err.Error())
	}
}
