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

// Def is one consumable item in the catalog. Kind selects which payout the
// item boosts ("exp" or "coins"); Multiplier is the personal bonus the item
// grants while its window runs; DurationMin is how long one activation
// lasts. Activating an item always starts a fresh window and overwrites any
// live window of the same kind — same-kind boosts replace, never combine.
// Rarity tiers the item by power/price and drives the card styling the same
// way card-skin rarities do (common < rare < epic < legend).
type Def struct {
	ID          string  `json:"id"`
	Name        Text    `json:"name"`
	Desc        Text    `json:"desc"`
	Kind        string  `json:"kind"` // "exp" | "coins"
	Multiplier  float64 `json:"multiplier"`
	DurationMin int     `json:"durationMinutes"`
	Price       int64   `json:"price"`  // coins, charged on purchase
	Rarity      string  `json:"rarity"` // common | rare | epic | legend
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
