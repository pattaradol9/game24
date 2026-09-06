// Difficulty metadata shared by the picker, the in-game badge and the room
// dialog — one place so a mode always shows the same suit and multiplier.
export const MODES = [
  { id: 'jack', suit: 'diamond', stars: 1, mult: 1 },
  { id: 'queen', suit: 'club', stars: 2, mult: 2 },
  { id: 'king', suit: 'spade', stars: 3, mult: 3 },
  { id: 'ace', suit: 'heart', stars: 5, mult: 5 },
]

export const modeMeta = (id) => MODES.find((m) => m.id === id) ?? MODES[1]
