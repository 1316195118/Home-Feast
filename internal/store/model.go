package store

import "sort"

// Dish 是菜品库中的一道菜。Builtin 为 true 表示随程序内置，自定义菜品可自由编辑。
type Dish struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Cuisine     string   `json:"cuisine"`
	Desc        string   `json:"desc"`
	Tags        []string `json:"tags"`
	Image       string   `json:"image"`
	Spicy       int      `json:"spicy"`
	Minutes     int      `json:"minutes"`
	Ingredients []string `json:"ingredients"` // 配料，一行一项（可带用量）
	Steps       []string `json:"steps"`       // 做法步骤，按数组顺序编号
	// Servings 是当前配料用量的基准份数；0 时前端按 2 人份处理，兼容旧菜谱。
	Servings int  `json:"servings,omitempty"`
	Favorite bool `json:"favorite"`
	// Rating 是「做过之后觉得怎么样」：0 未评、1 一般、2 好吃、3 超赞。
	// 只影响推荐权重，不参与任何统计口径。
	Rating    int    `json:"rating"`
	Builtin   bool   `json:"builtin"`
	CreatedAt string `json:"createdAt"`
}

// Member 是家庭中的一个人，用头像 emoji 与昵称区分，不收集任何个人信息。
type Member struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Avatar string   `json:"avatar"`
	Color  string   `json:"color"`
	Avoid  []string `json:"avoid,omitempty"` // 忌口 / 口味避雷标签，omitempty 兼容旧 members.json
	// Allergies 是硬性过敏原：自动配餐直接排除，手动点菜必须二次确认。
	Allergies []string `json:"allergies,omitempty"`
	// HasPin 是计算字段：由 ListMembers() 按 pins.json 填充，不落盘、不含任何密码材料。
	HasPin    bool   `json:"hasPin,omitempty"`
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
	Date string `json:"date"`
	// 分工支持多人：ChefIDs / DishwasherIDs 是唯一真相；
	// 旧的单人字段保留并与数组首位同步，保证还没更新的旧前端也能显示第一个人。
	ChefIDs       []string `json:"chefIds,omitempty"`
	DishwasherIDs []string `json:"dishwasherIds,omitempty"`
	// MealDuties 按餐次分别记录分工（key 是餐次 key，如 breakfast/lunch/dinner）。
	// 顶层的 ChefIDs/DishwasherIDs 只作为旧数据兼容与「全天」语义，界面按餐读取。
	MealDuties   map[string]MealDuty `json:"mealDuties,omitempty"`
	ChefID       string              `json:"chefId"`
	DishwasherID string              `json:"dishwasherId"`
	Note         string              `json:"note"`
	Orders       []Order             `json:"orders"`
	UpdatedAt    string              `json:"updatedAt"`
	// 骰子 PK：Dice 保存每人点数，DicePhase 为 rolling/tied/done，
	// PKShown 记录已经看过结果动画的成员（按用户维度只播一次）。
	// omitempty 保证旧的 plans.json / 备份不含这些字段也能直接读。
	Dice      map[string]int `json:"dice,omitempty"`
	DicePhase string         `json:"dicePhase,omitempty"`
	PKShown   []string       `json:"pkShown,omitempty"`
}

// NormalizeDuty 把旧数据里的单人字段补进数组，并把数组首位同步回单人字段。
// 读和写都调用它，前端因此永远能拿到 chefIds / dishwasherIds 两个数组。
func (p *Plan) NormalizeDuty() {
	if len(p.ChefIDs) == 0 && p.ChefID != "" {
		p.ChefIDs = []string{p.ChefID}
	}
	if len(p.DishwasherIDs) == 0 && p.DishwasherID != "" {
		p.DishwasherIDs = []string{p.DishwasherID}
	}
	p.ChefID = ""
	if len(p.ChefIDs) > 0 {
		p.ChefID = p.ChefIDs[0]
	}
	p.DishwasherID = ""
	if len(p.DishwasherIDs) > 0 {
		p.DishwasherID = p.DishwasherIDs[0]
	}
}

// MealDuty 是某一餐的分工。
type MealDuty struct {
	ChefIDs       []string `json:"chefIds,omitempty"`
	DishwasherIDs []string `json:"dishwasherIds,omitempty"`
}

// DutyFor 返回某餐的分工。没有单独安排时返回空值——按用户要求，
// 空档就是「没人认领」，不沿用上一餐。
func (p *Plan) DutyFor(meal string) MealDuty {
	if meal == "" || p.MealDuties == nil {
		return MealDuty{}
	}
	return p.MealDuties[meal]
}

// PlanPatch 是编辑当天分工时允许改动的字段，nil 表示保持不变。
type PlanPatch struct {
	// DutyMeal 非空时，本次分工只改这一餐；为空表示改顶层（兼容旧调用）
	DutyMeal      string
	ChefIDs       *[]string
	DishwasherIDs *[]string
	ChefID        *string // 兼容旧调用：等价于把数组设为单元素
	DishwasherID  *string
	Note          *string
	Dice          *map[string]int
	DicePhase     *string
	PKShown       *[]string
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

// PantryStock 是某样冰箱食材的数量、单位和保质期。
// Pantry 仍保留纯名称数组，旧前端与旧备份无需迁移；数量信息为空时只做「冰箱有」提示。
type PantryStock struct {
	Quantity float64 `json:"quantity,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Added    string  `json:"added,omitempty"`
	Expires  string  `json:"expires,omitempty"`
}

// ShoppingMark 是全家共享的采购状态。Status 为 claimed（有人去买）或 bought（已买到）。
type ShoppingMark struct {
	Date      string `json:"date"`
	Status    string `json:"status"`
	UserID    string `json:"userId,omitempty"`
	UserName  string `json:"userName,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
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
	CuisineOrder   []string   `json:"cuisineOrder"`   // 菜系展示顺序（空 = 用内置顺序）
	Meals          []MealSlot `json:"meals"`          // 空表示使用默认三餐
	Roster         Roster     `json:"roster"`         // 每周掌勺 / 洗碗排班
	Pantry         []string   `json:"pantry"`         // 冰箱里现有的食材，供智能推荐匹配
	// PantryAdded 记录每样食材加入冰箱的日期（YYYY-MM-DD），用于「放了几天」提醒。
	// 与 Pantry 分开存：老数据没有这个字段也能正常工作，食材本身仍是纯字符串数组。
	PantryAdded map[string]string `json:"pantryAdded,omitempty"`
	// PantryStock 记录数量、单位与保质期，用于整周采购自动扣除库存和临期提醒。
	PantryStock map[string]PantryStock `json:"pantryStock,omitempty"`
	// ShoppingMarks 跨设备共享采购状态，key 为归一化后的食材名。
	ShoppingMarks map[string]ShoppingMark `json:"shoppingMarks,omitempty"`
	AutoBackup    bool                    `json:"autoBackup"` // 每天 03:30 自动打包到 data/backups/
	BackupKeep    int                     `json:"backupKeep"` // 自动备份保留份数，0 视为默认 7
	// ImageMode 控制配图清晰度：auto=局域网原图/外网缩略图（默认），full=总是原图，thumb=总是缩略图。
	ImageMode string `json:"imageMode,omitempty"`
	// AutoCheckUpdate 是否在设置页每天自动检查新版本，默认 true（空值视为开）。
	AutoCheckUpdate *bool `json:"autoCheckUpdate,omitempty"`
	// ProxyURL 在线更新走 HTTP/SOCKS5 代理，例如 http://10.10.10.254:7890；空表示直连。
	// 只影响更新下载，业务接口不受影响。
	ProxyURL string `json:"proxyUrl,omitempty"`
	// FamilyID 是「多家庭隔离」第一阶段的地基：只用于备份命名空间，
	// 当前所有数据仍共用一份 JSON，不做分仓，旧数据零值兼容。
	FamilyID  string `json:"familyId,omitempty"`
	UpdatedAt string `json:"updatedAt"`
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
	{"sichuan", "川渝菜", "🌶️", "/dishes/sichuan.webp", "麻辣鲜香，巴蜀一锅", "#f3d9cd"},
	{"xinjiang", "新疆菜", "🍢", "/dishes/xinjiang.webp", "豪迈香辣，西域风味", "#e8d5b8"},
	{"cantonese", "粤菜", "🥢", "/dishes/cantonese.webp", "清鲜精致，讲究火候", "#dce9dd"},
	{"shandong", "鲁菜", "🐟", "/dishes/shandong.webp", "咸鲜醇厚，擅用高汤", "#f0e2c8"},
	{"dongbei", "东北菜", "🥘", "/dishes/dongbei.webp", "量大实在，咸鲜下饭", "#efe3d0"},
	{"jiangsu", "苏菜", "🍶", "/dishes/jiangsu.webp", "清雅细腻，刀工见长", "#e3e0f0"},
	{"benbang", "本帮菜", "🍜", "/dishes/benbang.webp", "浓油赤酱，甜咸适口", "#f0ddd3"},
	{"zhejiang", "浙菜", "🍃", "/dishes/zhejiang.webp", "清爽软嫩，原味为本", "#dceadf"},
	{"fujian", "闽菜", "🍲", "/dishes/fujian.webp", "汤鲜味醇，山海相融", "#f4dfcd"},
	{"hunan", "湘菜", "🔥", "/dishes/hunan.webp", "香辣浓烈，下饭首选", "#f5d5cb"},
	{"anhui", "徽菜", "🏔️", "/dishes/anhui.webp", "重油重色，山野风味", "#e6ddcd"},
	{"homestyle", "家常菜", "🍳", "/dishes/homestyle.webp", "天天吃也不腻的老味道", "#f6e6c9"},
	{"soup", "汤羹", "🥣", "/dishes/soup.webp", "一碗热汤，饭才圆满", "#dfe9ec"},
	{"staple", "主食", "🍚", "/dishes/staple.webp", "米面馒头，管饱担当", "#eee4cf"},
	{"cold", "凉菜", "🥗", "/dishes/cold.webp", "爽口开胃，先上桌", "#e1ebd8"},
	{"snack", "小吃", "🥟", "/dishes/snack.webp", "解馋小食，随手一口", "#f3e6d0"},
	{"breakfast", "早餐", "🥞", "/dishes/breakfast.webp", "一天从好好吃饭开始", "#f7e7d2"},
	{"dessert", "甜品饮品", "🍰", "/dishes/dessert.webp", "饭后那口甜", "#f2dce2"},
	{"western", "西餐烘焙", "🍝", "/dishes/western.webp", "偶尔换换口味", "#e4e2ee"},
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

// OrderedCuisines 按用户自定义顺序返回分类；没出现在 order 里的保持内置顺序，排在后面。
// order 为空时直接返回内置顺序，老数据零值兼容。
func OrderedCuisines(order []string) []Cuisine {
	if len(order) == 0 {
		return Cuisines
	}
	rank := make(map[string]int, len(order))
	for i, k := range order {
		rank[k] = i
	}
	out := cloneSlice(Cuisines)
	sort.SliceStable(out, func(i, j int) bool {
		ri, oi := rank[out[i].Key]
		rj, oj := rank[out[j].Key]
		switch {
		case oi && oj:
			return ri < rj
		case oi:
			return true // 用户指定过顺序的排前面
		case oj:
			return false
		default:
			return false // 都没指定：SliceStable 会保持内置相对顺序
		}
	})
	return out
}
