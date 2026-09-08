package handler

import (
	"bytes"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

const adminEmail = "boss@game.test"

// newAdminTestAPI enables the admin portal by allowlisting one email and
// returns the API, its mux, and a signed-in admin session token.
func newAdminTestAPI(t *testing.T) (*API, *chi.Mux, string) {
	t.Helper()
	api, mux := newTestAPI(t)
	api.Cfg.AdminEmails = []string{adminEmail}
	api.Cfg.DBPath = filepath.Join(t.TempDir(), "test.db")
	_, token, err := api.Store.GoogleLogin("sub-admin", adminEmail, "Boss", "")
	if err != nil {
		t.Fatal(err)
	}
	return api, mux, token
}

func doJSON(t *testing.T, mux *chi.Mux, method, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	json.Unmarshal(res.Body.Bytes(), &out)
	return res.Code, out
}

func TestAdminDisabledWithoutAllowlist(t *testing.T) {
	_, mux := newTestAPI(t) // no AdminEmails configured
	for _, path := range []string{"/api/v1/admin/session", "/api/v1/admin/overview", "/api/v1/admin/players"} {
		if code, res := get(t, mux, path, ""); code != 404 || res["success"] != false {
			t.Fatalf("%s should 404 when disabled, got %d %v", path, code, res)
		}
	}
}

func TestAdminAllowlistEnforced(t *testing.T) {
	api, mux, adminToken := newAdminTestAPI(t)

	// not signed in
	if code, _ := get(t, mux, "/api/v1/admin/overview", ""); code != 401 {
		t.Fatalf("no token should 401, got %d", code)
	}
	// guest token: signed in but never an admin
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Guest"}, "")
	guestToken := data(res)["token"].(string)
	if code, _ := get(t, mux, "/api/v1/admin/overview", guestToken); code != 403 {
		t.Fatalf("guest token should 403, got %d", code)
	}
	// google account that is not on the allowlist
	_, plainToken, err := api.Store.GoogleLogin("sub-plain", "plain@game.test", "Plain", "")
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, mux, "/api/v1/admin/overview", plainToken); code != 403 {
		t.Fatalf("non-allowlisted google token should 403, got %d", code)
	}
	// allowlisted account passes and is flagged in its own profile
	if code, _ := get(t, mux, "/api/v1/admin/overview", adminToken); code != 200 {
		t.Fatalf("allowlisted token should 200, got %d", code)
	}
	code, res := get(t, mux, "/api/v1/me", adminToken)
	if code != 200 || data(res)["player"].(map[string]any)["isAdmin"] != true {
		t.Fatalf("allowlisted /me should carry isAdmin=true: %d %v", code, res)
	}
	code, res = get(t, mux, "/api/v1/me", guestToken)
	if code != 200 || data(res)["player"].(map[string]any)["isAdmin"] != false {
		t.Fatalf("guest /me should carry isAdmin=false: %d %v", code, res)
	}
}

func TestAdminBannedAdminLosesAccess(t *testing.T) {
	api, mux, adminToken := newAdminTestAPI(t)
	p, err := api.Store.PlayerByToken(adminToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Store.SetPlayerBanned("admin:x", p.ID, true, "misbehaving"); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, mux, "/api/v1/admin/overview", adminToken); code != 403 {
		t.Fatalf("banned admin should 403, got %d", code)
	}
}

func TestAdminCannotBanSelf(t *testing.T) {
	_, mux, adminToken := newAdminTestAPI(t)
	selfID := adminActorID(t, mux, adminToken)

	// banning the acting admin's own account is refused outright
	code, res := doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+selfID,
		map[string]any{"banned": true, "banReason": "oops"}, adminToken)
	if code != 400 {
		t.Fatalf("self ban = %d, want 400 (%v)", code, res)
	}
	// the refusal leaves no ban event and the admin session keeps working
	_, res = get(t, mux, "/api/v1/admin/events?action=admin.ban", adminToken)
	if total := data(res)["total"].(float64); total != 0 {
		t.Fatalf("admin.ban events = %v, want 0", total)
	}
	if code, _ := get(t, mux, "/api/v1/admin/overview", adminToken); code != 200 {
		t.Fatalf("admin should keep access after the refused self-ban, got %d", code)
	}
}

func TestAdminPortalFlow(t *testing.T) {
	_, mux, adminToken := newAdminTestAPI(t)

	// a signed-in player to administer
	code, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "AdminTarget"}, "")
	if code != 200 {
		t.Fatalf("create player: %d %v", code, res)
	}
	player := data(res)["player"].(map[string]any)
	playerID := player["id"].(string)
	playerToken := data(res)["token"].(string)

	// list players finds them
	code, res = get(t, mux, "/api/v1/admin/players?search=admintarget", adminToken)
	if code != 200 {
		t.Fatalf("admin players: %d %v", code, res)
	}
	if total := data(res)["total"].(float64); total != 1 {
		t.Fatalf("search total = %v", total)
	}

	// overview exposes counters
	code, res = get(t, mux, "/api/v1/admin/overview", adminToken)
	if code != 200 {
		t.Fatalf("overview: %d %v", code, res)
	}
	players := data(res)["players"].(map[string]any)
	if players["total"].(float64) != 2 {
		t.Fatalf("overview players = %v", players)
	}

	// session reports who is signed in
	code, res = get(t, mux, "/api/v1/admin/session", adminToken)
	if code != 200 || data(res)["admin"].(map[string]any)["nickname"] != "Boss" {
		t.Fatalf("session: %d %v", code, res)
	}

	// ban the player: their token stops working immediately
	code, res = doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+playerID,
		map[string]any{"banned": true, "banReason": "cheating"}, adminToken)
	if code != 200 {
		t.Fatalf("ban: %d %v", code, res)
	}
	banned := data(res)["player"].(map[string]any)
	if banned["banned"] != true || banned["banReason"] != "cheating" {
		t.Fatalf("banned player payload = %v", banned)
	}
	if code, _ = get(t, mux, "/api/v1/me", playerToken); code != 403 {
		t.Fatalf("banned player /me should 403, got %d", code)
	}

	// unbanning with the same token restores access immediately
	code, res = doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+playerID,
		map[string]any{"banned": false}, adminToken)
	if code != 200 || data(res)["player"].(map[string]any)["banned"] != false {
		t.Fatalf("unban: %d %v", code, res)
	}
	if code, _ = get(t, mux, "/api/v1/me", playerToken); code != 200 {
		t.Fatalf("unbanned player /me should 200, got %d", code)
	}

	// rename via admin
	code, res = doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+playerID,
		map[string]any{"nickname": "RenamedByAdmin"}, adminToken)
	if code != 200 || data(res)["player"].(map[string]any)["nickname"] != "RenamedByAdmin" {
		t.Fatalf("admin rename: %d %v", code, res)
	}

	// set EXP: mode stats + total resync, negative value rejected
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/exp",
		map[string]any{"mode": "queen", "value": 250}, adminToken)
	if code != 200 {
		t.Fatalf("set exp: %d %v", code, res)
	}
	after := data(res)["player"].(map[string]any)
	if after["totalExp"].(float64) != 250 {
		t.Fatalf("totalExp after set = %v", after["totalExp"])
	}
	if code, _ = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/exp",
		map[string]any{"mode": "queen", "value": -1}, adminToken); code != 400 {
		t.Fatal("negative value should 400")
	}

	// player detail carries per-mode stats
	code, res = get(t, mux, "/api/v1/admin/players/"+playerID, adminToken)
	if code != 200 {
		t.Fatalf("detail: %d %v", code, res)
	}
	perMode := data(res)["player"].(map[string]any)["perMode"].(map[string]any)
	if perMode["queen"] == nil {
		t.Fatalf("per-mode stats missing: %v", perMode)
	}

	// events recorded with the acting admin as actor
	code, res = get(t, mux, "/api/v1/admin/events?action=admin.ban", adminToken)
	if code != 200 {
		t.Fatalf("events: %d %v", code, res)
	}
	if total := data(res)["total"].(float64); total != 1 {
		t.Fatalf("admin.ban events = %v, want 1", total)
	}
	events := data(res)["events"].([]any)
	if actor := events[0].(map[string]any)["actor"].(string); actor != "admin:"+adminActorID(t, mux, adminToken) {
		t.Fatalf("ban event actor = %q, want acting admin id", actor)
	}

	// reset stats for a mode that has none -> 404; all-mode reset -> 200
	if code, _ = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/stats/reset",
		map[string]any{"mode": "ace"}, adminToken); code != 404 {
		t.Fatalf("unknown mode reset should 404, got %d", code)
	}
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/stats/reset",
		map[string]any{}, adminToken)
	if code != 200 || data(res)["player"].(map[string]any)["totalExp"].(float64) != 0 {
		t.Fatalf("full reset: %d %v", code, res)
	}

	// delete the player
	code, res = doJSON(t, mux, "DELETE", "/api/v1/admin/players/"+playerID, nil, adminToken)
	if code != 200 {
		t.Fatalf("delete: %d %v", code, res)
	}
	if code, _ = get(t, mux, "/api/v1/admin/players/"+playerID, adminToken); code != 404 {
		t.Fatalf("deleted player should 404, got %d", code)
	}
}

func adminActorID(t *testing.T, mux *chi.Mux, token string) string {
	t.Helper()
	_, res := get(t, mux, "/api/v1/admin/session", token)
	return data(res)["admin"].(map[string]any)["playerId"].(string)
}

func TestAdminCoinsAndAchievements(t *testing.T) {
	_, mux, adminToken := newAdminTestAPI(t)

	code, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Coiny"}, "")
	if code != 200 {
		t.Fatalf("create player: %d %v", code, res)
	}
	playerID := data(res)["player"].(map[string]any)["id"].(string)
	base := "/api/v1/admin/players/" + playerID

	// coin set lands on the player; negative values are rejected
	code, res = doJSON(t, mux, "POST", base+"/coins", map[string]any{"value": 500}, adminToken)
	if code != 200 || data(res)["player"].(map[string]any)["totalCoins"].(float64) != 500 {
		t.Fatalf("coin set: %d %v", code, res)
	}
	if code, _ = doJSON(t, mux, "POST", base+"/coins", map[string]any{"value": -1}, adminToken); code != 400 {
		t.Fatal("negative coin value should 400")
	}
	if code, _ = doJSON(t, mux, "POST", base+"/coins", map[string]any{"value": 5}, "not-a-token"); code != 401 {
		t.Fatal("coins endpoint must stay admin-only")
	}
	code, res = get(t, mux, base, adminToken)
	if got := data(res)["player"].(map[string]any)["totalCoins"].(float64); got != 500 {
		t.Fatalf("detail coins = %v", got)
	}
	if n := len(data(res)["achievements"].([]any)); n != 0 {
		t.Fatalf("fresh player achievements = %d", n)
	}

	// granting an achievement pays its standard rewards (solve-25: 40/25)
	code, res = doJSON(t, mux, "POST", base+"/achievements/grant", map[string]any{"id": "solve-25"}, adminToken)
	if code != 200 {
		t.Fatalf("grant: %d %v", code, res)
	}
	after := data(res)["player"].(map[string]any)
	if after["totalExp"].(float64) != 40 || after["totalCoins"].(float64) != 525 {
		t.Fatalf("after grant: exp=%v coins=%v", after["totalExp"], after["totalCoins"])
	}
	if code, _ = doJSON(t, mux, "POST", base+"/achievements/grant", map[string]any{"id": "solve-25"}, adminToken); code != 409 {
		t.Fatal("double grant should 409")
	}
	if code, _ = doJSON(t, mux, "POST", base+"/achievements/grant", map[string]any{"id": "nope"}, adminToken); code != 404 {
		t.Fatal("unknown achievement should 404")
	}

	// detail lists the unlock with its timestamp
	code, res = get(t, mux, base, adminToken)
	achs := data(res)["achievements"].([]any)
	if len(achs) != 1 || achs[0].(map[string]any)["id"] != "solve-25" {
		t.Fatalf("detail achievements = %v", achs)
	}
	if _, ok := achs[0].(map[string]any)["unlockedAt"].(string); !ok {
		t.Fatalf("unlockedAt missing: %v", achs[0])
	}

	// revoke drops the unlock; banked rewards stay
	if code, _ = doJSON(t, mux, "POST", base+"/achievements/revoke", map[string]any{"id": "solve-25"}, adminToken); code != 200 {
		t.Fatal("revoke should 200")
	}
	if code, _ = doJSON(t, mux, "POST", base+"/achievements/revoke", map[string]any{"id": "solve-25"}, adminToken); code != 404 {
		t.Fatal("double revoke should 404")
	}
	code, res = get(t, mux, base, adminToken)
	if got := data(res)["player"].(map[string]any)["totalCoins"].(float64); got != 525 {
		t.Fatalf("revoke clawed back coins: %v", got)
	}
	if n := len(data(res)["achievements"].([]any)); n != 0 {
		t.Fatal("achievement survived revoke")
	}

	// every mutation left its audit trail
	for action, want := range map[string]float64{
		"admin.coins.set":          1,
		"admin.achievement.grant":  1,
		"admin.achievement.revoke": 1,
	} {
		_, res = get(t, mux, "/api/v1/admin/events?action="+action, adminToken)
		if total := data(res)["total"].(float64); total != want {
			t.Fatalf("%s events = %v, want %v", action, total, want)
		}
	}
}

func TestAdminSetTier(t *testing.T) {
	_, mux, adminToken := newAdminTestAPI(t)

	code, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Tiered"}, "")
	if code != 200 {
		t.Fatalf("create player: %d %v", code, res)
	}
	playerID := data(res)["player"].(map[string]any)["id"].(string)
	playerToken := data(res)["token"].(string)

	// unknown tier names are rejected up front
	if code, _ = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/tier",
		map[string]any{"tier": "diamond+"}, adminToken); code != 400 {
		t.Fatal("unknown tier should 400")
	}

	// the override shows up in the player JSON with the tier bonus applied
	// (master: TierBonusQuota 2 → single hint quota 3)
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/tier",
		map[string]any{"tier": "master"}, adminToken)
	if code != 200 {
		t.Fatalf("set tier: %d %v", code, res)
	}
	after := data(res)["player"].(map[string]any)
	if after["tier"] != "master" || after["tierOverride"] != true {
		t.Fatalf("player after override = %v", after)
	}
	code, res = get(t, mux, "/api/v1/me", playerToken)
	me := data(res)["player"].(map[string]any)
	if me["tier"] != "master" || me["tierHintBonus"].(float64) != 2 {
		t.Fatalf("guest /me after override = %v", me)
	}

	// admin detail flags the override; clearing falls back to the level
	code, res = get(t, mux, "/api/v1/admin/players/"+playerID, adminToken)
	if data(res)["player"].(map[string]any)["tierOverride"] != true {
		t.Fatal("detail should flag the tier override")
	}
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/tier",
		map[string]any{"tier": ""}, adminToken)
	if code != 200 {
		t.Fatalf("clear tier: %d %v", code, res)
	}
	cleared := data(res)["player"].(map[string]any)
	if cleared["tier"] != "bronze" || cleared["tierOverride"] == true {
		t.Fatalf("player after clear = %v", cleared)
	}

	// both mutations left their audit trail
	_, res = get(t, mux, "/api/v1/admin/events?action=admin.tier.set", adminToken)
	if total := data(res)["total"].(float64); total != 2 {
		t.Fatalf("admin.tier.set events = %v, want 2", total)
	}
}

func TestAdminLeaderboardIncludesBannedFlag(t *testing.T) {
	_, mux, adminToken := newAdminTestAPI(t)
	code, res := get(t, mux, "/api/v1/admin/leaderboard?mode=queen", adminToken)
	if code != 200 {
		t.Fatalf("admin leaderboard: %d %v", code, res)
	}
	entries := data(res)["entries"].([]any)
	if len(entries) != 0 {
		t.Fatalf("expected empty board, got %v", entries)
	}
	if code, _ := get(t, mux, "/api/v1/admin/leaderboard?mode=nope", adminToken); code != 400 {
		t.Fatal("unknown mode should 400")
	}
}

func TestAdminSettingsBackupAndReset(t *testing.T) {
	api, mux, adminToken := newAdminTestAPI(t)
	backupDir := filepath.Dir(api.Cfg.DBPath)

	// seed something to wipe
	if code, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Doomed"}, ""); code != 200 {
		t.Fatalf("seed player: %d %v", code, res)
	}

	// settings shows counts
	code, res := get(t, mux, "/api/v1/admin/settings", adminToken)
	if code != 200 {
		t.Fatalf("settings: %d %v", code, res)
	}
	settings := data(res)
	if settings["players"].(float64) != 2 || settings["dbFile"] != "test.db" {
		t.Fatalf("settings payload = %v", settings)
	}

	// backup streams a sqlite file
	req := httptest.NewRequest("GET", "/api/v1/admin/db/backup", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	bres := httptest.NewRecorder()
	mux.ServeHTTP(bres, req)
	if bres.Code != 200 {
		t.Fatalf("backup: %d %s", bres.Code, bres.Body.String())
	}
	if cd := bres.Header().Get("Content-Disposition"); !strings.Contains(cd, "game24-backup-") {
		t.Fatalf("content disposition = %q", cd)
	}
	if magic := bres.Body.Bytes()[:15]; string(magic) != "SQLite format 3"[:15] {
		t.Fatalf("backup is not a sqlite file: %q", magic)
	}
	// a manual download must not leave files behind
	matches, _ := filepath.Glob(filepath.Join(backupDir, "game24-backup-*"))
	if len(matches) != 0 {
		t.Fatalf("backup download left files behind: %v", matches)
	}

	// reset requires the confirmation word
	if code, _ := doJSON(t, mux, "POST", "/api/v1/admin/db/reset", map[string]string{"confirm": "yes"}, adminToken); code != 400 {
		t.Fatal("reset without RESET confirmation should 400")
	}
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/db/reset", map[string]string{"confirm": "RESET"}, adminToken)
	if code != 200 {
		t.Fatalf("reset: %d %v", code, res)
	}
	wiped := data(res)["wiped"].(map[string]any)
	if wiped["players"].(float64) != 2 {
		t.Fatalf("wiped = %v", wiped)
	}
	if data(res)["backupFile"] == "" {
		t.Fatal("reset did not report a backup file")
	}

	// the pre-reset safety backup stays on disk next to the database
	matches, _ = filepath.Glob(filepath.Join(backupDir, "game24-backup-*"))
	if len(matches) != 1 {
		t.Fatalf("reset should keep exactly one safety backup, found %v", matches)
	}

	// database is empty and the admin session died with it
	if code, _ := get(t, mux, "/api/v1/admin/overview", adminToken); code != 401 {
		t.Fatalf("admin token should die on reset, got %d", code)
	}
	code, res = get(t, mux, "/api/v1/leaderboard?mode=queen", "")
	if code != 200 || len(data(res)["entries"].([]any)) != 0 {
		t.Fatalf("leaderboard after reset: %d %v", code, res)
	}
}

func TestAdminSettingsRestoreBackup(t *testing.T) {
	api, mux, adminToken := newAdminTestAPI(t)
	backupDir := filepath.Dir(api.Cfg.DBPath)

	// seed a guest so the snapshot has content beyond the admin account
	if code, _ := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Keeper"}, ""); code != 200 {
		t.Fatal("seed player failed")
	}

	// download a snapshot and place it as a server-side backup
	req := httptest.NewRequest("GET", "/api/v1/admin/db/backup", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	bres := httptest.NewRecorder()
	mux.ServeHTTP(bres, req)
	if bres.Code != 200 {
		t.Fatalf("backup: %d %s", bres.Code, bres.Body.String())
	}
	snap := bres.Body.Bytes()
	snapName := "game24-backup-20990101-000000.db"
	if err := os.WriteFile(filepath.Join(backupDir, snapName), snap, 0o644); err != nil {
		t.Fatal(err)
	}

	// diverge after the snapshot: one more guest appears
	if code, _ := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Latecomer"}, ""); code != 200 {
		t.Fatal("second seed failed")
	}

	// the listing shows the placed snapshot with its size
	code, res := get(t, mux, "/api/v1/admin/db/backups", adminToken)
	if code != 200 {
		t.Fatalf("backups list: %d %v", code, res)
	}
	list, _ := data(res)["backups"].([]any)
	if len(list) != 1 {
		t.Fatalf("backups = %v, want one entry", list)
	}
	row := list[0].(map[string]any)
	if row["name"] != snapName || row["size"].(float64) == 0 || row["modified"] == "" {
		t.Fatalf("backup entry = %v", row)
	}

	// the confirmation word is required
	if code, _ := doJSON(t, mux, "POST", "/api/v1/admin/db/restore",
		map[string]string{"name": snapName, "confirm": "yes"}, adminToken); code != 400 {
		t.Fatal("restore without RESTORE confirmation should 400")
	}
	// names are constrained to server-written snapshots — no traversal
	if code, _ := doJSON(t, mux, "POST", "/api/v1/admin/db/restore",
		map[string]string{"name": "../evil.db", "confirm": "RESTORE"}, adminToken); code != 400 {
		t.Fatal("path traversal should 400")
	}
	if code, _ := doJSON(t, mux, "POST", "/api/v1/admin/db/restore",
		map[string]string{"name": "game24-backup-20990101-999999.db", "confirm": "RESTORE"}, adminToken); code != 404 {
		t.Fatal("missing backup should 404")
	}
	if code, _ := doJSON(t, mux, "POST", "/api/v1/admin/db/restore",
		map[string]string{"confirm": "RESTORE"}, adminToken); code != 400 {
		t.Fatal("restore without a source should 400")
	}

	// restore by name: the pre-snapshot world comes back
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/db/restore",
		map[string]string{"name": snapName, "confirm": "RESTORE"}, adminToken)
	if code != 200 {
		t.Fatalf("restore: %d %v", code, res)
	}
	rows := data(res)["restoredRows"].(map[string]any)
	if rows["players"].(float64) != 2 || data(res)["safetyBackup"] == "" {
		t.Fatalf("restore payload = %v", res)
	}
	// Latecomer is gone, the admin token recorded in the snapshot lives again
	code, res = get(t, mux, "/api/v1/admin/players", adminToken)
	if code != 200 || data(res)["total"].(float64) != 2 {
		t.Fatalf("players after restore: %d %v", code, res)
	}
	// a safety snapshot of the pre-restore state joined the listing
	matches, _ := filepath.Glob(filepath.Join(backupDir, "game24-backup-*"))
	if len(matches) != 2 {
		t.Fatalf("expected snapshot + safety backup on disk, found %v", matches)
	}

	// upload path: the same snapshot comes back as multipart
	if code, _ := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Latecomer2"}, ""); code != 200 {
		t.Fatal("third seed failed")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("confirm", "RESTORE")
	fw, _ := mw.CreateFormFile("file", "snap.db")
	fw.Write(snap)
	mw.Close()
	ureq := httptest.NewRequest("POST", "/api/v1/admin/db/restore", &buf)
	ureq.Header.Set("Content-Type", mw.FormDataContentType())
	ureq.Header.Set("Authorization", "Bearer "+adminToken)
	ures := httptest.NewRecorder()
	mux.ServeHTTP(ures, ureq)
	if ures.Code != 200 {
		t.Fatalf("upload restore: %d %s", ures.Code, ures.Body.String())
	}
	var upRes map[string]any
	json.Unmarshal(ures.Body.Bytes(), &upRes)
	if upRows := data(upRes)["restoredRows"].(map[string]any); upRows["players"].(float64) != 2 {
		t.Fatalf("upload restore rows = %v", upRes)
	}

	// a garbage upload is refused and leaves nothing behind
	var bad bytes.Buffer
	bw := multipart.NewWriter(&bad)
	bw.WriteField("confirm", "RESTORE")
	fw, _ = bw.CreateFormFile("file", "junk.db")
	fw.Write([]byte("not a database"))
	bw.Close()
	breq := httptest.NewRequest("POST", "/api/v1/admin/db/restore", &bad)
	breq.Header.Set("Content-Type", bw.FormDataContentType())
	breq.Header.Set("Authorization", "Bearer "+adminToken)
	bres2 := httptest.NewRecorder()
	mux.ServeHTTP(bres2, breq)
	if bres2.Code != 400 {
		t.Fatalf("garbage upload should 400, got %d %s", bres2.Code, bres2.Body.String())
	}

	// staged uploads are cleaned up; only the timestamped snapshots remain
	leftovers, _ := filepath.Glob(filepath.Join(backupDir, "game24-upload-*"))
	if len(leftovers) != 0 {
		t.Fatalf("staged uploads left behind: %v", leftovers)
	}
	matches, _ = filepath.Glob(filepath.Join(backupDir, "game24-backup-*"))
	if len(matches) != 3 { // placed snapshot + two safety backups
		t.Fatalf("expected three backup files, found %v", matches)
	}
}

func TestAdminServerBoosts(t *testing.T) {
	api, mux, adminTok := newAdminTestAPI(t)
	playerTok := googlePlayerToken(t, api, "booster")

	// the boosts group lives behind the admin gate like everything else
	if code, _ := get(t, mux, "/api/v1/admin/boosts", playerTok); code != 403 {
		t.Fatalf("player boosts access = %d, want 403", code)
	}
	if code, _ := get(t, mux, "/api/v1/admin/boosts/exp", adminTok); code != 200 {
		t.Fatalf("admin exp boost get = %d, want 200", code)
	}
	if code, _ := get(t, mux, "/api/v1/admin/boosts/magic", adminTok); code != 404 {
		t.Fatalf("unknown kind = %d, want 404", code)
	}

	// fresh state: both kinds are disarmed 2x/60min drafts
	code, res := get(t, mux, "/api/v1/admin/boosts", adminTok)
	if code != 200 {
		t.Fatalf("boosts list: %d %v", code, res)
	}
	list := data(res)["boosts"].([]any)
	if len(list) != 2 {
		t.Fatalf("boosts list size = %d, want 2", len(list))
	}
	first := list[0].(map[string]any)
	if first["kind"] != "exp" || first["multiplier"].(float64) != 2 || first["active"].(bool) {
		t.Fatalf("fresh exp boost = %v, want a disarmed 2x draft", first)
	}

	// out-of-range values are refused without touching the row
	for _, body := range []map[string]any{
		{"multiplier": 0, "durationMinutes": 60},
		{"multiplier": 3, "durationMinutes": 0},
		{"multiplier": 500.5, "durationMinutes": 60},
	} {
		if code, _ := doJSON(t, mux, "PUT", "/api/v1/admin/boosts/exp", body, adminTok); code != 400 {
			t.Fatalf("invalid boost %v -> %d, want 400", body, code)
		}
	}

	// save drafts, then arm the exp ×2.5 boost; the coin draft stays idle
	code, res = doJSON(t, mux, "PUT", "/api/v1/admin/boosts/exp",
		map[string]any{"multiplier": 2.5, "durationMinutes": 90, "label": "Weekend"}, adminTok)
	if code != 200 {
		t.Fatalf("boost put: %d %v", code, res)
	}
	if b := data(res)["boost"].(map[string]any); b["active"].(bool) || b["multiplier"].(float64) != 2.5 {
		t.Fatalf("saved draft = %v, want disarmed 2.5x", b)
	}
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/boosts/exp/enable", nil, adminTok)
	if code != 200 {
		t.Fatalf("boost enable: %d %v", code, res)
	}
	if b := data(res)["boost"].(map[string]any); !b["active"].(bool) || b["remainingSec"].(float64) <= 0 || b["endsAt"] == "" {
		t.Fatalf("armed boost = %v, want an active window", b)
	}

	// the running boost shows up in the player's profile
	code, res = get(t, mux, "/api/v1/me", playerTok)
	if code != 200 {
		t.Fatalf("me: %d %v", code, res)
	}
	player := data(res)["player"].(map[string]any)
	boosts := player["boosts"].(map[string]any)
	if _, ok := boosts["coins"]; ok {
		t.Fatalf("idle coin boost must not appear in the profile: %v", boosts)
	}
	if boost := boosts["exp"].(map[string]any); boost["multiplier"].(float64) != 2.5 {
		t.Fatalf("player exp boost = %v, want 2.5x", boost)
	}

	// a solved hand pays the multiplied EXP; coins stay on base points
	d := solveOneHand(t, mux, playerTok, "queen")
	points, exp := d["points"].(float64), d["exp"].(float64)
	if want := math.Round(points * 2.5); exp != want {
		t.Fatalf("exp = %v, want %v (points %v)", exp, want, points)
	}
	if exp <= points {
		t.Fatalf("boosted exp %v must exceed base points %v", exp, points)
	}
	baseCoins := math.Floor(points / 10)
	if coins := d["coins"].(float64); coins != baseCoins {
		t.Fatalf("coins = %v, want %v (unboosted)", coins, baseCoins)
	}

	// arm the coin boost too, next hand pays both multipliers
	code, res = doJSON(t, mux, "PUT", "/api/v1/admin/boosts/coins",
		map[string]any{"multiplier": 3, "durationMinutes": 30}, adminTok)
	if code != 200 {
		t.Fatalf("coin boost put: %d %v", code, res)
	}
	if _, res = doJSON(t, mux, "POST", "/api/v1/admin/boosts/coins/enable", nil, adminTok); code != 200 {
		t.Fatalf("coin boost enable: %d %v", code, res)
	}
	code, res = get(t, mux, "/api/v1/me", playerTok)
	boosts = data(res)["player"].(map[string]any)["boosts"].(map[string]any)
	if len(boosts) != 2 {
		t.Fatalf("profile boosts = %v, want both kinds running", boosts)
	}
	d = solveOneHand(t, mux, playerTok, "queen")
	points, exp = d["points"].(float64), d["exp"].(float64)
	if want := math.Round(points * 2.5); exp != want {
		t.Fatalf("exp = %v, want %v", exp, want)
	}
	if want := math.Floor(points/10) * 3; d["coins"].(float64) != want {
		t.Fatalf("coins = %v, want %v (×3)", d["coins"], want)
	}

	// disabling stops the payouts and clears the profile's boost view
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/boosts/exp/disable", nil, adminTok)
	if code != 200 {
		t.Fatalf("boost disable: %d %v", code, res)
	}
	if data(res)["boost"].(map[string]any)["active"].(bool) {
		t.Fatal("boost still active after disable")
	}
	d = solveOneHand(t, mux, playerTok, "queen")
	if d["exp"].(float64) != d["points"].(float64) {
		t.Fatalf("exp after disable = %v, want base %v", d["exp"], d["points"])
	}
	code, res = get(t, mux, "/api/v1/me", playerTok)
	boosts = data(res)["player"].(map[string]any)["boosts"].(map[string]any)
	if _, ok := boosts["exp"]; ok {
		t.Fatalf("exp boost must vanish from the profile once disabled: %v", boosts)
	}
	if _, ok := boosts["coins"]; !ok {
		t.Fatal("coin boost must still be listed while it runs")
	}
}
