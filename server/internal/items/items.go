// Package items defines the consumable boost-item catalog: one-shot items a
// player buys with coins, stocks in their inventory and activates for a
// timed personal payout boost. Personal boosts stack additively with the
// server-wide boost campaigns (a ×2 server boost plus a ×2 item pays ×3,
// never ×4) — the stacking itself lives in the store's payout path. Items of
// the same kind are mutually exclusive: activating one REPLACES whatever
// window of that kind is still running (the client warns before the swap).
// Buying and using are restricted to Google-signed-in players (enforced by
// the handler/store, not here).
package items

// Text is a bilingual user-facing copy pair.
type Text struct {
	En string `json:"en"`
	Th string `json:"th"`
}

// Def is one consumable item in the catalog. Kind selects what the item
// does: "exp" and "coins" arm a personal payout boost — Multiplier is the
// personal bonus while its window runs and DurationMin is how long one
// activation lasts; activating an item always starts a fresh window and
// overwrites any live window of the same kind (same-kind boosts replace,
// never combine). Kind "time" is a play helper instead: it never arms a
// window, its ExtraSeconds are what one unit adds to a solo hand's countdown
// when time runs out, and the store spends it through the round-extend path
// rather than the boost-window path. Kind "skip" is the other play helper:
// one unit lets the player bail out of the current solo hand through the
// Skip button, once per play session (the store spends it through the
// round-skip path). Rarity tiers the item by power/price
// and drives the card styling the same way card-skin rarities do
// (common < rare < epic < legend).
type Def struct {
	ID           string  `json:"id"`
	Name         Text    `json:"name"`
	Desc         Text    `json:"desc"`
	Kind         string  `json:"kind"` // "exp" | "coins" | "time"
	Multiplier   float64 `json:"multiplier"`
	DurationMin  int     `json:"durationMinutes"`
	ExtraSeconds int     `json:"extraSeconds"` // time items only: seconds one unit adds to a hand
	Price        int64   `json:"price"`        // coins, charged on purchase
	Rarity       string  `json:"rarity"`       // common | rare | epic | legend
}

// Catalog lists every item in showcase order (per kind: gentle → fierce,
// cheapest first).
var Catalog = []Def{
	{
		ID: "coin2", Name: Text{En: "Coin Charm", Th: "เครื่องรางเหรียญ"}, Kind: "coins",
		Multiplier: 2, DurationMin: 30, Price: 300, Rarity: "common",
		Desc: Text{
			En: "Personal ×2 coin payout for 30 minutes. Stacks additively with server boosts (×2 + ×2 = ×3). Replaces any running coin boost.",
			Th: "โบนัสเหรียญส่วนตัว ×2 นาน 30 นาที ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน (×2 + ×2 = ×3) ใช้ซ้ำจะแทนที่บูสต์เหรียญเดิม",
		},
	},
	{
		ID: "exp2", Name: Text{En: "EXP Tonic", Th: "โทนิค EXP"}, Kind: "exp",
		Multiplier: 2, DurationMin: 30, Price: 400, Rarity: "common",
		Desc: Text{
			En: "Personal ×2 EXP for 30 minutes. Stacks additively with server boosts (×2 + ×2 = ×3). Replaces any running EXP boost.",
			Th: "โบนัส EXP ส่วนตัว ×2 นาน 30 นาที ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน (×2 + ×2 = ×3) ใช้ซ้ำจะแทนที่บูสต์ EXP เดิม",
		},
	},
	{
		ID: "coin2h", Name: Text{En: "Enduring Coin Charm", Th: "เครื่องรางเหรียญยาว"}, Kind: "coins",
		Multiplier: 2, DurationMin: 60, Price: 550, Rarity: "rare",
		Desc: Text{
			En: "Personal ×2 coin payout for a full hour. Stacks additively with server boosts. Replaces any running coin boost.",
			Th: "โบนัสเหรียญส่วนตัว ×2 นานหนึ่งชั่วโมงเต็ม ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน ใช้ซ้ำจะแทนที่บูสต์เหรียญเดิม",
		},
	},
	{
		ID: "exp2h", Name: Text{En: "Enduring EXP Tonic", Th: "โทนิค EXP ยาว"}, Kind: "exp",
		Multiplier: 2, DurationMin: 60, Price: 720, Rarity: "rare",
		Desc: Text{
			En: "Personal ×2 EXP for a full hour. Stacks additively with server boosts. Replaces any running EXP boost.",
			Th: "โบนัส EXP ส่วนตัว ×2 นานหนึ่งชั่วโมงเต็ม ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน ใช้ซ้ำจะแทนที่บูสต์ EXP เดิม",
		},
	},
	{
		ID: "coin5", Name: Text{En: "Coin Trove", Th: "กล่องสมบัติเหรียญ"}, Kind: "coins",
		Multiplier: 5, DurationMin: 30, Price: 1100, Rarity: "epic",
		Desc: Text{
			En: "Personal ×5 coin payout for 30 minutes. Stacks additively with server boosts (×2 + ×5 = ×6). Replaces any running coin boost.",
			Th: "โบนัสเหรียญส่วนตัว ×5 นาน 30 นาที ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน (×2 + ×5 = ×6) ใช้ซ้ำจะแทนที่บูสต์เหรียญเดิม",
		},
	},
	{
		ID: "exp5", Name: Text{En: "Grand EXP Elixir", Th: "อีลิกซ์ EXP ใหญ่"}, Kind: "exp",
		Multiplier: 5, DurationMin: 30, Price: 1400, Rarity: "epic",
		Desc: Text{
			En: "Personal ×5 EXP for 30 minutes. Stacks additively with server boosts (×2 + ×5 = ×6). Replaces any running EXP boost.",
			Th: "โบนัส EXP ส่วนตัว ×5 นาน 30 นาที ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน (×2 + ×5 = ×6) ใช้ซ้ำจะแทนที่บูสต์ EXP เดิม",
		},
	},
	{
		ID: "coin10", Name: Text{En: "Midas Quarter-Hour", Th: "หนึ่งไต้ของมิดาส"}, Kind: "coins",
		Multiplier: 10, DurationMin: 15, Price: 1900, Rarity: "legend",
		Desc: Text{
			En: "Personal ×10 coin payout for 15 frantic minutes. Stacks additively with server boosts. Replaces any running coin boost.",
			Th: "โบนัสเหรียญส่วนตัว ×10 นาน 15 นาทีสุดระทึก ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน ใช้ซ้ำจะแทนที่บูสต์เหรียญเดิม",
		},
	},
	{
		ID: "exp10", Name: Text{En: "EXP Overdrive", Th: "EXP ทะลุขีด"}, Kind: "exp",
		Multiplier: 10, DurationMin: 15, Price: 2400, Rarity: "legend",
		Desc: Text{
			En: "Personal ×10 EXP for 15 frantic minutes. Stacks additively with server boosts. Replaces any running EXP boost.",
			Th: "โบนัส EXP ส่วนตัว ×10 นาน 15 นาทีสุดระทึก ซ้อนกับบูสต์เซิร์ฟเวอร์แบบบวกกัน ใช้ซ้ำจะแทนที่บูสต์ EXP เดิม",
		},
	},
	{
		ID: "time30", Name: Text{En: "Sand Saver", Th: "นาฬิกาทรายต่อเวลา"}, Kind: "time",
		ExtraSeconds: 30, Price: 150, Rarity: "common",
		Desc: Text{
			En: "When a solo hand's time runs out, the game asks before spending one unit to add 30 more seconds. One use per play session.",
			Th: "เมื่อเวลาของมือเดี่ยวหมด เกมจะถามก่อนใช้ 1 ชิ้นเพื่อต่อเวลาอีก 30 วินาที ใช้ได้ 1 ครั้งต่อการเข้าเล่นหนึ่งครั้ง",
		},
	},
	{
		ID: "skip1", Name: Text{En: "Skip Pass", Th: "บัตรข้ามมือ"}, Kind: "skip",
		Price: 120, Rarity: "common",
		Desc: Text{
			En: "The Skip button spends one unit to fold the current hand and show the solution. One skip per play session.",
			Th: "กดปุ่ม \"ข้าม\" เพื่อใช้ 1 ชิ้น ยอมแพ้มือปัจจุบันและดูเฉลยทันที ใช้ได้ 1 ครั้งต่อการเข้าเล่นหนึ่งครั้ง",
		},
	},
}

// TimeItem returns the catalog's play-helper time item (kind "time") — the
// one the round-extend path spends. False when the catalog carries none.
func TimeItem() (Def, bool) {
	for _, it := range Catalog {
		if it.Kind == "time" {
			return it, true
		}
	}
	return Def{}, false
}

// SkipItem returns the catalog's play-helper skip item (kind "skip") — the
// one the round-skip path spends. False when the catalog carries none.
func SkipItem() (Def, bool) {
	for _, it := range Catalog {
		if it.Kind == "skip" {
			return it, true
		}
	}
	return Def{}, false
}

// ByID returns the item with the given id and whether it exists.
func ByID(id string) (Def, bool) {
	for _, it := range Catalog {
		if it.ID == id {
			return it, true
		}
	}
	return Def{}, false
}

// RarityRank orders the rarity tiers (higher = flashier) so clients and the
// store can pick the dominant tier of a stack.
var rarityRank = map[string]int{"common": 0, "rare": 1, "epic": 2, "legend": 3}

// RarityRank returns the sort rank of a rarity tier (unknown → common).
func RarityRank(rarity string) int {
	return rarityRank[rarity]
}
