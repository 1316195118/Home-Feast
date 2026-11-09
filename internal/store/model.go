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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	Color     string `json:"color"`
	CreatedAt string `json:"createdAt"`
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

// Settings 是可以随手改的家庭级配置。
type Settings struct {
	FamilyName     string     `json:"familyName"`
	HiddenCuisines []string   `json:"hiddenCuisines"` // 用户选择不展示的菜系 key 列表
	Meals          []MealSlot `json:"meals"`           // 空表示使用默认三餐
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
	{"chongqing", "重庆菜", "🌶️", "/dishes/sichuan.jpg", "麻辣鲜香，山城江湖味", "#f3d9cd"},
	{"sichuan", "川菜", "🌶️", "/dishes/sichuan.jpg", "麻辣鲜香，一菜一格", "#f3d9cd"},
	{"cantonese", "粤菜", "🥢", "/dishes/cantonese.jpg", "清鲜精致，讲究火候", "#dce9dd"},
	{"shandong", "鲁菜", "🐟", "/dishes/shandong.jpg", "咸鲜醇厚，擅用高汤", "#f0e2c8"},
	{"jiangsu", "苏菜", "🍶", "/dishes/jiangsu.jpg", "清雅细腻，刀工见长", "#e3e0f0"},
	{"zhejiang", "浙菜", "🍃", "/dishes/zhejiang.jpg", "清爽软嫩，原味为本", "#dceadf"},
	{"fujian", "闽菜", "🍲", "/dishes/fujian.jpg", "汤鲜味醇，山海相融", "#f4dfcd"},
	{"hunan", "湘菜", "🔥", "/dishes/hunan.jpg", "香辣浓烈，下饭首选", "#f5d5cb"},
	{"anhui", "徽菜", "🏔️", "/dishes/anhui.jpg", "重油重色，山野风味", "#e6ddcd"},
	{"homestyle", "家常菜", "🍳", "/dishes/homestyle.jpg", "天天吃也不腻的老味道", "#f6e6c9"},
	{"soup", "汤羹", "🥣", "/dishes/soup-staple.jpg", "一碗热汤，饭才圆满", "#dfe9ec"},
	{"staple", "主食", "🍚", "/dishes/soup-staple.jpg", "米面馒头，管饱担当", "#eee4cf"},
	{"cold", "凉菜", "🥗", "/dishes/homestyle.jpg", "爽口开胃，先上桌", "#e1ebd8"},
	{"breakfast", "早餐", "🥞", "/dishes/homestyle.jpg", "一天从好好吃饭开始", "#f7e7d2"},
	{"dessert", "甜品饮品", "🍰", "/dishes/homestyle.jpg", "饭后那口甜", "#f2dce2"},
	{"western", "西餐烘焙", "🍝", "/dishes/homestyle.jpg", "偶尔换换口味", "#e4e2ee"},
}

// CuisineByKey 返回分类定义，未知分类回退到家常菜。
func CuisineByKey(key string) Cuisine {
	for _, c := range Cuisines {
		if c.Key == key {
			return c
		}
	}
	return Cuisines[8]
}
