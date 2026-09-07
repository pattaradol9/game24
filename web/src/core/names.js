// Fun random nickname generator shared by the entry modal and the rename
// modal. Adjective + animal + a two-digit tail keeps names short, unique
// enough for a room, and always within the 24-char server cap.

const ADJECTIVES = [
  'Swift', 'Lucky', 'Cosmic', 'Turbo', 'Mighty', 'Clever', 'Sunny', 'Nimble',
  'Brave', 'Funky', 'Silent', 'Royal', 'Hyper', 'Cheeky', 'Golden', 'Witty',
]

const ANIMALS = [
  'Tiger', 'Panda', 'Falcon', 'Otter', 'Dragon', 'Koala', 'Phoenix', 'Wolf',
  'Rabbit', 'Shark', 'Pixel', 'Comet', 'Fox', 'Bear', 'Hawk', 'Nova',
]

const pick = (list) => list[Math.floor(Math.random() * list.length)]

export function randomName() {
  const tail = Math.floor(Math.random() * 90) + 10
  return `${pick(ADJECTIVES)}${pick(ANIMALS)}${tail}`
}
