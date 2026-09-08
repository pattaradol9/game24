// Package skins defines the card-skin catalog: purchasable visual themes
// applied to the playing cards. Skins are bought with coins earned from
// gameplay and are restricted to Google-signed-in players (enforced by the
// handler/store, not here).
package skins

// Def is one card skin in the catalog.
type Def struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Price  int64  `json:"price"`  // 0 = default, always owned
	Rarity string `json:"rarity"` // common | rare | epic | legend
	Desc   Text   `json:"desc"`
}

// Text is a bilingual user-facing copy pair.
type Text struct {
	En string `json:"en"`
	Th string `json:"th"`
}

// Catalog lists every skin in showcase order (default first, priciest last).
var Catalog = []Def{
	{
		ID: "classic", Name: "Classic", Price: 0, Rarity: "common",
		Desc: Text{En: "The original deck — warm white card stock.", Th: "กองไพ่ต้นฉบับ สีขาวอุ่นคลาสสิก"},
	},
	{
		ID: "mono", Name: "Mono", Price: 800, Rarity: "common",
		Desc: Text{En: "Pure black on white. No color, just numbers.", Th: "ดำล้วนบนขาว ไม่มีสี มีแต่ตัวเลข"},
	},
	{
		ID: "midnight", Name: "Midnight", Price: 1000, Rarity: "rare",
		Desc: Text{En: "Deep navy cards for late-night sessions.", Th: "การ์ดสีน้ำเงินเข้ม สำหรับเล่นยามดึก"},
	},
	{
		ID: "forest", Name: "Forest", Price: 1200, Rarity: "rare",
		Desc: Text{En: "Moss green faces with cream ink.", Th: "การ์ดเขียวมอส ตัวเลขสีครีม"},
	},
	{
		ID: "sakura", Name: "Sakura", Price: 1500, Rarity: "rare",
		Desc: Text{En: "Blush pink petals on every hand.", Th: "กลีบซากุระสีชมพู บนทุกหน้าไพ่"},
	},
	{
		ID: "sunset", Name: "Sunset", Price: 1500, Rarity: "rare",
		Desc: Text{En: "Warm dusk gradient from orange to rose.", Th: "ไล่เฉดสีค่ำอุ่น ๆ จากส้มสู่ชมพู"},
	},
	{
		ID: "ocean", Name: "Ocean", Price: 2000, Rarity: "rare",
		Desc: Text{En: "Deep-sea teal that keeps its cool.", Th: "เขียวน้ำเงินลึกเหมือนก้นทะเล"},
	},
	{
		ID: "neon", Name: "Neon", Price: 2500, Rarity: "epic",
		Desc: Text{En: "Electric green on pitch black. Glows (honestly).", Th: "เขียวนีออนบนดำสนิท เรืองแสงจริง ๆ"},
	},
	{
		ID: "royal", Name: "Royal", Price: 3000, Rarity: "epic",
		Desc: Text{En: "Deep purple with gilded gold trim.", Th: "ม่วงหรูหรา ขอบทองคำโบราณ"},
	},
	{
		ID: "gold", Name: "Gold", Price: 4000, Rarity: "epic",
		Desc: Text{En: "Brushed gold stock for high rollers.", Th: "การ์ดสีทอง สำหรับสายฮีโร่ตัวจริง"},
	},
	{
		ID: "galaxy", Name: "Galaxy", Price: 5000, Rarity: "legend",
		Desc: Text{En: "A pocket of deep space on the table.", Th: "ห้วงอวกาศลึก ในคราบใบไพ่"},
	},
	{
		ID: "inferno", Name: "Inferno", Price: 6500, Rarity: "legend",
		Desc: Text{En: "Charcoal edge burning into ember red.", Th: "ดำถ่านรอยเขียม ลุกเป็นเพลิงแดง"},
	},
}

// ByID returns the skin with the given id and whether it exists.
func ByID(id string) (Def, bool) {
	for _, s := range Catalog {
		if s.ID == id {
			return s, true
		}
	}
	return Def{}, false
}
