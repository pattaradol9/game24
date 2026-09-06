// Synthesized mobile-game SFX — WebAudio only, no asset files.
// Melodic merge sounds pitch up with the combo counter, the timer gets a
// heartbeat, and celebrations get a drumroll + fanfare.
let ctx = null
let master = null
let enabled = localStorage.getItem('g24_sound') !== 'off'

function ac() {
  ctx ??= new (window.AudioContext || window.webkitAudioContext)()
  if (ctx.state === 'suspended') ctx.resume()
  return ctx
}

function out() {
  // route everything through one master gain so muting is instant
  if (!master) {
    master = ac().createGain()
    master.gain.value = 0.9
    master.connect(ctx.destination)
  }
  return master
}

function tone(freq, dur = 0.08, type = 'square', gain = 0.04, when = 0, slideTo = null) {
  if (!enabled) return
  try {
    const c = ac()
    const osc = c.createOscillator()
    const vol = c.createGain()
    osc.type = type
    osc.frequency.setValueAtTime(freq, c.currentTime + when)
    if (slideTo) osc.frequency.exponentialRampToValueAtTime(slideTo, c.currentTime + when + dur)
    vol.gain.setValueAtTime(gain, c.currentTime + when)
    vol.gain.exponentialRampToValueAtTime(0.0001, c.currentTime + when + dur)
    osc.connect(vol).connect(out())
    osc.start(c.currentTime + when)
    osc.stop(c.currentTime + when + dur)
  } catch { /* audio unavailable */ }
}

function noise(dur = 0.12, gain = 0.03, when = 0, hp = 900) {
  if (!enabled) return
  try {
    const c = ac()
    const len = Math.max(1, (dur * c.sampleRate) | 0)
    const buf = c.createBuffer(1, len, c.sampleRate)
    const data = buf.getChannelData(0)
    for (let i = 0; i < len; i++) data[i] = Math.random() * 2 - 1
    const src = c.createBufferSource()
    src.buffer = buf
    const filter = c.createBiquadFilter()
    filter.type = 'highpass'
    filter.frequency.value = hp
    const vol = c.createGain()
    vol.gain.setValueAtTime(gain, c.currentTime + when)
    vol.gain.exponentialRampToValueAtTime(0.0001, c.currentTime + when + dur)
    src.connect(filter).connect(vol).connect(out())
    src.start(c.currentTime + when)
    src.stop(c.currentTime + when + dur)
  } catch { /* audio unavailable */ }
}

// pentatonic ladder so consecutive merges always sound pleasant
const COMBO_SCALE = [523.25, 587.33, 659.25, 783.99, 880, 1046.5, 1174.66, 1318.51]

export const sfx = {
  click: () => { tone(520, 0.05, 'square', 0.03); noise(0.03, 0.012, 0, 2500) },
  pop: () => tone(340, 0.07, 'triangle', 0.05, 0, 560),
  cardSlide: () => noise(0.09, 0.025, 0, 1600),
  deal: (i = 0) => { noise(0.06, 0.02, i * 0.09, 2000); tone(300 + i * 60, 0.05, 'triangle', 0.02, i * 0.09) },
  merge: (combo = 0) => {
    const f = COMBO_SCALE[Math.min(combo, COMBO_SCALE.length - 1)]
    tone(f, 0.1, 'triangle', 0.06)
    tone(f * 2, 0.07, 'sine', 0.03, 0.02)
    noise(0.05, 0.015, 0, 2200)
  },
  select: () => tone(660, 0.04, 'sine', 0.035),
  wrong: () => {
    tone(220, 0.16, 'sawtooth', 0.05, 0, 110)
    tone(207, 0.2, 'square', 0.03, 0.06, 104)
  },
  win: () => {
    const seq = [[523, 0], [659, 0.1], [784, 0.2], [1046, 0.3], [784, 0.42], [1046, 0.52]]
    seq.forEach(([f, w]) => tone(f, 0.16, 'square', 0.05, w))
    noise(0.4, 0.02, 0.3, 1200)
  },
  lose: () => {
    tone(330, 0.18, 'sawtooth', 0.04, 0, 250)
    tone(196, 0.3, 'sawtooth', 0.04, 0.15, 150)
  },
  tick: () => tone(880, 0.03, 'square', 0.02),
  count: (final = false) => {
    if (final) { tone(880, 0.22, 'square', 0.06); tone(1108, 0.3, 'square', 0.05, 0.05) }
    else tone(440, 0.1, 'square', 0.05)
  },
  heartbeat: () => {
    tone(70, 0.09, 'sine', 0.12)
    tone(62, 0.11, 'sine', 0.1, 0.16)
  },
  drumroll: () => {
    for (let i = 0; i < 10; i++) noise(0.045, 0.035, i * 0.07, 700)
  },
  star: (i = 0) => tone(700 + i * 180, 0.12, 'triangle', 0.055, 0, 900 + i * 180),
  levelUp: () => {
    [[440, 0], [554, 0.09], [659, 0.18], [880, 0.27]].forEach(([f, w]) => tone(f, 0.1, 'square', 0.05, w))
    tone(1760, 0.3, 'sine', 0.04, 0.36)
  },
  whoosh: () => noise(0.25, 0.03, 0, 600),
  boing: () => tone(180, 0.18, 'sine', 0.07, 0, 420),
}

export function soundEnabled() {
  return enabled
}

export function toggleSound() {
  enabled = !enabled
  localStorage.setItem('g24_sound', enabled ? 'on' : 'off')
  return enabled
}
