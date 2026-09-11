// Standalone probe: drive a 2-seat room over the real websocket against the
// scratch server and dump every message around start/extend/regen/hint.
import { chromium } from '@playwright/test'
import { mintToken } from '../helpers/fake-google.mjs'

const API = 'http://127.0.0.1:18099/api/v1'

const browser = await chromium.launch()
const page = await browser.newPage()
await page.goto(API + '/healthz').catch(() => page.goto('about:blank'))

const result = await page.evaluate(async () => {
  const B = 'http://127.0.0.1:18099/api/v1'
  const post = (path, body, token) =>
    fetch(B + path, {
      method: 'POST',
      headers: { 'content-type': 'application/json', ...(token ? { authorization: 'Bearer ' + token } : {}) },
      body: JSON.stringify(body ?? {}),
    }).then((r) => r.json())

  const host = await post('/players', { nickname: 'probe-host' })
  const member = await post('/players', { nickname: 'probe-member' })
  const room = await post('/rooms', { mode: 'jack', rounds: 12, hintQuota: 3, extendQuota: 1, regenQuota: 2 }, host.data.token)
  const code = room.data.code

  const connect = (token, name) =>
    new Promise((resolve) => {
      const ws = new WebSocket('ws://127.0.0.1:18099/api/v1/ws/room/' + code)
      const log = []
      ws.onopen = () => ws.send(JSON.stringify({ type: 'join', data: { name, token } }))
      ws.onmessage = (ev) => {
        const m = JSON.parse(ev.data)
        log.push({ t: Date.now() % 100000, type: m.type, data: m.data })
        resolve({ ws, log })
      }
    })

  const h = await connect(host.data.token, 'probe-host')
  const m = await connect(member.data.token, 'probe-member')
  const dump = []
  const listen = (conn, tag) => conn.ws.addEventListener('message', (ev) => {
    const msg = JSON.parse(ev.data)
    dump.push(`${tag} ${msg.type} ${JSON.stringify(msg.data)?.slice(0, 90)}`)
  })
  listen(h, 'H'); listen(m, 'M')

  h.ws.send(JSON.stringify({ type: 'start', data: {} }))
  await new Promise((r) => setTimeout(r, 1500))

  m.ws.send(JSON.stringify({ type: 'extend', data: {} }))
  await new Promise((r) => setTimeout(r, 800))

  h.ws.send(JSON.stringify({ type: 'hint', data: {} }))
  await new Promise((r) => setTimeout(r, 500))

  m.ws.send(JSON.stringify({ type: 'regen', data: {} }))
  await new Promise((r) => setTimeout(r, 800))

  return { code, dump, hostLog: h.log.map((l) => l.type), memberLog: m.log.map((l) => l.type) }
})

console.log(JSON.stringify(result, null, 1))
await browser.close()
