package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 应用内定时备份：每天 03:30 把四份 JSON 与 uploads 打包到 data/backups/，
// 按设置保留最近 N 份。Docker 里 /data 已经挂到 NAS，等于自动备份到 NAS 目录，
// 不需要在宿主机额外配 cron，也不会因为容器重建丢配置。
const (
	backupHour   = 3
	backupMinute = 30
	defaultKeep  = 7
)

// backupDir 支持 MENU_BACKUP_DIR：在 docker-compose 里把 NAS 上另一个目录挂进来，
// 备份就与运行数据物理分开；不设时退回 /data/backups（同一 NAS 挂载内的子目录）。
func (s *Server) backupDir() string {
	if d := strings.TrimSpace(os.Getenv("MENU_BACKUP_DIR")); d != "" {
		return d
	}
	return filepath.Join(s.st.Dir(), "backups")
}

// familyID 是「多家庭隔离」第一阶段的地基：备份文件名带家庭标识，
// 将来按 familyId 分目录时现有数据可以直接对上，不需要改名或迁移。
func (s *Server) familyID() string {
	id := strings.TrimSpace(s.st.GetSettings().FamilyID)
	if id == "" {
		return "default"
	}
	return id
}

func (s *Server) writeBackupZip(w io.Writer) error {
	zw := zip.NewWriter(w)
	root := s.st.Dir()
	for _, name := range jsonNames {
		b, err := os.ReadFile(s.st.Path(name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		entry, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(b); err != nil {
			return err
		}
	}
	// 与手动导出保持同一口径：只收当前菜品和订单实际引用的图片，
	// 避免历史改图留下的孤儿文件把每日备份重新撑到几百 MB。
	used := s.usedImages()
	walkErr := filepath.WalkDir(filepath.Join(root, "uploads"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !used[d.Name()] {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		if err := addBackupFile(zw, filepath.ToSlash(rel), p); err != nil {
			return err
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		thumbPath := filepath.Join(filepath.Dir(p), "thumb", base+".jpg")
		name := filepath.ToSlash(filepath.Join(filepath.Dir(rel), "thumb", base+".jpg"))
		if err := addBackupFile(zw, name, thumbPath); err != nil {
			return err
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	return zw.Close()
}

func addBackupFile(zw *zip.Writer, name, source string) error {
	f, err := os.Open(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	entry, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, f)
	return err
}

func (s *Server) runBackup() (string, int64, error) {
	// 自动任务和多台设备的手动备份可能同时触发。串行化后既避免同名文件
	// 互相覆盖，也避免两个 ZIP 同时读写导致的损坏。
	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	now := time.Now()
	suffix := strings.TrimPrefix(randomName(), "img")
	name := fmt.Sprintf("family-menu-%s-%s-%s.zip", s.familyID(), now.Format("20060102-150405"), suffix)
	path := filepath.Join(dir, name)
	tmp, err := os.CreateTemp(dir, ".family-menu-*.zip.tmp")
	if err != nil {
		return "", 0, err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	defer cleanup()

	writeErr := s.writeBackupZip(tmp)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		if writeErr != nil {
			return "", 0, writeErr
		}
		if syncErr != nil {
			return "", 0, syncErr
		}
		return "", 0, closeErr
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	s.pruneBackups()
	return name, info.Size(), nil
}

func (s *Server) pruneBackups() {
	keep := s.st.GetSettings().BackupKeep
	if keep <= 0 {
		keep = defaultKeep
	}
	if keep > 60 {
		keep = 60
	}
	entries, err := os.ReadDir(s.backupDir())
	if err != nil {
		return
	}
	type item struct {
		name string
		mod  time.Time
	}
	var list []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "family-menu-") || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		info, err := e.Info()
		if err == nil {
			list = append(list, item{e.Name(), info.ModTime()})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].mod.After(list[j].mod) })
	for i := keep; i < len(list); i++ {
		_ = os.Remove(filepath.Join(s.backupDir(), list[i].name))
	}
}

func (s *Server) hasBackupToday(day string) bool {
	entries, _ := os.ReadDir(s.backupDir())
	prefix := fmt.Sprintf("family-menu-%s-%s", s.familyID(), day)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			return true
		}
	}
	return false
}

func (s *Server) startAutoBackup() {
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		lastDay := ""
		for {
			select {
			case <-s.stopCh:
				return
			case <-tick.C:
			}
			// 后台 goroutine 无 net/http 的 recover 保护：单次备份 panic 会
			// 终结整个进程。这里兜住，下一分钟照常继续。
			func() {
				defer func() {
					if v := recover(); v != nil {
						log.Printf("自动备份 panic（已恢复）: %v", v)
					}
				}()
				s.autoBackupTick(&lastDay)
			}()
		}
	}()
}

// autoBackupTick 执行一次定时备份检查；lastDay 记录当天是否已处理。
func (s *Server) autoBackupTick(lastDay *string) {
	if !s.st.GetSettings().AutoBackup {
		return
	}
	now := time.Now()
	if now.Hour() != backupHour || now.Minute() != backupMinute {
		return
	}
	day := now.Format("20060102")
	if day == *lastDay || s.hasBackupToday(day) {
		*lastDay = day
		return
	}
	if _, _, err := s.runBackup(); err == nil {
		*lastDay = day
	}
}

func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) {
	entries, _ := os.ReadDir(s.backupDir())
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"name": e.Name(),
			"size": info.Size(),
			"at":   info.ModTime().Format("2006-01-02 15:04"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["at"].(string) > out[j]["at"].(string) })
	jsonOK(w, out)
}

func (s *Server) runBackupNow(w http.ResponseWriter, r *http.Request) {
	name, size, err := s.runBackup()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "备份失败："+err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "name": name, "size": size})
}
