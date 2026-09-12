package game

import "math"

// The solo difficulty ladder: a run that starts at any mode does not stay
// there. Every LadderStepRounds hands dealt climbs one rung — the puzzle
// steps up one mode (ace caps the walk) and the countdown shrinks by 10% —
// until the countdown bottoms out at LadderMinTimeSec, the hardest state the
// ladder can reach ("H": hardest puzzles, minimum clock).
const (
	LadderStepRounds = 10 // hands dealt per rung
	LadderMinTimeSec = 30 // seconds; the countdown's floor
	LadderTimeKeep   = 0.9 // each rung keeps 90% of the run's base window
	LadderMaxStage   = 40  // sanity clamp on client-supplied stage numbers
)

// LadderStageFor reports the rung the n-th hand of a run plays (n is
// 1-based): hands 1–10 stay on stage 0, hand 11 climbs to stage 1, and so on.
func LadderStageFor(handNo int) int {
	if handNo < 1 {
		handNo = 1
	}
	return (handNo - 1) / LadderStepRounds
}

// LadderConfig resolves stage k of a run that started at start. The puzzle
// mode walks the mode ladder from start upward (one mode per rung, ace is
// the cap) while the countdown shrinks 10% per rung off the run's base
// window, never below LadderMinTimeSec. EXP/coin multiplier and the dealt
// number range always follow the resolved mode.
func LadderConfig(start Mode, stage int) (Mode, ModeConfig, error) {
	base, err := Config(start)
	if err != nil {
		return "", ModeConfig{}, err
	}
	if stage < 0 {
		stage = 0
	}
	if stage > LadderMaxStage {
		stage = LadderMaxStage
	}
	modes := AllModes()
	idx := 0
	for i, m := range modes {
		if m == start {
			idx = i
			break
		}
	}
	mode := modes[min(idx+stage, len(modes)-1)]
	cfg := MustConfig(mode)
	limit := int(math.Round(float64(base.TimeLimitSec) * math.Pow(LadderTimeKeep, float64(stage))))
	if limit < LadderMinTimeSec {
		limit = LadderMinTimeSec
	}
	cfg.TimeLimitSec = limit
	return mode, cfg, nil
}
