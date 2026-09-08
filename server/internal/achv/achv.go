// Package achv defines the achievement catalog and the pure unlock logic.
// Achievements are evaluated against a Snapshot of a player's lifetime
// statistics; newly unlocked entries carry EXP and coin rewards that scale
// with the achievement's tier (bronze → legend).
package achv

// Tier is the difficulty band of an achievement; rewards escalate per tier.
type Tier string

const (
	TierBronze   Tier = "bronze"
	TierSilver   Tier = "silver"
	TierGold     Tier = "gold"
	TierPlatinum Tier = "platinum"
	TierLegend   Tier = "legend"
)

// Metric names the lifetime counter an achievement tracks.
type Metric string

const (
	MetricTotalExp     Metric = "totalExp"
	MetricLevel        Metric = "level"
	MetricTotalCoins   Metric = "totalCoins"
	MetricSolved       Metric = "solved"       // Param: mode id, or "" for all modes
	MetricSkipped      Metric = "skipped"      // hands skipped / timed out, all modes
	MetricWins         Metric = "wins"         // multiplayer round wins
	MetricBestStreak   Metric = "bestStreak"   // Param: mode id, or "" for max across modes
	MetricHintsUsed    Metric = "hintsUsed"    // lifetime hint count
	MetricSolvesNoHint Metric = "solvesNoHint" // hands solved without any hint
	MetricBestTimeMs   Metric = "bestTimeMs"   // fastest solve; lower is better
	MetricModesPlayed  Metric = "modesPlayed"  // distinct modes with ≥1 solve
	MetricPlayDays     Metric = "playDays"     // distinct calendar days with ≥1 dealt hand
)

// Text is a bilingual user-facing copy pair.
type Text struct {
	En string `json:"en"`
	Th string `json:"th"`
}

// Def is one achievement in the catalog.
type Def struct {
	ID         string `json:"id"`
	Tier       Tier   `json:"tier"`
	Metric     Metric `json:"-"`
	Param      string `json:"-"` // mode id for mode-scoped metrics
	Target     int64  `json:"target"`
	ExpReward  int64  `json:"expReward"`
	CoinReward int64  `json:"coinReward"`
	Title      Text   `json:"title"`
	Desc       Text   `json:"desc"`
}

// Snapshot is the lifetime-stat view a player is evaluated against. The
// store fills it; this package never touches the database.
type Snapshot struct {
	TotalExp     int64
	TotalCoins   int64
	Level        int64
	Solved       map[string]int64 // mode -> hands solved ("" = total)
	Skipped      int64
	Wins         int64
	BestStreak   map[string]int64 // mode -> best streak ("" = max across modes)
	HintsUsed    int64
	SolvesNoHint int64
	BestTimeMs   int64 // 0 = no solve recorded yet
	ModesPlayed  int64
	PlayDays     int64
}

// Catalog lists all achievements, easiest family first. Exactly 50 entries.
var Catalog = []Def{
	// --- lifetime solves ---
	{ID: "solve-1", Tier: TierBronze, Metric: MetricSolved, Target: 1, ExpReward: 20, CoinReward: 15,
		Title: Text{En: "First Blood", Th: "เปิดสกอร์"},
		Desc:  Text{En: "Solve your first hand.", Th: "แก้มือแรกสำเร็จ"}},
	{ID: "solve-25", Tier: TierBronze, Metric: MetricSolved, Target: 25, ExpReward: 40, CoinReward: 25,
		Title: Text{En: "Getting Warm", Th: "เริ่มเข้าอ่อน"},
		Desc:  Text{En: "Solve 25 hands.", Th: "แก้สำเร็จ 25 มือ"}},
	{ID: "solve-100", Tier: TierSilver, Metric: MetricSolved, Target: 100, ExpReward: 80, CoinReward: 50,
		Title: Text{En: "Century Solver", Th: "นักแก้ร้อย"},
		Desc:  Text{En: "Solve 100 hands.", Th: "แก้สำเร็จ 100 มือ"}},
	{ID: "solve-250", Tier: TierSilver, Metric: MetricSolved, Target: 250, ExpReward: 150, CoinReward: 90,
		Title: Text{En: "Serial Solver", Th: "นักสะสมเฉลย"},
		Desc:  Text{En: "Solve 250 hands.", Th: "แก้สำเร็จ 250 มือ"}},
	{ID: "solve-500", Tier: TierGold, Metric: MetricSolved, Target: 500, ExpReward: 300, CoinReward: 180,
		Title: Text{En: "Hand Hunter", Th: "นักล่าห้าร้อย"},
		Desc:  Text{En: "Solve 500 hands.", Th: "แก้สำเร็จ 500 มือ"}},
	{ID: "solve-1000", Tier: TierGold, Metric: MetricSolved, Target: 1000, ExpReward: 500, CoinReward: 300,
		Title: Text{En: "Four-Digit Club", Th: "สโมสรพันมือ"},
		Desc:  Text{En: "Solve 1,000 hands.", Th: "แก้สำเร็จ 1,000 มือ"}},
	{ID: "solve-2500", Tier: TierPlatinum, Metric: MetricSolved, Target: 2500, ExpReward: 900, CoinReward: 600,
		Title: Text{En: "Grandmaster Grinder", Th: "จอมขยันแกรนด์มาสเตอร์"},
		Desc:  Text{En: "Solve 2,500 hands.", Th: "แก้สำเร็จ 2,500 มือ"}},
	{ID: "solve-5000", Tier: TierLegend, Metric: MetricSolved, Target: 5000, ExpReward: 1800, CoinReward: 1200,
		Title: Text{En: "Living Legend", Th: "ตำนานของเกม"},
		Desc:  Text{En: "Solve 5,000 hands.", Th: "แก้สำเร็จ 5,000 มือ"}},

	// --- best streak ---
	{ID: "streak-3", Tier: TierBronze, Metric: MetricBestStreak, Target: 3, ExpReward: 25, CoinReward: 15,
		Title: Text{En: "Hat-Trick", Th: "แฮตทริก"},
		Desc:  Text{En: "Reach a 3-hand streak.", Th: "สตรีคต่อเนื่อง 3 มือ"}},
	{ID: "streak-5", Tier: TierBronze, Metric: MetricBestStreak, Target: 5, ExpReward: 40, CoinReward: 25,
		Title: Text{En: "High Five", Th: "ไฮ้ไฟฟ์"},
		Desc:  Text{En: "Reach a 5-hand streak.", Th: "สตรีคต่อเนื่อง 5 มือ"}},
	{ID: "streak-10", Tier: TierSilver, Metric: MetricBestStreak, Target: 10, ExpReward: 90, CoinReward: 60,
		Title: Text{En: "Perfect Ten", Th: "เทนล้วน ๆ"},
		Desc:  Text{En: "Reach a 10-hand streak.", Th: "สตรีคต่อเนื่อง 10 มือ"}},
	{ID: "streak-20", Tier: TierGold, Metric: MetricBestStreak, Target: 20, ExpReward: 250, CoinReward: 150,
		Title: Text{En: "Unstoppable", Th: "หยุดไม่อยู่"},
		Desc:  Text{En: "Reach a 20-hand streak.", Th: "สตรีคต่อเนื่อง 20 มือ"}},
	{ID: "streak-35", Tier: TierPlatinum, Metric: MetricBestStreak, Target: 35, ExpReward: 600, CoinReward: 400,
		Title: Text{En: "Iron Focus", Th: "สมาธิเหล็กกล้า"},
		Desc:  Text{En: "Reach a 35-hand streak.", Th: "สตรีคต่อเนื่อง 35 มือ"}},
	{ID: "streak-50", Tier: TierLegend, Metric: MetricBestStreak, Target: 50, ExpReward: 1200, CoinReward: 800,
		Title: Text{En: "Half-Century Run", Th: "ครึ่งศตวรรษ"},
		Desc:  Text{En: "Reach a 50-hand streak.", Th: "สตรีคต่อเนื่อง 50 มือ"}},

	// --- level milestones ---
	{ID: "level-5", Tier: TierBronze, Metric: MetricLevel, Target: 5, ExpReward: 30, CoinReward: 20,
		Title: Text{En: "Apprentice", Th: "ผู้เริ่มต้น"},
		Desc:  Text{En: "Reach level 5.", Th: "ถึงเลเวล 5"}},
	{ID: "level-10", Tier: TierBronze, Metric: MetricLevel, Target: 10, ExpReward: 60, CoinReward: 40,
		Title: Text{En: "Journeyman", Th: "ช่างฝีมือ"},
		Desc:  Text{En: "Reach level 10.", Th: "ถึงเลเวล 10"}},
	{ID: "level-20", Tier: TierSilver, Metric: MetricLevel, Target: 20, ExpReward: 120, CoinReward: 80,
		Title: Text{En: "Silver Climber", Th: "นักไต่ระดับเงิน"},
		Desc:  Text{En: "Reach level 20.", Th: "ถึงเลเวล 20"}},
	{ID: "level-30", Tier: TierGold, Metric: MetricLevel, Target: 30, ExpReward: 260, CoinReward: 160,
		Title: Text{En: "Golden Mind", Th: "จิตทอง"},
		Desc:  Text{En: "Reach level 30.", Th: "ถึงเลเวล 30"}},
	{ID: "level-50", Tier: TierPlatinum, Metric: MetricLevel, Target: 50, ExpReward: 550, CoinReward: 350,
		Title: Text{En: "Platinum Thinker", Th: "นักคิดแพลตินัม"},
		Desc:  Text{En: "Reach level 50.", Th: "ถึงเลเวล 50"}},
	{ID: "level-70", Tier: TierPlatinum, Metric: MetricLevel, Target: 70, ExpReward: 800, CoinReward: 500,
		Title: Text{En: "Diamond Elite", Th: "หัวกะทิเพชร"},
		Desc:  Text{En: "Reach level 70.", Th: "ถึงเลเวล 70"}},
	{ID: "level-100", Tier: TierLegend, Metric: MetricLevel, Target: 100, ExpReward: 2000, CoinReward: 1300,
		Title: Text{En: "Master of 24", Th: "ราชาเกม 24"},
		Desc:  Text{En: "Reach level 100.", Th: "ถึงเลเวล 100"}},

	// --- per-mode first solves ---
	{ID: "jack-first", Tier: TierBronze, Metric: MetricSolved, Param: "jack", Target: 1, ExpReward: 25, CoinReward: 15,
		Title: Text{En: "Jack Attack", Th: "จ๊าคแอทแท็ค"},
		Desc:  Text{En: "Solve your first Jack hand.", Th: "แก้โหมดแจ็คสำเร็จครั้งแรก"}},
	{ID: "queen-first", Tier: TierBronze, Metric: MetricSolved, Param: "queen", Target: 1, ExpReward: 25, CoinReward: 15,
		Title: Text{En: "Fit for a Queen", Th: "ถวายความเคารพคุณควีน"},
		Desc:  Text{En: "Solve your first Queen hand.", Th: "แก้โหมดควีนสำเร็จครั้งแรก"}},
	{ID: "king-first", Tier: TierSilver, Metric: MetricSolved, Param: "king", Target: 1, ExpReward: 50, CoinReward: 30,
		Title: Text{En: "Fraction Throne", Th: "บัลลังก์เศษส่วน"},
		Desc:  Text{En: "Solve your first King hand.", Th: "แก้โหมดคิงสำเร็จครั้งแรก"}},
	{ID: "ace-first", Tier: TierGold, Metric: MetricSolved, Param: "ace", Target: 1, ExpReward: 90, CoinReward: 60,
		Title: Text{En: "Ace Up the Sleeve", Th: "เอซซ่อนในแขน"},
		Desc:  Text{En: "Solve your first Ace hand.", Th: "แก้โหมดเอซสำเร็จครั้งแรก"}},

	// --- per-mode mastery ---
	{ID: "jack-master", Tier: TierSilver, Metric: MetricSolved, Param: "jack", Target: 100, ExpReward: 100, CoinReward: 70,
		Title: Text{En: "Jack Whisperer", Th: "ผู้เข้าใจแจ็ค"},
		Desc:  Text{En: "Solve 100 Jack hands.", Th: "แก้โหมดแจ็ค 100 มือ"}},
	{ID: "queen-master", Tier: TierSilver, Metric: MetricSolved, Param: "queen", Target: 100, ExpReward: 120, CoinReward: 80,
		Title: Text{En: "Queen's Guard", Th: "องครักษ์ควีน"},
		Desc:  Text{En: "Solve 100 Queen hands.", Th: "แก้โหมดควีน 100 มือ"}},
	{ID: "king-master", Tier: TierGold, Metric: MetricSolved, Param: "king", Target: 100, ExpReward: 220, CoinReward: 140,
		Title: Text{En: "Fraction Royalty", Th: "เชื้อพระวงศ์เศษส่วน"},
		Desc:  Text{En: "Solve 100 King hands.", Th: "แก้โหมดคิง 100 มือ"}},
	{ID: "ace-master", Tier: TierPlatinum, Metric: MetricSolved, Param: "ace", Target: 100, ExpReward: 350, CoinReward: 220,
		Title: Text{En: "Ace of Aces", Th: "เอซเหนือเอซ"},
		Desc:  Text{En: "Solve 100 Ace hands.", Th: "แก้โหมดเอซ 100 มือ"}},

	// --- breadth ---
	{ID: "all-rounder", Tier: TierSilver, Metric: MetricModesPlayed, Target: 4, ExpReward: 100, CoinReward: 70,
		Title: Text{En: "All-Rounder", Th: "รอบด้าน"},
		Desc:  Text{En: "Solve at least one hand in every mode.", Th: "แก้ได้ครบทุกโหมดอย่างน้อยมือละหนึ่ง"}},

	// --- speed ---
	{ID: "swift-30s", Tier: TierBronze, Metric: MetricBestTimeMs, Target: 30000, ExpReward: 40, CoinReward: 25,
		Title: Text{En: "Quick Draw", Th: "มือไว"},
		Desc:  Text{En: "Solve a hand in under 30 seconds.", Th: "แก้มือได้ในเวลาต่ำกว่า 30 วินาที"}},
	{ID: "swift-15s", Tier: TierSilver, Metric: MetricBestTimeMs, Target: 15000, ExpReward: 90, CoinReward: 60,
		Title: Text{En: "Speed Demon", Th: "ปีศาจความเร็ว"},
		Desc:  Text{En: "Solve a hand in under 15 seconds.", Th: "แก้มือได้ในเวลาต่ำกว่า 15 วินาที"}},
	{ID: "swift-8s", Tier: TierGold, Metric: MetricBestTimeMs, Target: 8000, ExpReward: 220, CoinReward: 140,
		Title: Text{En: "Blur Hands", Th: "มือลับตา"},
		Desc:  Text{En: "Solve a hand in under 8 seconds.", Th: "แก้มือได้ในเวลาต่ำกว่า 8 วินาที"}},
	{ID: "swift-4s", Tier: TierLegend, Metric: MetricBestTimeMs, Target: 4000, ExpReward: 600, CoinReward: 400,
		Title: Text{En: "Lightning Legend", Th: "สายฟ้าฟาด"},
		Desc:  Text{En: "Solve a hand in under 4 seconds.", Th: "แก้มือได้ในเวลาต่ำกว่า 4 วินาที"}},

	// --- multiplayer wins ---
	{ID: "first-win", Tier: TierBronze, Metric: MetricWins, Target: 1, ExpReward: 30, CoinReward: 20,
		Title: Text{En: "First Conquest", Th: "ชนะครั้งแรก"},
		Desc:  Text{En: "Win your first room round.", Th: "ชนะรอบในห้องแข่งครั้งแรก"}},
	{ID: "win-10", Tier: TierBronze, Metric: MetricWins, Target: 10, ExpReward: 60, CoinReward: 40,
		Title: Text{En: "Rising Rival", Th: "คู่แข่งขึ้นบันได"},
		Desc:  Text{En: "Win 10 room rounds.", Th: "ชนะในห้องแข่ง 10 รอบ"}},
	{ID: "win-50", Tier: TierSilver, Metric: MetricWins, Target: 50, ExpReward: 150, CoinReward: 100,
		Title: Text{En: "Table Threat", Th: "นักชกประจำโต๊ะ"},
		Desc:  Text{En: "Win 50 room rounds.", Th: "ชนะในห้องแข่ง 50 รอบ"}},
	{ID: "win-200", Tier: TierGold, Metric: MetricWins, Target: 200, ExpReward: 400, CoinReward: 260,
		Title: Text{En: "Room Dominator", Th: "เจ้าห้อง"},
		Desc:  Text{En: "Win 200 room rounds.", Th: "ชนะในห้องแข่ง 200 รอบ"}},
	{ID: "win-500", Tier: TierPlatinum, Metric: MetricWins, Target: 500, ExpReward: 900, CoinReward: 600,
		Title: Text{En: "Untouchable", Th: "ไม่มีใครแตะต้อง"},
		Desc:  Text{En: "Win 500 room rounds.", Th: "ชนะในห้องแข่ง 500 รอบ"}},

	// --- coin wealth ---
	{ID: "coin-500", Tier: TierBronze, Metric: MetricTotalCoins, Target: 500, ExpReward: 50, CoinReward: 50,
		Title: Text{En: "Pocket Change", Th: "เงินคลุกรัก"},
		Desc:  Text{En: "Hold 500 coins at once.", Th: "สะสมเหรียญ 500 เหรียญ"}},
	{ID: "coin-2000", Tier: TierSilver, Metric: MetricTotalCoins, Target: 2000, ExpReward: 120, CoinReward: 120,
		Title: Text{En: "Full Purse", Th: "กระเป๋าเต็ม"},
		Desc:  Text{En: "Hold 2,000 coins at once.", Th: "สะสมเหรียญ 2,000 เหรียญ"}},
	{ID: "coin-10000", Tier: TierGold, Metric: MetricTotalCoins, Target: 10000, ExpReward: 350, CoinReward: 350,
		Title: Text{En: "Coin Baron", Th: "บารอนเหรียญ"},
		Desc:  Text{En: "Hold 10,000 coins at once.", Th: "สะสมเหรียญ 10,000 เหรียญ"}},
	{ID: "coin-50000", Tier: TierPlatinum, Metric: MetricTotalCoins, Target: 50000, ExpReward: 1000, CoinReward: 1000,
		Title: Text{En: "Mint Condition", Th: "โรงกษาปณ์เคลื่อนที่"},
		Desc:  Text{En: "Hold 50,000 coins at once.", Th: "สะสมเหรียญ 50,000 เหรียญ"}},

	// --- hints ---
	{ID: "purist-25", Tier: TierSilver, Metric: MetricSolvesNoHint, Target: 25, ExpReward: 120, CoinReward: 80,
		Title: Text{En: "Pure Solver", Th: "นักแก้บริสุทธิ์"},
		Desc:  Text{En: "Solve 25 hands without using any hint.", Th: "แก้ 25 มือโดยไม่ใช้ตัวช่วยเลย"}},
	{ID: "purist-100", Tier: TierGold, Metric: MetricSolvesNoHint, Target: 100, ExpReward: 300, CoinReward: 200,
		Title: Text{En: "Hintless Master", Th: "ปรมาจารย์ไร้ตัวช่วย"},
		Desc:  Text{En: "Solve 100 hands without using any hint.", Th: "แก้ 100 มือโดยไม่ใช้ตัวช่วยเลย"}},
	{ID: "hint-curious", Tier: TierBronze, Metric: MetricHintsUsed, Target: 10, ExpReward: 25, CoinReward: 15,
		Title: Text{En: "Curious Mind", Th: "จิตใจอยากรู้"},
		Desc:  Text{En: "Use 10 hints.", Th: "ใช้ตัวช่วย 10 ครั้ง"}},

	// --- dedication ---
	{ID: "days-3", Tier: TierBronze, Metric: MetricPlayDays, Target: 3, ExpReward: 60, CoinReward: 40,
		Title: Text{En: "Regular", Th: "ลูกค้าประจำ"},
		Desc:  Text{En: "Play on 3 different days.", Th: "เล่นต่างวันกัน 3 วัน"}},
	{ID: "days-7", Tier: TierSilver, Metric: MetricPlayDays, Target: 7, ExpReward: 150, CoinReward: 100,
		Title: Text{En: "Weekly Ritual", Th: "พิธีประจำสัปดาห์"},
		Desc:  Text{En: "Play on 7 different days.", Th: "เล่นต่างวันกัน 7 วัน"}},
	{ID: "days-30", Tier: TierGold, Metric: MetricPlayDays, Target: 30, ExpReward: 400, CoinReward: 260,
		Title: Text{En: "Devoted", Th: "ศรัทธามั่น"},
		Desc:  Text{En: "Play on 30 different days.", Th: "เล่นต่างวันกัน 30 วัน"}},

	// --- skips ---
	{ID: "skip-10", Tier: TierBronze, Metric: MetricSkipped, Target: 10, ExpReward: 30, CoinReward: 20,
		Title: Text{En: "Strategist", Th: "นักวางแผน"},
		Desc:  Text{En: "Skip 10 hands. Sometimes wisdom is knowing when to fold.", Th: "ข้าม 10 มือ บางทีความฉลาดคือรู้ว่าเมื่อไหร่ควรวาง"}},
}

// ByID returns the achievement with the given id and whether it exists.
func ByID(id string) (Def, bool) {
	for _, d := range Catalog {
		if d.ID == id {
			return d, true
		}
	}
	return Def{}, false
}

// Value reads the snapshot field a def tracks. For lower-is-better metrics
// (best solve time) the raw value is returned; callers apply the comparison.
func Value(d Def, s Snapshot) int64 {
	switch d.Metric {
	case MetricTotalExp:
		return s.TotalExp
	case MetricLevel:
		return s.Level
	case MetricTotalCoins:
		return s.TotalCoins
	case MetricSolved:
		return s.Solved[d.Param]
	case MetricSkipped:
		return s.Skipped
	case MetricWins:
		return s.Wins
	case MetricBestStreak:
		return s.BestStreak[d.Param]
	case MetricHintsUsed:
		return s.HintsUsed
	case MetricSolvesNoHint:
		return s.SolvesNoHint
	case MetricBestTimeMs:
		return s.BestTimeMs
	case MetricModesPlayed:
		return s.ModesPlayed
	case MetricPlayDays:
		return s.PlayDays
	default:
		return 0
	}
}

// unlockedWhen reports whether a def's value crosses its target. Best-time
// achievements unlock when a solve exists (value > 0) that is fast enough;
// everything else unlocks once the counter reaches the target.
func unlockedWhen(d Def, value int64) bool {
	if d.Metric == MetricBestTimeMs {
		return value > 0 && value <= d.Target
	}
	return value >= d.Target
}

// Evaluate returns the catalog entries the snapshot now satisfies but the
// unlocked set does not contain, in catalog order.
func Evaluate(s Snapshot, unlocked map[string]bool) []Def {
	var out []Def
	for _, d := range Catalog {
		if unlocked[d.ID] {
			continue
		}
		if unlockedWhen(d, Value(d, s)) {
			out = append(out, d)
		}
	}
	return out
}
