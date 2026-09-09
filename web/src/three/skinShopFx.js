// skinShopFx — the card-skin shop ambience ("the atelier"): ghostly playing
// cards tumbling slowly down a curtain of gold glitter, over breathing
// bokeh blobs, scheduled glint flares and a slow light sweep — ONE canvas
// behind the skin grid.
//
// Architecture mirrors itemFx/skinScenes: the pure layout/integration
// helpers at the top are exported for node tests, the scene builder is a
// plain function over the shared `api` ({ THREE, back, owned, rand, tex,
// w0, pxScale, onScale }) and returns { update(t, dt, w) }. This module must
// stay free of top-level three imports.

import { glintCycler, gpuCloud, sprite } from './skinScenes.js'
import { sweep } from './itemFx.js'

/* ============================================================ pure helpers */

/** Card-face palettes: one champagne, one pale blue, one rose (soft, warm). */
export const CARD_TINTS = ['#f3d98b', '#aebfec', '#e8b7cd']

/**
 * Spec for a tumbling ghost card: a base column it sways around, a fall
 * speed, three spin axes and a per-card scale/opacity. Pure — nothing here
 * touches three or the DOM.
 */
export function cardField(count, w0, depth, rng = Math.random) {
  const out = []
  for (let i = 0; i < count; i++) {
    out.push({
      baseX: (rng() * 2 - 1) * (w0.x + 0.4),
      y: (rng() * 2 - 1) * (w0.y + 0.6),
      z: -depth * (0.55 + rng() * 0.45), // ghost cards live in the far half
      vy: 0.1 + rng() * 0.2, // world units per second, always falling
      sway: 0.3 + rng() * 0.7,
      swayF: 0.2 + rng() * 0.5,
      phase: rng() * Math.PI * 2,
      rx: rng() * Math.PI * 2,
      ry: rng() * Math.PI * 2,
      rz: (rng() * 2 - 1) * 0.35,
      vrx: (rng() * 2 - 1) * 0.22,
      vry: (rng() * 2 - 1) * 0.34,
      vrz: (rng() * 2 - 1) * 0.1,
      scale: 0.65 + rng() * 0.85,
      opacity: 0.1 + rng() * 0.13,
      tint: Math.floor(rng() * CARD_TINTS.length),
    })
  }
  return out
}

/**
 * Advance the card field by `dt` seconds at scene time `elapsed`: every card
 * falls, sways around its base column and tumbles on its three axes; a card
 * that drops past the bottom wrap back to the ceiling. Mutates the specs.
 */
export function advanceCards(cards, dt, elapsed, w0) {
  const floor = -w0.y - 0.8
  const ceiling = w0.y + 0.8
  for (const c of cards) {
    c.y -= c.vy * dt
    if (c.y < floor) c.y = ceiling
    c.x = c.baseX + Math.sin(elapsed * c.swayF + c.phase) * c.sway
    c.rx += c.vrx * dt
    c.ry += c.vry * dt
    c.rz += c.vrz * dt
  }
  return cards
}

/* ================================================================ textures */

/** A soft-lit card face ("24", of course) drawn once per tint. Caller owns
 *  disposal. Painted mostly white so the material colour tints it cleanly. */
export function makeCardTexture(THREE, tint) {
  const cv = document.createElement('canvas')
  cv.width = 96
  cv.height = 134
  const c = cv.getContext('2d')
  const rr = (x, y, w, h, r) => {
    c.beginPath()
    c.moveTo(x + r, y)
    c.arcTo(x + w, y, x + w, y + h, r)
    c.arcTo(x + w, y + h, x, y + h, r)
    c.arcTo(x, y + h, x, y, r)
    c.arcTo(x, y, x + w, y, r)
    c.closePath()
  }
  rr(3, 3, 90, 128, 10)
  c.fillStyle = '#f7f4ec'
  c.fill()
  c.lineWidth = 3
  c.strokeStyle = tint
  c.stroke()
  c.lineWidth = 1.5
  rr(10, 10, 76, 114, 6)
  c.stroke()
  c.fillStyle = '#404a63'
  c.font = '600 44px ui-sans-serif, system-ui, sans-serif'
  c.textAlign = 'center'
  c.textBaseline = 'middle'
  c.fillText('24', 48, 70)
  const tex = new THREE.CanvasTexture(cv)
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}

/* ================================================================ scenes */

/**
 * The atelier scene: falling ghost cards over a gold glitter curtain, with
 * three breathing bokeh blobs (gold / blue / violet — a wink at the rarity
 * ladder), scheduled star flares and a slow champagne sheen sweep.
 */
export function createSkinShopScene(api) {
  const { THREE, back, owned, tex, w0 } = api

  const glitter = gpuCloud(api, {
    count: 180,
    colors: [0xffe08a, 0xffd166, 0xfff3c4, 0xffffff],
    size: [0.022, 0.055],
    speed: [-0.32, -0.08], // sifting down, like loose glitter
    sway: [0.04, 0.16],
    twF: [1.2, 3],
    opacity: 0.85,
  })
  const halo = gpuCloud(api, {
    count: 64,
    colors: [0xf3d98b, 0xfff0c9],
    size: [0.045, 0.08],
    spin: 0.1,
    bound: 1000,
    opacity: 0.65,
    layout: (() => {
      // a wide, low ring at the pane's feet
      const pts = new Float32Array(64 * 2)
      for (let i = 0; i < 64; i++) {
        const a = (i / 64) * Math.PI * 2
        pts[i * 2] = Math.cos(a) * Math.min(3.4, w0.x * 0.62)
        pts[i * 2 + 1] = Math.sin(a) * Math.min(1.7, w0.y * 0.34) - w0.y * 0.34
      }
      return pts
    })(),
  })
  const glints = glintCycler(api, { n: 3, color: 0xfff3c4, scale: 0.8, period: 8 })

  // breathing rarity bokeh: gold, pale blue, violet
  const bokeh = [
    sprite(api, { tex: tex.puff, color: 0xf3d98b, scale: 7, scaleY: 4.4, pos: [-w0.x * 0.44, w0.y * 0.42, -3], opacity: 0.2 }),
    sprite(api, { tex: tex.puff, color: 0x8fb2f5, scale: 6, scaleY: 3.8, pos: [w0.x * 0.46, w0.y * 0.1, -3.2], opacity: 0.14 }),
    sprite(api, { tex: tex.puff, color: 0xb283f0, scale: 6.4, scaleY: 4, pos: [w0.x * 0.1, -w0.y * 0.5, -3.4], opacity: 0.15 }),
  ]

  // the ghost cards themselves
  const specs = cardField(11, w0, 4)
  const cardGeo = new THREE.PlaneGeometry(0.5, 0.7)
  owned.push(cardGeo)
  const tintTex = CARD_TINTS.map((tint) => {
    const t = makeCardTexture(THREE, tint)
    owned.push(t)
    return t
  })
  const cards = specs.map((spec) => {
    const mat = new THREE.MeshBasicMaterial({
      map: tintTex[spec.tint],
      transparent: true,
      opacity: spec.opacity,
      depthWrite: false,
      side: THREE.DoubleSide,
    })
    owned.push(mat)
    const mesh = new THREE.Mesh(cardGeo, mat)
    back.add(mesh)
    return { spec, mesh }
  })

  // metallic champagne sheen sweeping across every few seconds
  const sheenMat = new THREE.MeshBasicMaterial({
    map: tex.streak, color: 0xfff3c4, transparent: true, opacity: 0,
    depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
  })
  const sheen = new THREE.Mesh(new THREE.PlaneGeometry(2.4, 9), sheenMat)
  sheen.rotation.z = -0.5
  back.add(sheen)
  owned.push(sheen.geometry, sheenMat)

  return {
    update(t, dt, w) {
      glitter.tick(t)
      halo.tick(t)
      glints.tick(t)
      advanceCards(specs, dt, t, w)
      for (const { spec, mesh } of cards) {
        mesh.position.set(spec.x, spec.y, spec.z)
        mesh.rotation.set(spec.rx, spec.ry, spec.rz)
        mesh.scale.setScalar(spec.scale)
        mesh.material.opacity = spec.opacity
      }
      bokeh[0].mat.opacity = 0.16 + Math.sin(t * 0.45) * 0.06
      bokeh[1].mat.opacity = 0.11 + Math.sin(t * 0.33 + 2) * 0.045
      bokeh[2].mat.opacity = 0.12 + Math.sin(t * 0.39 + 4) * 0.05
      const s = sweep(t, { period: 9, duty: 0.2, span: w.x * 1.4 })
      if (s) {
        sheen.visible = true
        sheen.position.x = s.x
        sheenMat.opacity = 0.18 * s.alpha
      } else {
        sheen.visible = false
      }
    },
  }
}
