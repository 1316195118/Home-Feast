// Package store 用纯 JSON 文件保存全部数据，不依赖数据库或外部服务。
// 每次写入都走「临时文件 → 落盘 → 原子改名」，断电最多回到上一份完整文件。
package store

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"familymenu/internal/seed"
)

const (
	dishesFile   = "dishes.json"
	membersFile  = "members.json"
	plansFile    = "plans.json"
	settingsFile = "settings.json"
	// 访问口令单独存放：不进 settings.json，避免随 bootstrap / 备份 zip 泄露。
	accessFile = "access.json"
	// 成员密码哈希单独存放，同样不进备份 zip，也不随 bootstrap 下发。
	// members.json 里只有 hasPin 这个计算出来的布尔值，不含任何密码材料。
	pinsFile = "pins.json"
)

// Store 是内存中的数据镜像，所有公开方法都可并发调用。
type Store struct {
	mu       sync.RWMutex
	dir      string
	dishes   []Dish
	members  []Member
	plans      map[string]Plan
	settings   Settings
	accessCode string
	pins       map[string]pinRecord
}

// accessFileData 是 access.json 的结构。
type accessFileData struct {
	Code string `json:"code"`
}

// pinRecord 只存随机盐与哈希，绝不存明文；pins.json 也不在备份白名单里。
type pinRecord struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}

func hashPin(salt, pin string) string {
	sum := sha256.Sum256([]byte(salt + "\x00" + pin))
	return hex.EncodeToString(sum[:])
}

func newSalt() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// Open 打开（必要时创建）数据目录。文件损坏时直接报错，绝不悄悄覆盖用户数据。
func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(filepath.Join(dir, "uploads"), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	s := &Store{dir: dir, plans: make(map[string]Plan), pins: make(map[string]pinRecord)}

	var planList []Plan
	for _, item := range []struct {
		file string
		dest any
	}{
		{dishesFile, &s.dishes},
		{membersFile, &s.members},
		{plansFile, &planList},
		{settingsFile, &s.settings},
	} {
		if err := readJSON(filepath.Join(dir, item.file), item.dest); err != nil {
			return nil, fmt.Errorf("读取 %s 失败: %w", item.file, err)
		}
	}
	for _, p := range planList {
		if p.Date != "" {
			s.plans[p.Date] = p
		}
	}
	var access accessFileData
	if err := readJSON(filepath.Join(dir, accessFile), &access); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", accessFile, err)
	}
	s.accessCode = access.Code
	// pins.json 不存在时保持空 map；它不在备份白名单里，恢复备份不会清掉现有密码。
	var pins map[string]pinRecord
	if err := readJSON(filepath.Join(dir, pinsFile), &pins); err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", pinsFile, err)
	}
	if pins == nil {
		pins = make(map[string]pinRecord)
	}
	s.pins = pins

	// 首次运行：写入内置菜品、示例成员和默认设置。
	if len(s.dishes) == 0 {
		for _, item := range seed.BuiltinDishes {
			s.dishes = append(s.dishes, Dish{
				ID: newID("d"), Name: item.Name, Cuisine: item.Cuisine,
				Desc: item.Desc, Tags: item.Tags, Spicy: item.Spicy,
				Minutes: item.Minutes, Builtin: true, CreatedAt: now(),
			})
		}
		if err := s.writeJSON(dishesFile, s.dishes); err != nil {
			return nil, err
		}
	}
	if len(s.members) == 0 {
		for _, item := range seed.StarterMembers {
			s.members = append(s.members, Member{
				ID: newID("u"), Name: item.Name, Avatar: item.Avatar,
				Color: item.Color, CreatedAt: now(),
			})
		}
		if err := s.writeJSON(membersFile, s.members); err != nil {
			return nil, err
		}
	}
	if s.settings.FamilyName == "" {
		s.settings = Settings{FamilyName: "我们家", UpdatedAt: now()}
	}
	// 旧 settings.json 没有自动备份字段：首次升级默认开启、保留 7 份。
	// 用户主动关闭后 BackupKeep 仍是 7，不会被这段逻辑重新打开。
	if s.settings.BackupKeep == 0 {
		s.settings.AutoBackup = true
		s.settings.BackupKeep = 7
		if err := s.writeJSON(settingsFile, s.settings); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Dir 返回数据目录，供上传与备份使用。
func (s *Store) Dir() string { return s.dir }

// Reload 在导入备份后重新把磁盘内容读进内存。
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		dishes   []Dish
		members  []Member
		planList []Plan
		settings Settings
	)
	for _, item := range []struct {
		file string
		dest any
	}{
		{dishesFile, &dishes}, {membersFile, &members},
		{plansFile, &planList}, {settingsFile, &settings},
	} {
		if err := readJSON(filepath.Join(s.dir, item.file), item.dest); err != nil {
			return err
		}
	}
	s.dishes, s.members, s.settings = dishes, members, settings
	s.plans = make(map[string]Plan)
	for _, p := range planList {
		s.plans[p.Date] = p
	}
	// 备份 zip 不含 access.json，恢复后口令保持当前值，不会被清掉。
	var access accessFileData
	if err := readJSON(filepath.Join(s.dir, accessFile), &access); err != nil {
		return err
	}
	s.accessCode = access.Code
	return nil
}

// AccessCode 返回网页里设置的访问口令（环境变量优先，见 api.Server）。
func (s *Store) AccessCode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accessCode
}

// SetAccessCode 写入新口令；空串表示关闭。先落盘再改内存，写失败不影响当前状态。
func (s *Store) SetAccessCode(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeJSON(accessFile, accessFileData{Code: code}); err != nil {
		return err
	}
	s.accessCode = code
	return nil
}

// ---------- 菜品 ----------

func (s *Store) ListDishes() []Dish {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Dish{}, s.dishes...)
}

// DishPatch 是编辑菜品时允许改动的字段，nil 表示保持不变。
type DishPatch struct {
	Name        *string   `json:"name"`
	Cuisine     *string   `json:"cuisine"`
	Desc        *string   `json:"desc"`
	Tags        *[]string `json:"tags"`
	Image       *string   `json:"image"`
	Spicy       *int      `json:"spicy"`
	Minutes     *int      `json:"minutes"`
	Ingredients *[]string `json:"ingredients"`
	Steps       *[]string `json:"steps"`
	Favorite    *bool     `json:"favorite"`
}

func (s *Store) AddDish(d Dish) (Dish, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d.ID = newID("d")
	d.Builtin = false
	d.CreatedAt = now()
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Ingredients == nil {
		d.Ingredients = []string{}
	}
	if d.Steps == nil {
		d.Steps = []string{}
	}
	next := append(append([]Dish{}, s.dishes...), d)
	if err := s.writeJSON(dishesFile, next); err != nil {
		return Dish{}, err
	}
	s.dishes = next
	return d, nil
}

func (s *Store) UpdateDish(id string, p DishPatch) (Dish, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Dish{}, s.dishes...)
	idx := -1
	for i := range next {
		if next[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Dish{}, errors.New("菜品不存在")
	}
	d := next[idx]
	applyPatch(&d, p)
	next[idx] = d
	if err := s.writeJSON(dishesFile, next); err != nil {
		return Dish{}, err
	}
	s.dishes = next
	return d, nil
}

func applyPatch(d *Dish, p DishPatch) {
	if p.Name != nil {
		d.Name = *p.Name
	}
	if p.Cuisine != nil {
		d.Cuisine = *p.Cuisine
	}
	if p.Desc != nil {
		d.Desc = *p.Desc
	}
	if p.Tags != nil {
		d.Tags = *p.Tags
	}
	if p.Image != nil {
		d.Image = *p.Image
	}
	if p.Spicy != nil {
		d.Spicy = *p.Spicy
	}
	if p.Minutes != nil {
		d.Minutes = *p.Minutes
	}
	if p.Ingredients != nil {
		d.Ingredients = *p.Ingredients
		if d.Ingredients == nil {
			d.Ingredients = []string{}
		}
	}
	if p.Steps != nil {
		d.Steps = *p.Steps
		if d.Steps == nil {
			d.Steps = []string{}
		}
	}
	if p.Favorite != nil {
		d.Favorite = *p.Favorite
	}
}

func (s *Store) DeleteDish(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Dish, 0, len(s.dishes))
	found := false
	for _, d := range s.dishes {
		if d.ID == id {
			found = true
			continue
		}
		next = append(next, d)
	}
	if !found {
		return errors.New("菜品不存在")
	}
	if err := s.writeJSON(dishesFile, next); err != nil {
		return err
	}
	s.dishes = next
	return nil
}

// ---------- 家庭成员 ----------

func (s *Store) ListMembers() []Member {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Member{}, s.members...)
	for i := range out {
		_, out[i].HasPin = s.pins[out[i].ID]
	}
	return out
}

// SetMemberPin 设置 / 修改 / 清除成员密码。已设过密码时必须先给对 oldPin，
// 防止别人随手把密码改掉；pin 为空串表示清除。
func (s *Store) SetMemberPin(id, pin, oldPin string) error {
	if n := len([]rune(pin)); pin != "" && (n < 4 || n > 16) {
		return errors.New("密码需要 4–16 个字符")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exists := false
	for _, m := range s.members {
		if m.ID == id {
			exists = true
			break
		}
	}
	if !exists {
		return errors.New("成员不存在")
	}
	current, has := s.pins[id]
	if has && subtle.ConstantTimeCompare(
		[]byte(hashPin(current.Salt, oldPin)), []byte(current.Hash),
	) != 1 {
		return errors.New("当前密码不正确")
	}
	next := make(map[string]pinRecord, len(s.pins)+1)
	for k, v := range s.pins {
		next[k] = v
	}
	if pin == "" {
		delete(next, id)
	} else {
		salt := newSalt()
		next[id] = pinRecord{Salt: salt, Hash: hashPin(salt, pin)}
	}
	if err := s.writeJSON(pinsFile, next); err != nil {
		return err
	}
	s.pins = next
	return nil
}

// VerifyMemberPin 校验成员密码；没设密码的成员直接通过。
func (s *Store) VerifyMemberPin(id, pin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, has := s.pins[id]
	if !has {
		return true
	}
	return subtle.ConstantTimeCompare(
		[]byte(hashPin(rec.Salt, pin)), []byte(rec.Hash),
	) == 1
}

// MemberPatch 是编辑成员时允许改动的字段。
type MemberPatch struct {
	Name   *string   `json:"name"`
	Avatar *string   `json:"avatar"`
	Color  *string   `json:"color"`
	Avoid  *[]string `json:"avoid"`
}

func (s *Store) AddMember(m Member) (Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.ID = newID("u")
	m.CreatedAt = now()
	next := append(append([]Member{}, s.members...), m)
	if err := s.writeJSON(membersFile, next); err != nil {
		return Member{}, err
	}
	s.members = next
	return m, nil
}

func (s *Store) UpdateMember(id string, p MemberPatch) (Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Member{}, s.members...)
	idx := -1
	for i := range next {
		if next[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Member{}, errors.New("成员不存在")
	}
	m := next[idx]
	if p.Name != nil {
		m.Name = *p.Name
	}
	if p.Avatar != nil {
		m.Avatar = *p.Avatar
	}
	if p.Color != nil {
		m.Color = *p.Color
	}
	if p.Avoid != nil {
		m.Avoid = append([]string(nil), (*p.Avoid)...)
	}
	next[idx] = m
	if err := s.writeJSON(membersFile, next); err != nil {
		return Member{}, err
	}
	s.members = next
	return m, nil
}

func (s *Store) DeleteMember(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Member, 0, len(s.members))
	found := false
	for _, m := range s.members {
		if m.ID == id {
			found = true
			continue
		}
		next = append(next, m)
	}
	if !found {
		return errors.New("成员不存在")
	}
	if err := s.writeJSON(membersFile, next); err != nil {
		return err
	}
	s.members = next
	// 成员删掉后顺手清掉他的密码记录，避免 pins.json 里堆积孤立条目
	if _, has := s.pins[id]; has {
		pins := make(map[string]pinRecord, len(s.pins))
		for k, v := range s.pins {
			if k != id {
				pins[k] = v
			}
		}
		if err := s.writeJSON(pinsFile, pins); err != nil {
			return err
		}
		s.pins = pins
	}
	return nil
}

// ---------- 每日安排 ----------

// 餐别的名字与结束时刻由用户在设置里自定义，相关逻辑集中在 meals.go。

// expireToday 在读取前清理当天已过期订单并持久化（调用者不持锁）。
func (s *Store) expireToday(date string) {
	now := time.Now()
	if date != now.Format("2006-01-02") {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[date]
	if !ok {
		return
	}
	if dropExpired(&p, now, effectiveMeals(s.settings.Meals)) {
		_ = s.persistPlans(p)
	}
}

func (s *Store) GetPlan(date string) Plan {
	s.expireToday(date)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.plans[date]; ok {
		if p.Orders == nil {
			p.Orders = []Order{}
		}
		return p
	}
	return Plan{Date: date, Orders: []Order{}}
}

func (s *Store) ListPlans() []Plan {
	s.expireToday(time.Now().Format("2006-01-02"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Plan, 0, len(s.plans))
	for _, p := range s.plans {
		if p.Orders == nil {
			p.Orders = []Order{}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// SavePlan 只改「今天谁掌勺 / 谁洗碗 / 备注」，保留已点的菜。
// 用 PlanPatch 的指针区分「没传」与「传了空值要清空」。
func (s *Store) SavePlan(date string, patch PlanPatch) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.plans[date]
	p.Date = date
	if p.Orders == nil {
		p.Orders = []Order{}
	}
	if patch.ChefID != nil {
		p.ChefID = *patch.ChefID
	}
	if patch.DishwasherID != nil {
		p.DishwasherID = *patch.DishwasherID
	}
	if patch.Note != nil {
		p.Note = *patch.Note
	}
	if patch.Dice != nil {
		// 深拷贝，避免把请求里的 map 直接挂到内存状态上
		next := make(map[string]int, len(*patch.Dice))
		for k, v := range *patch.Dice {
			next[k] = v
		}
		p.Dice = next
	}
	if patch.DicePhase != nil {
		p.DicePhase = *patch.DicePhase
	}
	if patch.PKShown != nil {
		p.PKShown = append([]string(nil), (*patch.PKShown)...)
	}
	p.UpdatedAt = now()
	if err := s.persistPlans(p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

func (s *Store) AddOrder(date string, o Order) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.plans[date]
	p.Date = date
	if p.Orders == nil {
		p.Orders = []Order{}
	}
	// 同一天同一道菜只记一条：连点、多端并发、旧客户端重试都不会在今日菜单里堆出重复行。
	// 返回当前 plan 而不是报错，保证接口幂等。
	for _, existing := range p.Orders {
		if existing.DishID == o.DishID {
			return p, nil
		}
	}
	o.ID = newID("o")
	o.CreatedAt = now()
	p.Orders = append(p.Orders, o)
	p.UpdatedAt = now()
	if err := s.persistPlans(p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

func (s *Store) RemoveOrder(date, orderID string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[date]
	if !ok {
		return Plan{}, errors.New("这天还没有点菜")
	}
	kept := make([]Order, 0, len(p.Orders))
	found := false
	for _, o := range p.Orders {
		if o.ID == orderID {
			found = true
			continue
		}
		kept = append(kept, o)
	}
	if !found {
		return Plan{}, errors.New("找不到这道已点的菜")
	}
	p.Orders = kept
	p.UpdatedAt = now()
	if err := s.persistPlans(p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

func (s *Store) ToggleOrder(date, orderID string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[date]
	if !ok {
		return Plan{}, errors.New("这天还没有点菜")
	}
	found := false
	for i := range p.Orders {
		if p.Orders[i].ID == orderID {
			p.Orders[i].Done = !p.Orders[i].Done
			found = true
			break
		}
	}
	if !found {
		return Plan{}, errors.New("找不到这道已点的菜")
	}
	p.UpdatedAt = now()
	if err := s.persistPlans(p); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// persistPlans 必须在持有写锁时调用。
func (s *Store) persistPlans(changed Plan) error {
	next := make(map[string]Plan, len(s.plans)+1)
	for k, v := range s.plans {
		next[k] = v
	}
	next[changed.Date] = changed
	list := make([]Plan, 0, len(next))
	for _, p := range next {
		if p.Orders == nil {
			p.Orders = []Order{}
		}
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Date < list[j].Date })
	if err := s.writeJSON(plansFile, list); err != nil {
		return err
	}
	s.plans = next
	return nil
}

// ---------- 设置 ----------

func (s *Store) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *Store) SaveSettings(v Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.UpdatedAt = now()
	if err := s.writeJSON(settingsFile, v); err != nil {
		return Settings{}, err
	}
	s.settings = v
	return v, nil
}

// ---------- 底层文件读写 ----------

func readJSON(path string, dest any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	return json.Unmarshal(b, dest)
}

func (s *Store) writeJSON(name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := filepath.Join(s.dir, "."+name+".tmp")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, name))
}

func now() string { return time.Now().Format(time.RFC3339) }

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(buf)
}
