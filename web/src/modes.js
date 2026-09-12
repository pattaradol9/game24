// Difficulty metadata shared by the picker, the in-game badge and the room
// dialog — one place so a mode always shows the same suit and multiplier.
export const MODES = [
  { id: 'jack', suit: 'diamond', stars: 1, mult: 1 },
  { id: 'queen', suit: 'club', stars: 2, mult: 2 },
  { id: 'king', suit: 'spade', stars: 3, mult: 3 },
  { id: 'ace', suit: 'heart', stars: 5, mult: 5 },
]

export const modeMeta = (id) => MODES.find((m) => m.id === id) ?? MODES[1]

// --- solo difficulty ladder -------------------------------------------------
// One solo run never sits still: every LADDER_STEP hands dealt climbs a rung
// (the puzzle steps up one mode, ace caps the walk) and the countdown drops
// 10% off the run's base window until it bottoms out at LADDER_MIN_TIME —
// the hardest state the ladder reaches ("H": ace puzzles, 30s clock). The
// server owns the actual numbers; these helpers only place the display.
export const LADDER_STEP = 10
export const LADDER_MIN_TIME = 30

// the rung the n-th hand of a run plays (n is 1-based): hands 1–10 stay on
// stage 0, hand 11 climbs to stage 1, and so on
export const ladderStageFor = (handNo) =>
  Math.max(0, Math.floor(((handNo < 1 ? 1 : handNo) - 1) / LADDER_STEP))

// how far the countdown has shrunk vs the run's base window, as a whole
// percent (0 while no rung has been climbed; the 30s floor caps the figure)
export const ladderDropPct = (timeLimit, baseTime) => {
  if (!baseTime || baseTime <= 0 || !timeLimit || timeLimit >= baseTime) return 0
  return Math.round((1 - timeLimit / baseTime) * 100)
}
