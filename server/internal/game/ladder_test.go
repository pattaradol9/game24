package game

import "testing"

func TestLadderStageFor(t *testing.T) {
	cases := []struct {
		handNo, want int
	}{
		{0, 0}, {1, 0}, {10, 0}, {11, 1}, {20, 1}, {21, 2}, {141, 14},
	}
	for _, tc := range cases {
		if got := LadderStageFor(tc.handNo); got != tc.want {
			t.Fatalf("LadderStageFor(%d) = %d, want %d", tc.handNo, got, tc.want)
		}
	}
}

// The ladder curve is pinned end to end: the countdown drops 10% per rung
// off the run's base window and floors at LadderMinTimeSec, while the puzzle
// mode walks one mode per rung and pins at ace.
func TestLadderConfig(t *testing.T) {
	t.Run("jack run climbs to the 30s floor", func(t *testing.T) {
		wantTimes := []int{120, 108, 97, 87, 79, 71, 64, 57, 52, 46, 42, 38, 34, 31, 30, 30}
		wantModes := []Mode{Jack, Queen, King, Ace, Ace, Ace, Ace, Ace, Ace, Ace, Ace, Ace, Ace, Ace, Ace, Ace}
		for stage, want := range wantTimes {
			mode, cfg, err := LadderConfig(Jack, stage)
			if err != nil {
				t.Fatalf("stage %d: %v", stage, err)
			}
			if mode != wantModes[stage] {
				t.Fatalf("stage %d mode = %s, want %s", stage, mode, wantModes[stage])
			}
			if cfg.TimeLimitSec != want {
				t.Fatalf("stage %d time = %d, want %d", stage, cfg.TimeLimitSec, want)
			}
			if cfg.TimeLimitSec < LadderMinTimeSec {
				t.Fatalf("stage %d broke the %ds floor: %d", stage, LadderMinTimeSec, cfg.TimeLimitSec)
			}
		}
		if cfg := MustConfig(Ace); cfg.TimeLimitSec != 60 {
			t.Fatalf("ladder mutated the ace config: %d", cfg.TimeLimitSec)
		}
	})
	t.Run("every start reaches the floor without going under it", func(t *testing.T) {
		for _, start := range AllModes() {
			base := MustConfig(start).TimeLimitSec
			for stage := 0; stage <= LadderMaxStage; stage++ {
				_, cfg, err := LadderConfig(start, stage)
				if err != nil {
					t.Fatalf("%s stage %d: %v", start, stage, err)
				}
				if cfg.TimeLimitSec < LadderMinTimeSec {
					t.Fatalf("%s stage %d under the floor: %d", start, stage, cfg.TimeLimitSec)
				}
				if cfg.TimeLimitSec > base {
					t.Fatalf("%s stage %d above its base: %d > %d", start, stage, cfg.TimeLimitSec, base)
				}
				if stage >= 3 {
					if mode, _, _ := LadderConfig(start, stage); mode != Ace {
						t.Fatalf("%s stage %d mode = %s, want ace cap", start, stage, mode)
					}
				}
			}
		}
	})
	t.Run("multiplier follows the resolved mode", func(t *testing.T) {
		if _, cfg, _ := LadderConfig(Jack, 1); cfg.Multiplier != 2 {
			t.Fatalf("jack run stage 1 (queen) multiplier = %d, want 2", cfg.Multiplier)
		}
		if _, cfg, _ := LadderConfig(Queen, 1); cfg.Multiplier != 3 {
			t.Fatalf("queen run stage 1 (king) multiplier = %d, want 3", cfg.Multiplier)
		}
	})
	t.Run("clamps and rejects", func(t *testing.T) {
		if mode, cfg, _ := LadderConfig(Jack, 999); mode != Ace || cfg.TimeLimitSec != LadderMinTimeSec {
			t.Fatalf("huge stage = %s @%ds, want ace @%d", mode, cfg.TimeLimitSec, LadderMinTimeSec)
		}
		if _, _, err := LadderConfig(Mode("joker"), 0); err == nil {
			t.Fatal("unknown start mode must error")
		}
	})
}
