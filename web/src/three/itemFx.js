// itemFx — the boost-item shop ambience ("the vault"): rising gold and amber
// motes over a slowly spinning halo ring, with breathing corner glows and a
// scheduled light sweep, rendered on ONE canvas behind the item cards.
//
// Architecture mirrors skinScenes: the pure layout/curve helpers at the top
// are exported for node tests, the scene builder is a plain function over the
// shared `api` ({ THREE, back, owned, rand, tex, w0, pxScale, onScale }) and
// returns { update(t, dt, w) }. This module must stay free of top-level
// three imports.

import { circleLayout, glintCycler, gpuCloud, sprite } from './skinScenes.js'

/* ============================================================ pure helpers */

/**
 * Half-extent (world units) of the halo ring so it always fits the pane:
 * it hugs the narrower visible axis and never grows past a graceful cap,
 * however wide the shop grid gets.
 */
export function haloRadius(w0, k = 0.5) {
  return Math.min(3.2, Math.min(w0.x, w0.y) * k)
}

/**
 * Light sweep across the pane: runs for the first `duty` fraction of each
 * `period` seconds, gliding from -span to +span with a sine alpha window.
 * Returns null while the sweep is off (the rest of the cycle).
 */
export function sweep(t, { period = 8, duty = 0.22, span = 1.4 } = {}) {
  const c = (t % period) / period
  if (c >= duty) return null
  const k = c / duty
  return { k, x: -span + k * span * 2, alpha: Math.sin(k * Math.PI) }
}

/** Idle breathing envelope: base ± amp, oscillating at `freq` Hz. */
export function breath(t, freq, base, amp) {
  return base + Math.sin(t * freq) * amp
}

/* ================================================================ scenes */

/**
 * The vault scene: two GPU mote families (EXP gold rising, coin amber
 * sinking), a spinning halo ring, three scheduled glint flares, breathing
 * corner glows and a slow diagonal sheen sweep.
 */
export function createItemScene(api) {
  const { THREE, back, owned, tex, w0 } = api

  // warm rising sparks — the shop's main weather
  const motes = gpuCloud(api, {
    count: 150,
    colors: [0xffd166, 0xffe9a3, 0xffcf6b, 0xf6b73c],
    size: [0.045, 0.1],
    speed: [0.22, 0.85],
    sway: [0.15, 0.5],
    twF: [0.8, 2.2],
    opacity: 0.9,
  })
  // near-static fine dust for depth
  const dust = gpuCloud(api, {
    count: 90,
    colors: [0xfff3c4, 0xffffff],
    size: [0.02, 0.042],
    speed: [-0.05, 0.08],
    sway: [0.08, 0.2],
    opacity: 0.5,
  })
  // slowly orbiting halo ring centred behind the grid
  const halo = gpuCloud(api, {
    count: 72,
    colors: [0xf3d98b, 0xffe9a8],
    size: [0.05, 0.09],
    spin: 0.16,
    bound: 1000,
    opacity: 0.8,
    layout: circleLayout(72, haloRadius(w0)),
  })
  const glints = glintCycler(api, { n: 3, color: 0xffe08a, scale: 0.85, period: 9 })
  const glowA = sprite(api, { tex: tex.puff, color: 0xf6b73c, scale: 7, scaleY: 4.2, pos: [-w0.x * 0.42, w0.y * 0.42, -3], opacity: 0.2 })
  const glowB = sprite(api, { tex: tex.puff, color: 0xeda32c, scale: 6, scaleY: 3.6, pos: [w0.x * 0.45, -w0.y * 0.45, -3.2], opacity: 0.16 })

  // metallic sheen sweeping across every few seconds (gold-skin motif)
  const sheenMat = new THREE.MeshBasicMaterial({
    map: tex.streak, color: 0xfff3c4, transparent: true, opacity: 0,
    depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
  })
  const sheen = new THREE.Mesh(new THREE.PlaneGeometry(2.6, 9), sheenMat)
  sheen.rotation.z = -0.55
  back.add(sheen)
  owned.push(sheen.geometry, sheenMat)

  return {
    update(t, dt, w) {
      motes.tick(t)
      dust.tick(t)
      halo.tick(t)
      glints.tick(t)
      glowA.mat.opacity = breath(t, 0.5, 0.17, 0.06)
      glowB.mat.opacity = breath(t, 0.38, 0.13, 0.05)
      const s = sweep(t, { period: 8, duty: 0.22, span: w.x * 1.4 })
      if (s) {
        sheen.visible = true
        sheen.position.x = s.x
        sheenMat.opacity = 0.2 * s.alpha
      } else {
        sheen.visible = false
      }
    },
  }
}
