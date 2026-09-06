package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
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

	// rename via admin
	code, res = doJSON(t, mux, "PATCH", "/api/v1/admin/players/"+playerID,
		map[string]any{"nickname": "RenamedByAdmin"}, adminToken)
	if code != 200 || data(res)["player"].(map[string]any)["nickname"] != "RenamedByAdmin" {
		t.Fatalf("admin rename: %d %v", code, res)
	}

	// grant EXP: mode stats + total resync, zero delta rejected
	code, res = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/exp",
		map[string]any{"mode": "queen", "delta": 250}, adminToken)
	if code != 200 {
		t.Fatalf("adjust exp: %d %v", code, res)
	}
	after := data(res)["player"].(map[string]any)
	if after["totalExp"].(float64) != 250 {
		t.Fatalf("totalExp after grant = %v", after["totalExp"])
	}
	if code, _ = doJSON(t, mux, "POST", "/api/v1/admin/players/"+playerID+"/exp",
		map[string]any{"mode": "queen", "delta": 0}, adminToken); code != 400 {
		t.Fatal("zero delta should 400")
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
