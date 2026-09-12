package handler

import (
	"testing"

	"github.com/pattaradol9/game24/server/internal/store"
)

func TestRoundBaseWindow(t *testing.T) {
	if got := roundBaseWindow(store.Round{Mode: "queen"}); got != 90 {
		t.Fatalf("mode fallback = %d, want 90", got)
	}
	if got := roundBaseWindow(store.Round{Mode: "queen", TimeLimitSec: 33}); got != 33 {
		t.Fatalf("stamped window = %d, want 33", got)
	}
}

// A solo run escalates through the create-round endpoint: the stage body
// field picks the ladder rung, which resolves the puzzle mode and the shrunk
// countdown, and the round row stamps the window its whole lifecycle (expiry
// fence, payout, extension echo) is judged against.
func TestCreateRoundLadder(t *testing.T) {
	api, mux := newTestAPI(t)
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Climber"}, "")
	token := data(res)["token"].(string)
	playerID := data(res)["player"].(map[string]any)["id"].(string)

	create := func(stage int) map[string]any {
		code, res := post(t, mux, "/api/v1/rounds", map[string]any{"mode": "jack", "stage": stage}, token)
		if code != 200 {
			t.Fatalf("create round stage %d: %d %v", stage, code, res)
		}
		return data(res)
	}

	// stage 0 is the run's starting mode at its own base window
	d := create(0)
	if d["mode"] != "jack" || int(d["timeLimit"].(float64)) != 120 || int(d["stage"].(float64)) != 0 {
		t.Fatalf("stage 0 = %v", d)
	}

	// hand 11 (stage 1): queen puzzles on a 10%-shorter clock
	d = create(1)
	if d["mode"] != "queen" || int(d["timeLimit"].(float64)) != 108 {
		t.Fatalf("stage 1 = %v", d)
	}
	if mult := int(d["multiplier"].(float64)); mult != 2 {
		t.Fatalf("stage 1 multiplier = %d, want queen's 2", mult)
	}

	// a bogus stage clamps to the hardest state: ace puzzles, 30s floor
	d = create(999)
	if d["mode"] != "ace" || int(d["timeLimit"].(float64)) != 30 {
		t.Fatalf("huge stage = %v", d)
	}

	// the stage-shrunk window is stamped on the stored round
	roundID := d["roundId"].(string)
	round, err := api.Store.RoundByID(roundID, playerID)
	if err != nil {
		t.Fatalf("round by id: %v", err)
	}
	if round.Mode != "ace" || round.TimeLimitSec != 30 {
		t.Fatalf("stored round = mode %s window %d, want ace/30", round.Mode, round.TimeLimitSec)
	}

	// negative stages land on stage 0 instead of erroring
	d = create(-5)
	if d["mode"] != "jack" || int(d["stage"].(float64)) != 0 {
		t.Fatalf("negative stage = %v", d)
	}

	// an unknown starting mode is still rejected
	if code, _ := post(t, mux, "/api/v1/rounds", map[string]any{"mode": "joker", "stage": 0}, token); code != 400 {
		t.Fatal("unknown mode must 400")
	}
}
