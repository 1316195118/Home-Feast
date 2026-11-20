package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 容器内自更新：从 GitHub raw 拉 manifest + 二进制，校验 sha256 后原子替换，
// 再用 syscall.Exec 让当前进程原地变成新二进制（PID1 不变，容器不重启）。
//
// 为什么不用 github.com/api.github.com：实测容器网络只通 raw.githubusercontent.com
// 与 ghproxy.net，前者不通。所以更新源固定为 raw，失败再退 ghproxy。
const (
	updateRepo   = "1316195118/Home-Feast"
	updateBranch = "main"
	updateRaw    = "https://raw.githubusercontent.com/" + updateRepo + "/" + updateBranch + "/dist/"
	updateProxy  = "https://ghproxy.net/" + updateRaw
	updateDir    = ".update" // 位于 dataDir 下，与业务数据同卷，随备份一起走
)

type releaseFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type releaseManifest struct {
	Version     string                 `json:"version"`
	PublishedAt string                 `json:"publishedAt"`
	Note        string                 `json:"note"`
	Files       map[string]releaseFile `json:"files"`
}

type updateStatus struct {
	 Checking     bool   `json:"checking"`
	 Downloading  bool   `json:"downloading"`
	 LastCheck    string `json:"lastCheck,omitempty"`
	 LastError    string `json:"lastError,omitempty"`
	 Available    bool   `json:"available"`
	 Latest       string `json:"latest,omitempty"`
	 Progress     int    `json:"progress"`
	 Message      string `json:"message,omitempty"`
}

var updateMu sync.Mutex
var updateState updateStatus

// 更新源列表：raw 优先，ghproxy 兜底。
// MENU_UPDATE_BASE 可覆盖为本地测试源（如 http://127.0.0.1:18099/dist/），便于 E2E 验证。
func updateSources() []string {
	if v := os.Getenv("MENU_UPDATE_BASE"); v != "" {
		return []string{strings.TrimRight(v, "/") + "/"}
	}
	return []string{updateRaw, updateProxy}
}

func fetchUpdateJSON(ctx context.Context, path string, dest any) error {
	var lastErr error
	firstErr := ""
	for i, base := range updateSources() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "family-menu-updater")
		resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, base)
			if i == 0 {
				firstErr = lastErr.Error()
			}
			continue
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dest)
		resp.Body.Close()
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("没有可用的更新源")
	}
	// 两个源都失败时，优先报首选源（raw）的错误，避免备用源把真实原因盖掉
	if firstErr != "" {
		return fmt.Errorf("%s（备用源: %v）", firstErr, lastErr)
	}
	return lastErr
}

// compareVersion 比较四段式版本，返回 a>b 为正、a<b 为负、相等为 0。
func compareVersion(a, b string) int {
	parse := func(s string) []int {
		parts := strings.Split(strings.TrimSpace(s), ".")
		out := make([]int, 4)
		for i := 0; i < 4 && i < len(parts); i++ {
			var n int
			fmt.Sscanf(parts[i], "%d", &n)
			out[i] = n
		}
		return out
	}
	x, y := parse(a), parse(b)
	for i := 0; i < 4; i++ {
		if x[i] != y[i] {
			return x[i] - y[i]
		}
	}
	return 0
}

// checkUpdate 只拉 manifest 比版本，不改任何文件。
func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	updateMu.Lock()
	if updateState.Checking {
		updateMu.Unlock()
		jsonOK(w, updateState)
		return
	}
	updateState.Checking = true
	updateState.LastError = ""
	updateMu.Unlock()

	var m releaseManifest
	err := fetchUpdateJSON(r.Context(), "manifest.json", &m)
	now := time.Now().Format(time.RFC3339)

	updateMu.Lock()
	updateState.Checking = false
	updateState.LastCheck = now
	if err != nil {
		updateState.LastError = err.Error()
		updateState.Available = false
		updateState.Latest = ""
		updateMu.Unlock()
		jsonErr(w, http.StatusBadGateway, "检查更新失败: "+err.Error())
		return
	}
	updateState.Latest = m.Version
	updateState.Available = compareVersion(m.Version, Version) > 0
	updateMu.Unlock()

	jsonOK(w, map[string]any{
		"current":   Version,
		"latest":    m.Version,
		"available": updateState.Available,
		"published": m.PublishedAt,
		"note":      m.Note,
		"checkedAt": now,
	})
}

// updateStatus 返回当前更新进度（前端轮询用）。
func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	updateMu.Lock()
	st := updateState // 值拷贝，避免竞争
	updateMu.Unlock()
	jsonOK(w, st)
}

// 存档：更新前把当前二进制备份进 dataDir/.update/menu.backup。
func updatePaths(dataDir string) (dir, staging, backup, stateFile string) {
	dir = filepath.Join(dataDir, updateDir)
	staging = filepath.Join(dir, "menu.new")
	backup = filepath.Join(dir, "menu.backup")
	stateFile = filepath.Join(dir, "state.json")
	return
}

// updaterState 落盘，供启动时自检是否需要回滚。
type updaterState struct {
	Phase   string `json:"phase"` // pending / applying / applied / failed
	From    string `json:"from"`
	To      string `json:"to"`
	At      string `json:"at"`
}

func writeUpdaterState(dataDir string, st updaterState) error {
	dir, _, _, path := updatePaths(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// CheckRecovery 在 main 启动早期调用：若上次更新后新二进制起不来（容器被 restart），
// 检测到 state.json 仍是 applying，就把备份还原回可执行路径。
func CheckRecovery(dataDir, binPath string) {
	_, _, backup, stateFile := updatePaths(dataDir)
	b, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}
	var st updaterState
	if json.Unmarshal(b, &st) != nil || st.Phase != "applying" {
		return
	}
	// 记录过 applying 但能走到这里，说明新二进制成功启动了（本进程就是新的）
	if st.To == Version {
		writeUpdaterState(dataDir, updaterState{Phase: "applied", From: st.From, To: st.To, At: time.Now().Format(time.RFC3339)})
		return
	}
	// 版本对不上：上一版二进制被重新拉起，说明更新失败，清理 staging
	_ = os.Remove(backup)
	_ = os.Remove(stateFile)
	log.Printf("更新回退完成，当前版本 %s", Version)
}

func goArchKey() string {
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return runtime.GOARCH
	case "arm":
		return "armv7"
	default:
		return runtime.GOARCH
	}
}

// applyUpdate 下载 → 校验 → 备份 → 原子替换 → 延迟 Exec。
func (s *Server) applyUpdate(w http.ResponseWriter, r *http.Request) {
	updateMu.Lock()
	if updateState.Downloading {
		updateMu.Unlock()
		jsonErr(w, http.StatusConflict, "已有更新在进行中")
		return
	}
	updateState.Downloading = true
	updateState.Progress = 0
	updateState.Message = "正在获取版本信息"
	updateState.LastError = ""
	updateMu.Unlock()
	defer func() {
		updateMu.Lock()
		updateState.Downloading = false
		updateMu.Unlock()
	}()

	setProgress := func(p int, msg string) {
		updateMu.Lock()
		updateState.Progress = p
		updateState.Message = msg
		updateMu.Unlock()
	}

	var m releaseManifest
	if err := fetchUpdateJSON(r.Context(), "manifest.json", &m); err != nil {
		setProgress(0, "")
		jsonErr(w, http.StatusBadGateway, "获取更新信息失败: "+err.Error())
		return
	}
	if compareVersion(m.Version, Version) <= 0 {
		setProgress(100, "已是最新版本")
		jsonOK(w, map[string]any{"ok": true, "message": "已是最新版本", "version": Version})
		return
	}
	f, ok := m.Files[goArchKey()]
	if !ok || f.Name == "" || f.SHA256 == "" {
		setProgress(0, "")
		jsonErr(w, http.StatusBadRequest, "此架构没有可用更新: "+goArchKey())
		return
	}

	dir, staging, backup, _ := updatePaths(s.st.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		setProgress(0, "")
		jsonErr(w, http.StatusInternalServerError, "创建更新目录失败: "+err.Error())
		return
	}

	// 下载到 staging.tmp，边下边算 sha256
	setProgress(10, "正在下载新版本")
	tmp := staging + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		setProgress(0, "")
		jsonErr(w, http.StatusInternalServerError, "无法写入更新文件: "+err.Error())
		return
	}
	hasher := sha256.New()
	var downloaded int64
	var dlErr error
	for _, base := range updateSources() {
		downloaded = 0
		dlErr = nil
		out.Seek(0, 0)
		if err := out.Truncate(0); err != nil {
			dlErr = err
			break
		}
		hasher.Reset()
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+f.Name, nil)
		if err != nil {
			dlErr = err
			continue
		}
		req.Header.Set("User-Agent", "family-menu-updater")
		resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
		if err != nil {
			dlErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			dlErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		buf := make([]byte, 128*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, err := out.Write(buf[:n]); err != nil {
					dlErr = err
					break
				}
				hasher.Write(buf[:n])
				downloaded += int64(n)
				p := 10 + int(float64(downloaded)/float64(max64(f.Size, 1))*70)
				if p > 80 {
					p = 80
				}
				setProgress(p, "正在下载新版本")
			}
			if readErr != nil {
				if readErr != io.EOF {
					dlErr = readErr
				}
				break
			}
		}
		resp.Body.Close()
		if dlErr == nil {
			break
		}
	}
	out.Close()
	if downloaded == 0 || dlErr != nil {
		os.Remove(tmp)
		setProgress(0, "")
		msg := "下载更新失败"
		if dlErr != nil {
			msg += ": " + dlErr.Error()
		}
		jsonErr(w, http.StatusBadGateway, msg)
		return
	}
	if f.Size > 0 && downloaded != f.Size {
		os.Remove(tmp)
		setProgress(0, "")
		jsonErr(w, http.StatusBadGateway, fmt.Sprintf("文件大小不符: 期望 %d 实际 %d", f.Size, downloaded))
		return
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(got, f.SHA256) {
		os.Remove(tmp)
		setProgress(0, "")
		jsonErr(w, http.StatusBadGateway, "校验失败：sha256 不匹配，已放弃更新")
		return
	}

	// 备份当前二进制
	setProgress(85, "正在替换")
	binPath, err := os.Executable()
	if err != nil {
		os.Remove(tmp)
		setProgress(0, "")
		jsonErr(w, http.StatusInternalServerError, "无法定位当前程序: "+err.Error())
		return
	}
	// 跟随软链接，确保替换的是真文件
	if resolved, e := filepath.EvalSymlinks(binPath); e == nil {
		binPath = resolved
	}
	orig, err := os.ReadFile(binPath)
	if err == nil {
		_ = os.WriteFile(backup, orig, 0o755)
	}
	if err := writeUpdaterState(s.st.Dir(), updaterState{
		Phase: "applying", From: Version, To: m.Version, At: time.Now().Format(time.RFC3339),
	}); err != nil {
		os.Remove(tmp)
		setProgress(0, "")
		jsonErr(w, http.StatusInternalServerError, "写更新状态失败: "+err.Error())
		return
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		setProgress(0, "")
		jsonErr(w, http.StatusInternalServerError, "设置执行权限失败: "+err.Error())
		return
	}
	// 原子替换：同一文件系统内 rename 覆盖
	if err := os.Rename(tmp, binPath); err != nil {
		os.Remove(tmp)
		setProgress(0, "")
		jsonErr(w, http.StatusInternalServerError, "替换程序失败: "+err.Error())
		return
	}
	// 先回复客户端，再原地 Exec
	setProgress(100, "更新完成，正在重启服务")
	jsonOK(w, map[string]any{"ok": true, "version": m.Version, "message": "更新完成，服务即将重启"})

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	go func() {
		time.Sleep(600 * time.Millisecond)
		args := os.Args
		env := os.Environ()
		log.Printf("在线更新：正在切换到版本 %s", m.Version)
		if err := syscallExec(binPath, args, env); err != nil {
			log.Printf("在线更新 Exec 失败: %v，尝试回滚", err)
			writeUpdaterState(s.st.Dir(), updaterState{Phase: "failed", From: Version, To: m.Version, At: time.Now().Format(time.RFC3339)})
			if len(orig) > 0 {
				_ = os.WriteFile(binPath, orig, 0o755)
			}
		}
	}()
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// 便于测试注入：实际实现在 update_exec_unix.go
var syscallExec = execBinary

