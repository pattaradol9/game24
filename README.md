# 24 Game 🎴

เกมคณิตศาสตร์สุดคลาสสิก **"24 Game"** ในรูปแบบเว็บเกมการ์ตูน — รับไพ่ 4 ใบ ใช้ `+ − × ÷` รวมให้เหลือไพ่เดียวและมีค่าเท่ากับ **24** เล่นคนเดียวเก็บ EXP ไต่ Tier หรือสร้างห้องแข่งกับเพื่อนแบบเรียลไทม์

## คุณสมบัติ

- **Single Player** — จับเวลา, streak, hint, ข้ามมือ (โชว์เฉลย), สะสม EXP ต่อโหมด
- **Multiplayer Room** — สร้างห้อง 6 หลัก แชร์ลิงก์เชิญเพื่อน แข่งกันแก้มือเดียวกัน คนแรกชนะรอบ กำหนดจำนวนรอบ 12/24/36/48 และโควตาตัวช่วย (ทุกคนเท่ากัน) จบแมตช์สรุปโพเดียมในห้อง (ผู้ชนะรอบยังได้ EXP เข้าสถิติจริง)
- **4 โหมดความยาก** — solver เป็นตัวการันตีคุณภาพมือทุกใบ:
  | โหมด | ตัวเลข | เงื่อนไขเฉลย | เวลา | EXP |
  |---|---|---|---|---|
  | JACK | 1–10 | มีเฉลยเลขเต็มล้วน | 120 วิ | ×1 |
  | QUEEN | 1–13 | คลาสสิก | 90 วิ | ×2 |
  | KING | 1–13 | ต้องใช้เศษส่วนขั้นกลาง | 75 วิ | ×3 |
  | ACE | 1–19 | เศษส่วน + เฉลยเดียวเท่านั้น | 60 วิ | ×5 |
- **ระบบ Tier + Level** (เฉพาะผู้เล่นที่ Sign in with Google)
  - Level: cumulative EXP = `30×(n−1)×n` → Lv.100 = 297,000 EXP
  - Tier = ทุก 20 เลเวล: Bronze / Silver (Lv.20) / Gold (Lv.40) / Platinum (Lv.60) / Diamond (Lv.80) / Master (Lv.100)
  - Tier ให้ **โบนัส hint ใน Single**: Gold/Platinum +1, Diamond/Master +2 (Bronze/Silver = 1 ครั้ง/มือ)
  - ผู้เล่น Anonymous (กรอกชื่อเล่น) เล่นได้ทุกโหมดแต่ไม่สะสมสถิติ
- **Leaderboard แยกตามโหมด** — เฉพาะผู้เล่นที่ล็อกอิน (guest ไม่ติดกระดาน)
- **กันโกง** — server เป็นคนแจกไพ่ บันทึกเวลาที่แจก และตรวจ trace การคำนวณด้วยเลขเศษส่วนแบบ exact ฝั่ง server เสมอ
- **เข้ารหัสข้อมูลผู้เล่น** — Envelope Encryption: Tink keyset (DEK, AES256-GCM) ถูกห่อด้วย Google Cloud KMS (KEK) เรียก KMS เฉพาะตอน start service แล้วถือ keyset ใน memory จนกว่าจะ restart (fail-closed) · คอลัมน์ที่เข้ารหัส: token, email, google_sub, nickname, picture · ค้นหาด้วย hash (`token_hash`, `sub_hash` + salt ใน wrapped blob)

## โครงสร้าง

```
web/     Vue 3 + Vite + vue-router (SPA การ์ตูน, ไทย/อังกฤษ, ธีมสว่าง/มืด)
server/  Go (chi + gorilla/websocket + modernc.org/sqlite + Tink)
         cmd/server + internal/{game,progress,crypto,store,room,ws,auth,handler,httpserver,webui,config}
```

Build แล้วได้ **binary เดียว** ที่ฝัง SPA ไว้ในตัว (`go:embed`) เปิดพอร์ตเดียวจบ

## เริ่มใช้งาน (development)

```bash
# terminal 1: API server (โหมดเข้ารหัส local, ไม่ต้องมี GCP)
cd server && go run ./cmd/server

# terminal 2: web dev server (proxy /api -> :8080)
cd web && npm install && npm run dev
```

เปิด http://localhost:5173

## Build & Deploy

```bash
make build          # web build -> embed -> single binary server/game24-server
./server/game24-server
```

หรือทั้งชุดด้วย Docker:

```bash
docker build -t game24 .
docker run --rm -p 8080:8080 -v game24-data:/data game24
```

### ตัวแปรสภาพแวดล้อม (ดูทั้งหมดใน `.env.example`)

| ตัวแปร | ค่าเริ่มต้น | ความหมาย |
|---|---|---|
| `APP_PORT` | 8080 | พอร์ทบริการ |
| `DB_PATH` | data/game24.db | ไฟล์ SQLite |
| `ENCRYPTION_MODE` | local | `local` (fake KMS, dev) หรือ `kms` (Google Cloud KMS) |
| `KMS_KEY_URI` | — | `gcp-kms://projects/.../cryptoKeys/...` (จำเป็นเมื่อ mode=kms) |
| `GOOGLE_APPLICATION_CREDENTIALS` | — | ไฟล์ service account (สิทธิ์ `cloudkms.cryptoKeyEncrypterDecrypter`) หรือเว้นว่างเพื่อใช้ ADC |
| `GOOGLE_OAUTH_CLIENT_ID` | — | OAuth Web client id; เว้นว่าง = ปิด Google sign-in (เล่น guest ได้) |
| `CORS_ORIGINS` | http://localhost:5173 | origins ที่อนุญาต |

### ตั้งค่า Sign in with Google

1. สร้าง OAuth Client (type: **Web application**) ใน Google Cloud Console
2. Authorized JavaScript origins: `http://localhost:5173`, `http://localhost:8080` (+ โดเมนจริง)
3. ใส่ client id ลง `GOOGLE_OAUTH_CLIENT_ID`

### ตั้งค่า Google Cloud KMS (production)

1. สร้าง KeyRing + CryptoKey (Software, AES-256/GCM ก็ได้ — Tink ใช้เป็น KEK ผ่าน envelope)
2. สร้าง service account ให้สิทธิ์ `roles/cloudkms.cryptoKeyEncrypterDecrypter`
3. ตั้ง `ENCRYPTION_MODE=kms`, `KMS_KEY_URI=gcp-kms://...`, และไฟล์ key ผ่าน `GOOGLE_APPLICATION_CREDENTIALS`
4. ครั้งแรกที่ start ระบบจะ generate keyset + salt แล้วห่อเก็บในตาราง `system_keys` — หลังจากนั้น start ทุกครั้งจะเรียก KMS แค่ครั้งเดียวเพื่อ unwrap

## การทดสอบ

```bash
make test   # go test ./... + web core (node --test)
make lint   # go vet + gofmt
```

## API สรุป (prefix `/api/v1`)

| Method | Path | คำอธิบาย |
|---|---|---|
| POST | `/players` | สมัคร guest {nickname} → player + token |
| POST | `/auth/google` | sign in {credential} → player + token |
| GET | `/me` | สถิติตัวเอง + level/tier + progress |
| POST | `/rounds` | แจกมือ {mode} (จำกัดเวลา + hint quota ตาม tier) |
| POST | `/rounds/{id}/submit` | ส่ง trace {steps} — server ตรวจ + ให้ EXP |
| POST | `/rounds/{id}/skip` | ข้าม (รีเซ็ต streak) + เฉลย |
| POST | `/rounds/{id}/hint` | ใช้ hint (โชว์ step แรก) |
| GET | `/leaderboard?mode=` | กระดานรายโหมด (เฉพาะผู้เล่นล็อกอิน) |
| POST | `/rooms` | สร้างห้อง {mode, rounds, hintQuota, regenQuota} → code + hostKey |
| WS | `/ws/room/{code}` | join/start/submit/hint/regen/leave |

ทุก response เป็น envelope `{success, message, data}`

## หมายเหตุด้านความปลอดภัย

- โหมด `ENCRYPTION_MODE=local` ใช้ fake KMS ของ Tink (KEK อยู่ใน DB เอง) **ห้ามใช้ใน production**
- ถ้า KMS เข้าไม่ได้ตอน start จะ **ไม่ start** (fail-closed) ตามดีไซน์; ระบบที่รันอยู่ไม่กระทบ
- TLS ควรทำที่ reverse proxy (nginx) หน้า service
- Key rotation: Tink keyset รองรับหลาย key (เปลี่ยน primary แล้ว re-encrypt ข้อมูลเก่า) — เป็นงานต่อยอด
