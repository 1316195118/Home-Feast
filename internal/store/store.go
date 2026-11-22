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

// ErrLastMember 由 DeleteMember 在同一个写事务内返回，防止两个并发删除把成员清空。
var ErrLastMember = errors.New("至少保留一位家庭成员")

// configFile 判断某文件是否属于配置目录（目前只有 settings.json）。
func configFile(name string) bool { return name == settingsFile }

// Store 是内存中的数据镜像，所有公开方法都可并发调用。
type Store struct {
	mu         sync.RWMutex
	revision   uint64 // 内存数据修订号；每次可见数据成功变化后递增，供 HTTP 快照缓存失效
	dir        string // 用户数据目录（JSON / uploads / backups）
	configDir  string // 配置目录，与 data 平级（家庭设置 settings.json）
	dishes     []Dish
	members    []Member
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

// pathOf 返回逻辑文件名的真实磁盘路径：配置类进独立 configDir，其余进 dir。
func (s *Store) pathOf(name string) string {
	if configFile(name) {
		return filepath.Join(s.configDir, name)
	}
	return filepath.Join(s.dir, name)
}

// Open 打开（必要时创建）数据目录与配置目录。文件损坏时直接报错，绝不悄悄覆盖用户数据。
func Open(dir, cfgDir string) (*Store, error) {
	if dir == "" {
		dir = "data"
	}
	if cfgDir == "" {
		cfgDir = "config"
	}
	if err := os.MkdirAll(filepath.Join(dir, "uploads"), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建配置目录: %w", err)
	}
	s := &Store{dir: dir, configDir: cfgDir, plans: make(map[string]Plan), pins: make(map[string]pinRecord)}

	// 迁移旧位置：data/settings.json 与 data/config/settings.json → 独立 config/settings.json。
	newSettings := s.pathOf(settingsFile)
	for _, old := range []string{
		filepath.Join(dir, settingsFile),
		filepath.Join(dir, "config", settingsFile),
	} {
		if filepath.Clean(old) == filepath.Clean(newSettings) {
			continue
		}
		if _, err := os.Stat(old); err == nil {
			if _, err = os.Stat(newSettings); errors.Is(err, os.ErrNotExist) {
				b, readErr := os.ReadFile(old)
				if readErr != nil {
					return nil, fmt.Errorf("迁移旧设置 %s: %w", old, readErr)
				}
				if writeErr := writeBytesAtomic(newSettings, b, 0o644); writeErr != nil {
					// 新文件没写成功时绝不删旧文件，避免迁移把唯一一份设置弄丢。
					return nil, fmt.Errorf("迁移旧设置到 %s: %w", newSettings, writeErr)
				}
			} else if err != nil {
				return nil, fmt.Errorf("检查新设置文件 %s: %w", newSettings, err)
			}
			// 到这里新设置已经存在且可读，旧位置删除失败也不影响本次启动。
			_ = os.Remove(old)
		}
	}
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
		if err := readJSON(s.pathOf(item.file), item.dest); err != nil {
			var ce *corruptError
			if errors.As(err, &ce) {
				// 文件损坏（不是权限问题）：留档一份再当空数据，服务照常起来。
				// 直接拒绝启动会让全家连页面都打不开，比丢这一份数据更糟。
				stamp := time.Now().Format("20060102-150405")
				keep := fmt.Sprintf("%s.corrupt-%s", ce.path, stamp)
				if renameErr := os.Rename(ce.path, keep); renameErr == nil {
					fmt.Fprintf(os.Stderr, "⚠ %s 已损坏，留档为 %s，本次用空数据启动\n", item.file, keep)
				} else {
					fmt.Fprintf(os.Stderr, "⚠ %s 已损坏且无法留档: %v\n", item.file, renameErr)
				}
				continue
			}
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
	s.repair()
	return s, nil
}

// repair 给加载进来的数据做一次体检，修掉能自动判断的脏数据：
//   - 菜品 / 成员：空 ID、重复 ID、空名字丢弃；未知菜系回退家常；辣度与耗时夹紧
//   - 每日安排：日期格式非法丢弃；分工里指向已删成员的 id 移除
//
// 只修「确定是错的」，不猜用户意图；有改动才写回磁盘。
func (s *Store) repair() {
	changedDishes, changedMembers := false, false

	seen := make(map[string]bool, len(s.dishes))
	dishes := make([]Dish, 0, len(s.dishes))
	for _, d := range s.dishes {
		if d.ID == "" || seen[d.ID] {
			changedDishes = true
			continue
		}
		seen[d.ID] = true
		if !knownCuisineKey(d.Cuisine) {
			d.Cuisine = "homestyle"
			changedDishes = true
		}
		if d.Spicy < 0 {
			d.Spicy = 0
			changedDishes = true
		}
		if d.Spicy > 3 {
			d.Spicy = 3
			changedDishes = true
		}
		if d.Minutes < 0 {
			d.Minutes = 0
			changedDishes = true
		}
		if d.Minutes > 600 {
			d.Minutes = 600
			changedDishes = true
		}
		dishes = append(dishes, d)
	}
	s.dishes = dishes

	seenMember := make(map[string]bool, len(s.members))
	members := make([]Member, 0, len(s.members))
	for _, m := range s.members {
		if m.ID == "" || m.Name == "" || seenMember[m.ID] {
			changedMembers = true
			continue
		}
		seenMember[m.ID] = true
		if m.Avatar == "" {
			m.Avatar = "🙂"
			changedMembers = true
		}
		if len(m.Color) != 7 || m.Color[0] != '#' {
			m.Color = paletteFor(len(members))
			changedMembers = true
		}
		members = append(members, m)
	}
	s.members = members

	changedPlans := false
	for date, p := range s.plans {
		if !validDateKey(date) {
			delete(s.plans, date)
			changedPlans = true
			continue
		}
		// 旧数据是「一天一份」：迁移成每一餐都相同的安排，否则按餐读取时整天都是空的。
		// 迁移后顶层仍保留，作为「全天」语义与旧前端兜底。
		if len(p.MealDuties) == 0 && (len(p.ChefIDs) > 0 || len(p.DishwasherIDs) > 0) {
			slots := effectiveMeals(s.settings.Meals)
			if len(slots) > 0 {
				p.MealDuties = make(map[string]MealDuty, len(slots))
				for _, m := range slots {
					p.MealDuties[m.Key] = MealDuty{
						ChefIDs:       append([]string(nil), p.ChefIDs...),
						DishwasherIDs: append([]string(nil), p.DishwasherIDs...),
					}
				}
				changedPlans = true
			}
		}
		p.ChefIDs = keepKnown(p.ChefIDs, seenMember)
		p.DishwasherIDs = keepKnown(p.DishwasherIDs, seenMember)
		p.PKShown = keepKnown(p.PKShown, seenMember)
		for meal, duty := range p.MealDuties {
			before := len(duty.ChefIDs) + len(duty.DishwasherIDs)
			duty.ChefIDs = keepKnown(duty.ChefIDs, seenMember)
			duty.DishwasherIDs = keepKnown(duty.DishwasherIDs, seenMember)
			if len(duty.ChefIDs)+len(duty.DishwasherIDs) != before {
				changedPlans = true
			}
			if len(duty.ChefIDs) == 0 && len(duty.DishwasherIDs) == 0 {
				delete(p.MealDuties, meal)
			} else {
				p.MealDuties[meal] = duty
			}
		}
		if p.Dice != nil {
			for id := range p.Dice {
				if !seenMember[id] {
					delete(p.Dice, id)
					changedPlans = true
				}
			}
		}
		p.NormalizeDuty()
		s.plans[date] = p
	}

	if changedDishes {
		_ = s.writeJSON(dishesFile, s.dishes)
	}
	if changedMembers {
		_ = s.writeJSON(membersFile, s.members)
	}
	if changedPlans {
		list := make([]Plan, 0, len(s.plans))
		for _, p := range s.plans {
			list = append(list, p)
		}
		_ = s.writeJSON(plansFile, list)
	}
}

func knownCuisineKey(key string) bool {
	for _, c := range Cuisines {
		if c.Key == key {
			return true
		}
	}
	return false
}

func paletteFor(i int) string {
	palette := []string{"#d9a066", "#cf8b9b", "#7fa98d", "#8d9bc4", "#c59a6d", "#a98bb5"}
	return palette[i%len(palette)]
}

// keepKnown 过滤掉指向不存在成员的 id，保持原顺序。
func keepKnown(ids []string, known map[string]bool) []string {
	if len(ids) == 0 {
		return ids
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if known[id] {
			out = append(out, id)
		}
	}
	return out
}

func validDateKey(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// Dir 返回数据目录，供上传与备份使用。
func (s *Store) Dir() string { return s.dir }

// Revision 返回当前内存数据修订号。修订号只保证单调递增，不参与持久化。
func (s *Store) Revision() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.revision
}

// markChangedLocked 必须在持有写锁、且内存状态已经变更后调用。
func (s *Store) markChangedLocked() {
	s.revision++
}

// Path 返回某个逻辑文件名的真实磁盘路径（配置类进 config/，其余进根目录）。
// 供备份导出/导入使用，使 zip 里保持扁平文件名、落盘时自动分流。
func (s *Store) Path(name string) string { return s.pathOf(name) }

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
		if err := readJSON(s.pathOf(item.file), item.dest); err != nil {
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
	// 导入的数据同样过一遍体检：备份可能来自旧版本，或被人手改过。
	s.repair()
	s.markChangedLocked()
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
	s.markChangedLocked()
	return nil
}

// ---------- 菜品 ----------

func (s *Store) ListDishes() []Dish {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneDishes(s.dishes)
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
	Rating      *int      `json:"rating"`
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
	s.markChangedLocked()
	return cloneDish(d), nil
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
	s.markChangedLocked()
	return cloneDish(d), nil
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
	if p.Rating != nil {
		v := *p.Rating
		if v < 0 {
			v = 0
		}
		if v > 3 {
			v = 3
		}
		d.Rating = v
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
	s.markChangedLocked()
	return nil
}

// ---------- 家庭成员 ----------

func (s *Store) ListMembers() []Member {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloneMembersLocked()
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
	s.markChangedLocked()
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
	s.markChangedLocked()
	_, m.HasPin = s.pins[m.ID]
	return cloneMember(m), nil
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
	s.markChangedLocked()
	_, m.HasPin = s.pins[m.ID]
	return cloneMember(m), nil
}

func (s *Store) DeleteMember(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.members) <= 1 {
		return ErrLastMember
	}
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
	s.markChangedLocked()
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
		s.markChangedLocked()
	}

	// 分工与骰子记录里若还留着这个人，一并清掉：否则首页会显示一个已经不在的成员。
	// 历史订单不动——那里存的是下单时的名字与头像快照，删人后仍应保留记录。
	plansChanged := false
	nextPlans := make(map[string]Plan, len(s.plans))
	for date, p := range s.plans {
		p = clonePlan(p)
		before := len(p.ChefIDs) + len(p.DishwasherIDs) + len(p.PKShown)
		for _, duty := range p.MealDuties {
			before += len(duty.ChefIDs) + len(duty.DishwasherIDs)
		}
		if p.Dice != nil {
			if _, has := p.Dice[id]; has {
				delete(p.Dice, id)
				plansChanged = true
			}
		}
		p.ChefIDs = removeID(p.ChefIDs, id)
		p.DishwasherIDs = removeID(p.DishwasherIDs, id)
		p.PKShown = removeID(p.PKShown, id)
		for meal, duty := range p.MealDuties {
			duty.ChefIDs = removeID(duty.ChefIDs, id)
			duty.DishwasherIDs = removeID(duty.DishwasherIDs, id)
			if len(duty.ChefIDs) == 0 && len(duty.DishwasherIDs) == 0 {
				delete(p.MealDuties, meal)
			} else {
				p.MealDuties[meal] = duty
			}
		}
		after := len(p.ChefIDs) + len(p.DishwasherIDs) + len(p.PKShown)
		for _, duty := range p.MealDuties {
			after += len(duty.ChefIDs) + len(duty.DishwasherIDs)
		}
		if after != before {
			plansChanged = true
		}
		p.NormalizeDuty()
		nextPlans[date] = p
	}
	if plansChanged {
		list := make([]Plan, 0, len(nextPlans))
		for _, p := range nextPlans {
			list = append(list, p)
		}
		if err := s.writeJSON(plansFile, list); err != nil {
			return err
		}
		s.plans = nextPlans
		s.markChangedLocked()
	}
	return nil
}

func removeID(ids []string, id string) []string {
	if len(ids) == 0 {
		return ids
	}
	out := make([]string, 0, len(ids))
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
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
	p = clonePlan(p)
	if dropExpired(&p, now, effectiveMeals(s.settings.Meals)) {
		_ = s.persistPlans(p)
	}
}

func (s *Store) GetPlan(date string) Plan {
	s.expireToday(date)
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.plans[date]; ok {
		p = clonePlan(p)
		if p.Orders == nil {
			p.Orders = []Order{}
		}
		p.NormalizeDuty() // 旧数据只有单人字段时，这里补出数组给前端
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
		p = clonePlan(p)
		if p.Orders == nil {
			p.Orders = []Order{}
		}
		p.NormalizeDuty()
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// PlanCount 只返回数量，避免 bootstrap 为了一个数字复制并排序整份历史。
func (s *Store) PlanCount() int {
	s.expireToday(time.Now().Format("2006-01-02"))
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.plans)
}

// planList 返回全部计划，供写盘使用。
func (s *Store) planList() []Plan {
	list := make([]Plan, 0, len(s.plans))
	for _, p := range s.plans {
		list = append(list, clonePlan(p))
	}
	return list
}

// SavePlan 只改「今天谁掌勺 / 谁洗碗 / 备注」，保留已点的菜。
// 用 PlanPatch 的指针区分「没传」与「传了空值要清空」。
func (s *Store) SavePlan(date string, patch PlanPatch) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := clonePlan(s.plans[date])
	p.Date = date
	if p.Orders == nil {
		p.Orders = []Order{}
	}
	// 按餐分工：本次只改这一餐，其他餐次与顶层保持原样
	if patch.DutyMeal != "" {
		if p.MealDuties == nil {
			p.MealDuties = make(map[string]MealDuty)
		}
		md := p.MealDuties[patch.DutyMeal]
		if patch.ChefIDs != nil {
			md.ChefIDs = append([]string(nil), (*patch.ChefIDs)...)
		}
		if patch.DishwasherIDs != nil {
			md.DishwasherIDs = append([]string(nil), (*patch.DishwasherIDs)...)
		}
		if len(md.ChefIDs) == 0 && len(md.DishwasherIDs) == 0 {
			delete(p.MealDuties, patch.DutyMeal) // 空名单直接删掉，别留空对象
		} else {
			p.MealDuties[patch.DutyMeal] = md
		}
		p.UpdatedAt = now()
		if err := s.writeJSON(plansFile, s.planList()); err != nil {
			return Plan{}, err
		}
		s.plans[date] = p
		s.markChangedLocked()
		return clonePlan(p), nil
	}

	if patch.ChefIDs != nil {
		p.ChefIDs = append([]string(nil), (*patch.ChefIDs)...)
		// 先清空单值字段：否则 NormalizeDuty 看到「数组空 + 单值有值」会把旧人补回来，
		// 表现为「取消认领」无效。
		p.ChefID = ""
	} else if patch.ChefID != nil {
		// 旧调用（单人）等价于把数组设成单元素
		if *patch.ChefID == "" {
			p.ChefIDs = nil
		} else {
			p.ChefIDs = []string{*patch.ChefID}
		}
	}
	if patch.DishwasherIDs != nil {
		p.DishwasherIDs = append([]string(nil), (*patch.DishwasherIDs)...)
		p.DishwasherID = ""
	} else if patch.DishwasherID != nil {
		if *patch.DishwasherID == "" {
			p.DishwasherIDs = nil
		} else {
			p.DishwasherIDs = []string{*patch.DishwasherID}
		}
	}
	p.NormalizeDuty()
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
	return clonePlan(p), nil
}

func (s *Store) AddOrder(date string, o Order) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := clonePlan(s.plans[date])
	p.Date = date
	if p.Orders == nil {
		p.Orders = []Order{}
	}
	// 同一天同一道菜只记一条：连点、多端并发、旧客户端重试都不会在今日菜单里堆出重复行。
	// 返回当前 plan 而不是报错，保证接口幂等。
	for _, existing := range p.Orders {
		if existing.DishID == o.DishID {
			return clonePlan(p), nil
		}
	}
	o.ID = newID("o")
	o.CreatedAt = now()
	p.Orders = append(p.Orders, o)
	p.UpdatedAt = now()
	if err := s.persistPlans(p); err != nil {
		return Plan{}, err
	}
	return clonePlan(p), nil
}

func (s *Store) RemoveOrder(date, orderID string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[date]
	if !ok {
		return Plan{}, errors.New("这天还没有点菜")
	}
	p = clonePlan(p)
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
	return clonePlan(p), nil
}

func (s *Store) ToggleOrder(date, orderID string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.plans[date]
	if !ok {
		return Plan{}, errors.New("这天还没有点菜")
	}
	p = clonePlan(p)
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
	return clonePlan(p), nil
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
	s.markChangedLocked()
	return nil
}

// ---------- 设置 ----------

func (s *Store) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSettings(s.settings)
}

func (s *Store) SaveSettings(v Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v = cloneSettings(v)
	v.UpdatedAt = now()
	if err := s.writeJSON(settingsFile, v); err != nil {
		return Settings{}, err
	}
	s.settings = v
	s.markChangedLocked()
	return cloneSettings(v), nil
}

// ---------- 底层文件读写 ----------

// corruptError 表示文件存在、也能读，但内容不是合法 JSON。
// 与「读不了」（权限 / IO 错误）区分开：前者可以留档后用空数据启动，后者必须报错。
type corruptError struct {
	path string
	err  error
}

func (e *corruptError) Error() string {
	return fmt.Sprintf("%s 内容损坏: %v", e.path, e.err)
}

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
	if err := json.Unmarshal(b, dest); err != nil {
		return &corruptError{path: path, err: err}
	}
	return nil
}

func (s *Store) writeJSON(name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	target := s.pathOf(name)
	return writeBytesAtomic(target, raw, 0o644)
}

// writeBytesAtomic 在同目录写临时文件、fsync 后再原子替换。
// 迁移旧配置和普通 JSON 保存共用这条路径，写失败时不会碰原文件。
func writeBytesAtomic(target string, raw []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = f.Write(raw); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err = os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func now() string { return time.Now().Format(time.RFC3339) }

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(buf)
}
