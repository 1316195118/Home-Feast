package api

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
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
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		entry, err := zw.Create(name)
		if err == nil {
			_, _ = entry.Write(b)
		}
	}
	_ = filepath.WalkDir(filepath.Join(root, "uploads"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		entry, err := zw.Create(filepath.ToSlash(rel))
		if err == nil {
			_, _ = entry.Write(b)
		}
		return nil
	})
	return zw.Close()
}

func (s *Server) runBackup() (string, int64, error) {
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	name := fmt.Sprintf("family-menu-%s-%s.zip", s.familyID(), time.Now().Format("20060102-1504"))
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", 0, err
	}
	writeErr := s.writeBackupZip(f)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return "", 0, writeErr
		}
		return "", 0, closeErr
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
		for range tick.C {
			if !s.st.GetSettings().AutoBackup {
				continue
			}
			now := time.Now()
			if now.Hour() != backupHour || now.Minute() != backupMinute {
				continue
			}
			day := now.Format("20060102")
			if day == lastDay || s.hasBackupToday(day) {
				lastDay = day
				continue
			}
			if _, _, err := s.runBackup(); err == nil {
				lastDay = day
			}
		}
	}()
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
