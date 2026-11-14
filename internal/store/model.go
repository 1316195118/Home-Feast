package store

// Dish 是菜品库中的一道菜。Builtin 为 true 表示随程序内置，自定义菜品可自由编辑。
type Dish struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Cuisine   string   `json:"cuisine"`
	Desc      string   `json:"desc"`
	Tags      []string `json:"tags"`
	Image       string   `json:"image"`
	Spicy       int      `json:"spicy"`
	Minutes     int      `json:"minutes"`
	Ingredients []string `json:"ingredients"` // 配料，一行一项（可带用量）
	Steps       []string `json:"steps"`       // 做法步骤，按数组顺序编号
	Favorite    bool     `json:"favorite"`
	Builtin   bool     `json:"builtin"`
	CreatedAt string   `json:"createdAt"`
}

// Member 是家庭中的一个人，用头像 emoji 与昵称区分，不收集任何个人信息。
type Member struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Avatar    string   `json:"avatar"`
	Color     string   `json:"color"`
	Avoid     []string `json:"avoid,omitempty"` // 忌口 / 口味避雷标签，omitempty 兼容旧 members.json
	// HasPin 是计算字段：由 ListMembers() 按 pins.json 填充，不落盘、不含任何密码材料。
	HasPin    bool     `json:"hasPin,omitempty"`
	CreatedAt string   `json:"createdAt"`
}

// Order 是一次点菜。除了引用 ID，还留下菜名、菜系与点菜人快照，
// 这样以后删掉菜品或成员，历史菜单依然读得懂。
type Order struct {
	ID          string `json:"id"`
	DishID      string `json:"dishId"`
	DishName    string `json:"dishName"`
	DishCuisine string `json:"dishCuisine"`
	DishImage   string `json:"dishImage"`
	UserID      string `json:"userId"`
	UserName    string `json:"userName"`
	UserAvatar  string `json:"userAvatar"`
	Note        string `json:"note"`
	Done        bool   `json:"done"`
	Meal        string `json:"meal"` // 餐别：breakfast / lunch / dinner，空串表示未归类
	CreatedAt   string `json:"createdAt"`
}

// Plan 保存某一天的完整安排：谁掌勺、谁洗碗、备注、大家点的菜。
// 新增字段用空值零值兼容旧的 plans.json 与旧备份，无需数据迁移。
type Plan struct {
	Date         string  `json:"date"`
	ChefID       string  `json:"chefId"`
	DishwasherID string  `json:"dishwasherId"`
	Note         string  `json:"note"`
	Orders       []Order `json:"orders"`
	UpdatedAt    string  `json:"updatedAt"`
	// 骰子 PK：Dice 保存每人点数，DicePhase 为 rolling/tied/done，
	// PKShown 记录已经看过结果动画的成员（按用户维度只播一次）。
	// omitempty 保证旧的 plans.json / 备份不含这些字段也能直接读。
	Dice      map[string]int `json:"dice,omitempty"`
	DicePhase string          `json:"dicePhase,omitempty"`
	PKShown   []string        `json:"pkShown,omitempty"`
}

// PlanPatch 是编辑当天分工时允许改动的字段，nil 表示保持不变。
type PlanPatch struct {
	ChefID       *string
	DishwasherID *string
	Note         *string
	Dice         *map[string]int
	DicePhase    *string
	PKShown      *[]string
}

// MealSlot 是一顿饭的配置，由用户在「设置 → 用餐时段」里自定义。
// End 是收起时刻 "HH:MM"（24 小时制）：过了这个时刻，今天归到这一餐的菜
// 会自动从今日菜单收起，历史记录仍保留。Key 是写入 Order.Meal 的稳定标识。
// 旧的 settings.json 没有 meals 字段时，代码会自动使用默认三餐。
type MealSlot struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	End   string `json:"end"`
}

// RosterDay 是每周某一天的分工。ChefID / WashID 为空串表示这天没安排。
type RosterDay struct {
	ChefID string `json:"chefId"`
	WashID string `json:"washId"`
}

// Roster 是每周掌勺 / 洗碗排班表。Days 的 key 用 "0".."6"（周日..周六，
// 与 JS Date.getDay() 一致），缺项表示那天不安排。Enabled 关掉后前端不提示、不套用。
// 放进 Settings 而不是新建 roster.json：复用 settings 的读写、bootstrap 与备份链路，
// 旧 settings.json 没有这两个字段时零值兼容，无需数据迁移。
type Roster struct {
	Enabled bool                 `json:"enabled"`
	Days    map[string]RosterDay `json:"days"`
}

// NormalizeRoster 校验排班：只保留合法的星期 key 与确实存在的成员 ID，
// 防止前端手滑写进脏数据；同时避免同一天掌勺和洗碗是同一人（家庭场景没意义）。
func NormalizeRoster(in Roster, members []Member) Roster {
	known := make(map[string]bool, len(members))
	for _, m := range members {
		known[m.ID] = true
	}
	out := Roster{Enabled: in.Enabled, Days: map[string]RosterDay{}}
	for k, day := range in.Days {
		if len(k) != 1 || k[0] < '0' || k[0] > '6' {
			continue
		}
		if !known[day.ChefID] {
			day.ChefID = ""
		}
		if !known[day.WashID] {
			day.WashID = ""
		}
		if day.ChefID == "" && day.WashID == "" {
			continue
		}
		out.Days[k] = day
	}
	return out
}

// Settings 是可以随手改的家庭级配置。
type Settings struct {
	FamilyName     string     `json:"familyName"`
	HiddenCuisines []string   `json:"hiddenCuisines"` // 用户选择不展示的菜系 key 列表
	Meals          []MealSlot `json:"meals"`           // 空表示使用默认三餐
	Roster         Roster     `json:"roster"`          // 每周掌勺 / 洗碗排班
	Pantry         []string   `json:"pantry"`          // 冰箱里现有的食材，供智能推荐匹配
	AutoBackup     bool       `json:"autoBackup"`      // 每天 03:30 自动打包到 data/backups/
	BackupKeep     int        `json:"backupKeep"`      // 自动备份保留份数，0 视为默认 7
	// FamilyID 是「多家庭隔离」第一阶段的地基：只用于备份命名空间，
	// 当前所有数据仍共用一份 JSON，不做分仓，旧数据零值兼容。
	FamilyID       string     `json:"familyId,omitempty"`
	UpdatedAt      string     `json:"updatedAt"`
}

// Cuisine 是菜系分类。Image 为 AI 绘制的菜系插画，菜品未单独配图时复用它。
type Cuisine struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Image string `json:"image"`
	Desc  string `json:"desc"`
	Tint  string `json:"tint"`
}

// Cuisines 是全部内置分类，顺序即界面上的展示顺序。
// 默认都展示；用户可在「设置 → 菜系显示」里逐个关掉不想要的（存进 Settings.HiddenCuisines）。
var Cuisines = []Cuisine{
	// 重庆菜已并入川渝菜：52 道原重庆菜在 dishes.json 里统一用 sichuan，
	// 不再单独设分类，避免家人点菜时在两个几乎一样的菜系间来回翻。
	{"sichuan", "川渝菜", "🌶️", "/dishes/sichuan.jpg", "麻辣鲜香，巴蜀一锅", "#f3d9cd"},
	{"xinjiang", "新疆菜", "🍢", "/dishes/xinjiang.jpg", "豪迈香辣，西域风味", "#e8d5b8"},
	{"cantonese", "粤菜", "🥢", "/dishes/cantonese.jpg", "清鲜精致，讲究火候", "#dce9dd"},
	{"shandong", "鲁菜", "🐟", "/dishes/shandong.jpg", "咸鲜醇厚，擅用高汤", "#f0e2c8"},
	{"dongbei", "东北菜", "🥘", "/dishes/dongbei.jpg", "量大实在，咸鲜下饭", "#efe3d0"},
	{"jiangsu", "苏菜", "🍶", "/dishes/jiangsu.jpg", "清雅细腻，刀工见长", "#e3e0f0"},
	{"benbang", "本帮菜", "🍜", "/dishes/benbang.jpg", "浓油赤酱，甜咸适口", "#f0ddd3"},
	{"zhejiang", "浙菜", "🍃", "/dishes/zhejiang.jpg", "清爽软嫩，原味为本", "#dceadf"},
	{"fujian", "闽菜", "🍲", "/dishes/fujian.jpg", "汤鲜味醇，山海相融", "#f4dfcd"},
	{"hunan", "湘菜", "🔥", "/dishes/hunan.jpg", "香辣浓烈，下饭首选", "#f5d5cb"},
	{"anhui", "徽菜", "🏔️", "/dishes/anhui.jpg", "重油重色，山野风味", "#e6ddcd"},
	{"homestyle", "家常菜", "🍳", "/dishes/homestyle.jpg", "天天吃也不腻的老味道", "#f6e6c9"},
	{"soup", "汤羹", "🥣", "/dishes/soup.jpg", "一碗热汤，饭才圆满", "#dfe9ec"},
	{"staple", "主食", "🍚", "/dishes/staple.jpg", "米面馒头，管饱担当", "#eee4cf"},
	{"cold", "凉菜", "🥗", "/dishes/cold.jpg", "爽口开胃，先上桌", "#e1ebd8"},
	{"snack", "小吃", "🥟", "/dishes/snack.jpg", "解馋小食，随手一口", "#f3e6d0"},
	{"breakfast", "早餐", "🥞", "/dishes/breakfast.jpg", "一天从好好吃饭开始", "#f7e7d2"},
	{"dessert", "甜品饮品", "🍰", "/dishes/dessert.jpg", "饭后那口甜", "#f2dce2"},
	{"western", "西餐烘焙", "🍝", "/dishes/western.jpg", "偶尔换换口味", "#e4e2ee"},
}

// CuisineByKey 返回分类定义，未知分类回退到家常菜。
// 这里按 key 找而不是写死下标：以前用 Cuisines[8]，新增分类后下标错位会回退成徽菜。
func CuisineByKey(key string) Cuisine {
	for _, c := range Cuisines {
		if c.Key == key {
			return c
		}
	}
	for _, c := range Cuisines {
		if c.Key == "homestyle" {
			return c
		}
	}
	return Cuisines[0]
}
