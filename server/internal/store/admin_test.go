package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pattaradol9/game24/server/internal/crypto"
)

func TestBanBlocksAuthLoginAndLeaderboard(t *testing.T) {
	s := openTest(t)
	gp, tok, err := s.GoogleLogin("sub-ban", "a@b.c", "Banned", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 120, true); err != nil {
		t.Fatal(err)
	}

	// ban: token auth fails, google re-login fails, public board hides them
	if _, err = s.SetPlayerBanned("admin:x", gp.ID, true, "cheating"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlayerByToken(tok); !errors.Is(err, ErrBanned) {
		t.Fatalf("banned token auth err = %v, want ErrBanned", err)
	}
	if _, _, err := s.GoogleLogin("sub-ban", "a@b.c", "Banned", ""); !errors.Is(err, ErrBanned) {
		t.Fatalf("banned google login err = %v, want ErrBanned", err)
	}
	board, err := s.Leaderboard("queen", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 0 {
		t.Fatalf("banned player still on public board: %+v", board)
	}

	// admin board still shows them, flagged
	adminBoard, err := s.AdminLeaderboard("queen", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminBoard) != 1 || !adminBoard[0].Banned {
		t.Fatalf("admin board = %+v, want one banned row", adminBoard)
	}

	// unban restores everything
	p, err := s.SetPlayerBanned("admin:x", gp.ID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Banned || p.BanReason != "" {
		t.Fatalf("unbanned player = %+v", p)
	}
	if _, err = s.PlayerByToken(tok); err != nil {
		t.Fatalf("token rejected after unban: %v", err)
	}
	board, _ = s.Leaderboard("queen", 10, 0)
	if len(board) != 1 {
		t.Fatalf("player missing from board after unban: %+v", board)
	}
}

func TestAdminListPlayersSearchAndFilters(t *testing.T) {
	s := openTest(t)
	if _, _, err := s.CreateGuest("GuestNick"); err != nil {
		t.Fatal(err)
	}
	gp, _, err := s.GoogleLogin("sub-list", "findme@mail.com", "GoogleNick", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetPlayerBanned("admin:x", gp.ID, true, "test"); err != nil {
		t.Fatal(err)
	}

	rows, total, err := s.AdminListPlayers("", "all", 50, 0)
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("list all = %d rows (total %d), err %v", len(rows), total, err)
	}

	rows, total, err = s.AdminListPlayers("googlenick", "", 50, 0)
	if err != nil || total != 1 || rows[0].ID != gp.ID {
		t.Fatalf("search by nickname = %+v (total %d), err %v", rows, total, err)
	}

	rows, total, err = s.AdminListPlayers("findme", "", 50, 0)
	if err != nil || total != 1 || rows[0].ID != gp.ID {
		t.Fatalf("search by email = %+v (total %d), err %v", rows, total, err)
	}

	rows, total, err = s.AdminListPlayers("", "banned", 50, 0)
	if err != nil || total != 1 || rows[0].ID != gp.ID {
		t.Fatalf("filter banned = %+v (total %d), err %v", rows, total, err)
	}

	rows, total, err = s.AdminListPlayers("", "guests", 50, 0)
	if err != nil || total != 1 || rows[0].IsGuest != true {
		t.Fatalf("filter guests = %+v (total %d), err %v", rows, total, err)
	}
}

func TestSetEXPResyncsTotal(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-exp", "e@f.g", "E", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"jack", "queen"} {
		if _, _, err = s.AwardEXP(gp.ID, m, 100, true); err != nil {
			t.Fatal(err)
		}
	}

	// overwrite: the mode takes the absolute value, total resyncs
	p, err := s.SetPlayerEXP("admin:x", gp.ID, "queen", 50)
	if err != nil {
		t.Fatal(err)
	}
	if p.Stats["queen"].Exp != 50 || p.TotalExp != 150 {
		t.Fatalf("after overwrite: queen=%d total=%d", p.Stats["queen"].Exp, p.TotalExp)
	}

	// a negative value clamps at zero and keeps total = sum of modes
	p, err = s.SetPlayerEXP("admin:x", gp.ID, "queen", -5)
	if err != nil {
		t.Fatal(err)
	}
	if p.Stats["queen"].Exp != 0 {
		t.Fatalf("queen exp should clamp at 0, got %d", p.Stats["queen"].Exp)
	}
	if p.TotalExp != 100 {
		t.Fatalf("total should resync to 100, got %d", p.TotalExp)
	}
}

func TestResetPlayerStats(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-reset", "r@t.v", "R", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"jack", "king"} {
		if _, _, err = s.AwardEXP(gp.ID, m, 80, true); err != nil {
			t.Fatal(err)
		}
	}

	// reset one mode: the other survives, total resyncs
	p, err := s.ResetPlayerStats("admin:x", gp.ID, "king")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Stats["king"]; ok {
		t.Fatal("king stats survived a reset")
	}
	if p.Stats["jack"].Exp != 80 || p.TotalExp != 80 {
		t.Fatalf("after partial reset: %+v total=%d", p.Stats, p.TotalExp)
	}

	// resetting an unknown mode reports not found
	if _, err = s.ResetPlayerStats("admin:x", gp.ID, "ace"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown mode reset err = %v, want ErrNotFound", err)
	}

	// reset everything
	p, err = s.ResetPlayerStats("admin:x", gp.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Stats) != 0 || p.TotalExp != 0 {
		t.Fatalf("after full reset: %+v total=%d", p.Stats, p.TotalExp)
	}
}

func TestDeletePlayerCascades(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-del", "d@e.f", "D", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 60, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRound(gp.ID, "queen", []int{1, 2, 3, 4}, ""); err != nil {
		t.Fatal(err)
	}

	if err = s.DeletePlayer("admin:x", gp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PlayerByID(gp.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted player lookup err = %v, want ErrNotFound", err)
	}
	var rounds, stats int
	s.db.QueryRow(`SELECT COUNT(*) FROM rounds`).Scan(&rounds)
	s.db.QueryRow(`SELECT COUNT(*) FROM player_mode_stats`).Scan(&stats)
	if rounds != 0 || stats != 0 {
		t.Fatalf("cascade left rows behind: rounds=%d stats=%d", rounds, stats)
	}
	if err = s.DeletePlayer("admin:x", gp.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double delete err = %v, want ErrNotFound", err)
	}
}

func TestDeleteOwnPlayer(t *testing.T) {
	s := openTest(t)
	p, token, err := s.CreateGuest("Wipee")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(p.ID, "queen", 60, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRound(p.ID, "queen", []int{1, 2, 3, 4}, ""); err != nil {
		t.Fatal(err)
	}

	if err = s.DeleteOwnPlayer(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.PlayerByID(p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted player lookup err = %v, want ErrNotFound", err)
	}
	if _, err = s.PlayerByToken(token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token after self-delete err = %v, want ErrNotFound", err)
	}
	var rounds, stats int
	s.db.QueryRow(`SELECT COUNT(*) FROM rounds`).Scan(&rounds)
	s.db.QueryRow(`SELECT COUNT(*) FROM player_mode_stats`).Scan(&stats)
	if rounds != 0 || stats != 0 {
		t.Fatalf("cascade left rows behind: rounds=%d stats=%d", rounds, stats)
	}
	// the audit trail names self-service deletions apart from admin ones
	events, _, err := s.AdminEvents("player.delete", 10, 0)
	if err != nil || len(events) != 1 {
		t.Fatalf("player.delete events err = %v (len %d), want exactly one", err, len(events))
	}
	if events[0].Actor != "player:"+p.ID {
		t.Fatalf("actor = %q, want player:%s", events[0].Actor, p.ID)
	}
	if err = s.DeleteOwnPlayer(p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double delete err = %v, want ErrNotFound", err)
	}
}

func TestEventLogRecordsLifecycleAndAdminActions(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-ev", "g@h.i", "G", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetPlayerBanned("admin:x", gp.ID, true, "spam"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetPlayerEXP("admin:x", gp.ID, "queen", 10); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ResetPlayerStats("admin:x", gp.ID, ""); err != nil {
		t.Fatal(err)
	}

	actions := map[string]bool{}
	events, total, err := s.AdminEvents("", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		actions[e.Action] = true
		if e.Target != gp.ID && e.Actor != "system" && !strings.HasPrefix(e.Actor, "admin") {
			t.Fatalf("unexpected actor/target: %+v", e)
		}
	}
	for _, want := range []string{"player.create", "admin.ban", "admin.exp.set", "admin.stats.reset"} {
		if !actions[want] {
			t.Fatalf("event %q missing (have %v, total=%d)", want, actions, total)
		}
	}

	bans, total, err := s.AdminEvents("admin.ban", 50, 0)
	if err != nil || len(bans) != 1 || total != 1 {
		t.Fatalf("filter by action = %d rows (total %d), err %v", len(bans), total, err)
	}
	if bans[0].Detail == "" || !strings.Contains(bans[0].Detail, "spam") {
		t.Fatalf("ban detail = %q, want reason recorded", bans[0].Detail)
	}
}

func TestSetPlayerCoins(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-coin", "c@d.e", "C", "")
	if err != nil {
		t.Fatal(err)
	}

	p, err := s.SetPlayerCoins("admin:x", gp.ID, 500)
	if err != nil {
		t.Fatal(err)
	}
	if p.TotalCoins != 500 {
		t.Fatalf("after set: coins=%d, want 500", p.TotalCoins)
	}

	// overwrite with a smaller balance, then with a negative one
	p, err = s.SetPlayerCoins("admin:x", gp.ID, 120)
	if err != nil {
		t.Fatal(err)
	}
	if p.TotalCoins != 120 {
		t.Fatalf("after overwrite: coins=%d, want 120", p.TotalCoins)
	}
	p, err = s.SetPlayerCoins("admin:x", gp.ID, -1)
	if err != nil {
		t.Fatal(err)
	}
	if p.TotalCoins != 0 {
		t.Fatalf("coins should clamp at 0, got %d", p.TotalCoins)
	}

	if _, err = s.SetPlayerCoins("admin:x", "missing", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown player err = %v, want ErrNotFound", err)
	}

	events, total, err := s.AdminEvents("admin.coins.set", 10, 0)
	if err != nil || total != 3 || len(events) != 3 {
		t.Fatalf("coins events = %d (total %d), err %v", len(events), total, err)
	}
	overwritten := false
	for _, e := range events {
		overwritten = overwritten ||
			(strings.Contains(e.Detail, `"after":120`) && strings.Contains(e.Detail, `"before":500`))
	}
	if !overwritten {
		t.Fatalf("coins events = %v, want the 500→120 overwrite", events)
	}
}

func TestGrantRevokeAchievement(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-ach", "a@b.cd", "A", "")
	if err != nil {
		t.Fatal(err)
	}

	// grant pays the entry's standard rewards (solve-25: 40 EXP / 25 coins)
	p, err := s.GrantAchievement("admin:x", gp.ID, "solve-25")
	if err != nil {
		t.Fatal(err)
	}
	if p.TotalExp != 40 || p.TotalCoins != 25 {
		t.Fatalf("after grant: exp=%d coins=%d, want 40/25", p.TotalExp, p.TotalCoins)
	}
	if _, err = s.GrantAchievement("admin:x", gp.ID, "solve-25"); !errors.Is(err, ErrAlreadyUnlocked) {
		t.Fatalf("double grant err = %v, want ErrAlreadyUnlocked", err)
	}
	if _, err = s.GrantAchievement("admin:x", gp.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown achievement err = %v, want ErrNotFound", err)
	}
	if _, err = s.GrantAchievement("admin:x", "missing", "solve-25"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown player err = %v, want ErrNotFound", err)
	}

	unlocked, err := s.PlayerAchievements(gp.ID)
	if err != nil || len(unlocked) != 1 || unlocked["solve-25"] == "" {
		t.Fatalf("unlocked = %v, err %v", unlocked, err)
	}

	// revoke removes the unlock but keeps banked rewards
	p, err = s.RevokeAchievement("admin:x", gp.ID, "solve-25")
	if err != nil {
		t.Fatal(err)
	}
	if p.TotalExp != 40 || p.TotalCoins != 25 {
		t.Fatalf("revoke must not claw back rewards: exp=%d coins=%d", p.TotalExp, p.TotalCoins)
	}
	if unlocked, _ = s.PlayerAchievements(gp.ID); len(unlocked) != 0 {
		t.Fatalf("unlocked after revoke = %v", unlocked)
	}
	if _, err = s.RevokeAchievement("admin:x", gp.ID, "solve-25"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double revoke err = %v, want ErrNotFound", err)
	}

	_, gtotal, err := s.AdminEvents("admin.achievement.grant", 10, 0)
	_, rtotal, rerr := s.AdminEvents("admin.achievement.revoke", 10, 0)
	if err != nil || rerr != nil || gtotal != 1 || rtotal != 1 {
		t.Fatalf("achievement events: grant=%d revoke=%d err=%v/%v", gtotal, rtotal, err, rerr)
	}
}

func TestSetPlayerTier(t *testing.T) {
	s := openTest(t)
	gp, _, err := s.GoogleLogin("sub-tier", "t@i.e", "T", "")
	if err != nil {
		t.Fatal(err)
	}

	// an override lands on the player and wins over the level-derived tier
	p, err := s.SetPlayerTier("admin:x", gp.ID, "master")
	if err != nil {
		t.Fatal(err)
	}
	if p.Tier != "master" || p.EffectiveTier() != 5 {
		t.Fatalf("after override: tier=%q effective=%d", p.Tier, p.EffectiveTier())
	}

	// clearing falls back to the derived tier (level 1 → bronze)
	p, err = s.SetPlayerTier("admin:x", gp.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Tier != "" || p.EffectiveTier() != 0 {
		t.Fatalf("after clear: tier=%q effective=%d", p.Tier, p.EffectiveTier())
	}

	if _, err = s.SetPlayerTier("admin:x", "missing", "gold"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown player err = %v, want ErrNotFound", err)
	}

	events, total, err := s.AdminEvents("admin.tier.set", 10, 0)
	if err != nil || total != 2 || len(events) != 2 {
		t.Fatalf("tier events = %d (total %d), err %v", len(events), total, err)
	}
}

func TestOverviewCounters(t *testing.T) {
	s := openTest(t)
	if _, _, err := s.CreateGuest("g1"); err != nil {
		t.Fatal(err)
	}
	gp, _, err := s.GoogleLogin("sub-ov", "o@p.q", "O", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 40, true); err != nil {
		t.Fatal(err)
	}
	r, err := s.CreateRound(gp.ID, "queen", []int{1, 2, 3, 4}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishRound(r.ID, "solved", 40, 5000); err != nil {
		t.Fatal(err)
	}

	o, err := s.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if o.Players["total"] != 2 || o.Players["guests"] != 1 || o.Players["google"] != 1 || o.Players["banned"] != 0 {
		t.Fatalf("players counters = %v", o.Players)
	}
	if o.RoundTotal != 1 || o.Rounds["solved"] != 1 {
		t.Fatalf("round counters = %v (total %d)", o.Rounds, o.RoundTotal)
	}
	if o.TotalExp != 40 {
		t.Fatalf("total exp = %d", o.TotalExp)
	}
	if len(o.Modes) != 1 || o.Modes[0].Mode != "queen" || o.Modes[0].HandsSolved != 1 {
		t.Fatalf("modes = %+v", o.Modes)
	}
}

func TestAdminRoundsListing(t *testing.T) {
	s := openTest(t)
	g1, _, err := s.CreateGuest("r1")
	if err != nil {
		t.Fatal(err)
	}
	g2, _, err := s.CreateGuest("r2")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []Player{g1, g2} {
		r, err := s.CreateRound(p.ID, "queen", []int{1, 2, 3, 4}, "")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.FinishRound(r.ID, "solved", 30, 4000); err != nil {
			t.Fatal(err)
		}
	}

	rows, total, err := s.AdminListRounds("", "", 50, 0)
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("rounds = %d (total %d), err %v", len(rows), total, err)
	}
	if rows[0].Nickname == "" || rows[0].FinishedAt == nil {
		t.Fatalf("round row = %+v", rows[0])
	}

	rows, total, err = s.AdminListRounds("queen", "open", 50, 0)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("filtered rounds = %d (total %d), err %v", len(rows), total, err)
	}

	mine, total, err := s.AdminPlayerRounds(g1.ID, 50, 0)
	if err != nil || total != 1 || len(mine) != 1 || mine[0].PlayerID != g1.ID {
		t.Fatalf("player rounds = %+v (total %d), err %v", mine, total, err)
	}
}

func TestBackupAndReset(t *testing.T) {
	s := openTest(t)
	if _, _, err := s.CreateGuest("g"); err != nil {
		t.Fatal(err)
	}
	gp, tok, err := s.GoogleLogin("sub-rst", "r@s.t", "R", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 90, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRound(gp.ID, "queen", []int{1, 2, 3, 4}, ""); err != nil {
		t.Fatal(err)
	}

	// backup: standalone snapshot that opens separately with the same data
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err := s.BackupTo(backup); err != nil {
		t.Fatal(err)
	}
	if err := s.BackupTo(backup); err == nil {
		t.Fatal("backup over an existing file should fail")
	}
	cr, err := crypto.New(testKey)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Open(backup, cr)
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	var snapPlayers int
	snap.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&snapPlayers)
	if snapPlayers != 2 {
		t.Fatalf("backup player count = %d, want 2", snapPlayers)
	}

	// reset: everything wiped, schema recreated, event logged, tokens dead
	rst, err := s.Reset("admin:x", filepath.Join(t.TempDir(), "pre-reset.db"))
	if err != nil {
		t.Fatal(err)
	}
	if rst.Players != 2 || rst.Rounds != 1 || rst.Events < 1 {
		t.Fatalf("reset stats = %+v", rst)
	}
	if _, err := s.PlayerByToken(tok); err == nil {
		t.Fatal("old token still works after reset")
	}
	var players, stats, rounds, events int
	s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&players)
	s.db.QueryRow(`SELECT COUNT(*) FROM player_mode_stats`).Scan(&stats)
	s.db.QueryRow(`SELECT COUNT(*) FROM rounds`).Scan(&rounds)
	s.db.QueryRow(`SELECT COUNT(*) FROM event_logs`).Scan(&events)
	if players != 0 || stats != 0 || rounds != 0 {
		t.Fatalf("tables not empty: players=%d stats=%d rounds=%d", players, stats, rounds)
	}
	ev, _, err := s.AdminEvents("", 10, 0)
	if err != nil || len(ev) != 1 || ev[0].Action != "db.reset" {
		t.Fatalf("fresh event log = %+v, err %v", ev, err)
	}
	if !strings.Contains(ev[0].Detail, `"actor":"admin:x"`) {
		t.Fatalf("db.reset detail missing actor: %s", ev[0].Detail)
	}

	// schema is fully usable again
	p2, _, err := s.CreateGuest("post-reset")
	if err != nil || p2.Nickname != "post-reset" {
		t.Fatalf("create after reset: %+v, %v", p2, err)
	}
}

func TestRestoreFrom(t *testing.T) {
	s := openTest(t)
	dir := t.TempDir()
	if _, _, err := s.CreateGuest("g1"); err != nil {
		t.Fatal(err)
	}
	gp, tok, err := s.GoogleLogin("sub-restore", "r@s.t", "R", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 90, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRound(gp.ID, "queen", []int{1, 2, 3, 4}, ""); err != nil {
		t.Fatal(err)
	}

	// the snapshot: two players, one round, 90 EXP on the google player
	backup := filepath.Join(dir, "game24-backup-20990101-000000.db")
	if err := s.BackupTo(backup); err != nil {
		t.Fatal(err)
	}

	// diverge after the snapshot: a new guest, more EXP
	_, deadTok, err := s.CreateGuest("g2")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.AwardEXP(gp.ID, "queen", 50, true); err != nil {
		t.Fatal(err)
	}

	// garbage files and snapshots without the game schema are refused, live
	// data untouched
	bad := filepath.Join(dir, "bad.db")
	if err := os.WriteFile(bad, []byte("definitely not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RestoreFrom("admin:x", bad, filepath.Join(dir, "s1.db")); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("garbage file: err = %v, want ErrInvalidBackup", err)
	}
	bare := filepath.Join(dir, "bare.db")
	bd, err := sql.Open("sqlite", "file:"+bare)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bd.Exec(`CREATE TABLE alien (x)`); err != nil {
		t.Fatal(err)
	}
	bd.Close()
	if _, err := s.RestoreFrom("admin:x", bare, filepath.Join(dir, "s2.db")); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("schema-less sqlite: err = %v, want ErrInvalidBackup", err)
	}

	// a snapshot written under a different ENCRYPTION_KEY cannot be restored
	cr2, err := crypto.New(strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	foreignStore, err := Open(filepath.Join(dir, "foreign.db"), cr2)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := foreignStore.CreateGuest("someone"); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "game24-backup-20990102-000000.db")
	if err := foreignStore.BackupTo(foreign); err != nil {
		t.Fatal(err)
	}
	foreignStore.Close()
	if _, err := s.RestoreFrom("admin:x", foreign, filepath.Join(dir, "s3.db")); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("foreign-key snapshot: err = %v, want ErrInvalidBackup", err)
	}
	var live int
	s.db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&live)
	if live != 3 {
		t.Fatalf("failed restores changed live data: players = %d, want 3", live)
	}

	// happy path: the snapshot state comes back, the divergence disappears
	st, err := s.RestoreFrom("admin:x", backup, filepath.Join(dir, "safety.db"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Players != 2 || st.Rounds != 1 {
		t.Fatalf("restore stats = %+v, want 2 players / 1 round", st)
	}
	p, err := s.PlayerByID(gp.ID)
	if err != nil || p.TotalExp != 90 {
		t.Fatalf("restored exp = %+v (%v), want 90", p, err)
	}
	if _, err := s.PlayerByToken(tok); err != nil {
		t.Fatalf("session recorded inside the snapshot should live again: %v", err)
	}
	if _, err := s.PlayerByToken(deadTok); err == nil {
		t.Fatal("session minted after the snapshot should be dead")
	}
	if _, err := os.Stat(filepath.Join(dir, "safety.db")); err != nil {
		t.Fatalf("no pre-restore safety snapshot kept: %v", err)
	}
	// the restored log carries the audit entry for the restore itself
	ev, _, err := s.AdminEvents("db.restore", 10, 0)
	if err != nil || len(ev) != 1 || !strings.Contains(ev[0].Detail, `"actor":"admin:x"`) {
		t.Fatalf("db.restore event = %+v, err %v", ev, err)
	}
	// the restored database is fully usable
	if _, _, err := s.CreateGuest("post-restore"); err != nil {
		t.Fatalf("create after restore: %v", err)
	}
}
