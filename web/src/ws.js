// Room websocket wrapper with auto-reconnect while the match is live.
export function connectRoom(code, { name, token, hostKey, resume }, { onMessage, onOpen, onClose, onGiveUp } = {}) {
  let ws = null
  let closedByUs = false
  let retries = 0

  function connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    ws = new WebSocket(`${proto}://${location.host}/api/v1/ws/room/${code}`)
    ws.onopen = () => {
      retries = 0
      // the resume secret reattaches this browser's old seat, so a
      // refresh (or a mid-game socket retry) never forks the player
      send('join', { name, token, hostKey, resume })
      onOpen?.()
    }
    ws.onmessage = (ev) => {
      try {
        onMessage?.(JSON.parse(ev.data))
      } catch {
        /* ignore malformed frames */
      }
    }
    ws.onclose = () => {
      if (closedByUs) return
      onClose?.()
      if (retries < 5) {
        retries++
        setTimeout(connect, 800 * retries)
      } else {
        onGiveUp?.()
      }
    }
    ws.onerror = () => ws.close()
  }

  function send(type, data = {}) {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type, data }))
    }
  }

  function close() {
    closedByUs = true
    ws?.close()
  }

  connect()
  return { send, close }
}
