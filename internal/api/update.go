package api

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	updateRepo = "1316195118/Home-Feast"
	// 用完整 ref 而不是 /main/：GitHub Raw 对短分支地址维护独立缓存，
	// 发版后几分钟内可能仍返回旧 manifest；refs/heads/main 会立即指向最新提交。
	updateBranch = "refs/heads/main"
	updateRaw    = "https://raw.githubusercontent.com/" + updateRepo + "/" + updateBranch + "/dist/"
	updateProxy  = "https://ghproxy.net/" + updateRaw
	updateDir    = ".update" // 位于 dataDir 下，与业务数据同卷，随备份一起走
)

type releaseFile struct {
	Name     string `json:"name"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Gz       string `json:"gz,omitempty"`       // gzip 版文件名
	GzSHA256 string `json:"gzSha256,omitempty"` // 压缩包哈希
	GzSize   int64  `json:"gzSize,omitempty"`
}

type releaseManifest struct {
	Version     string                 `json:"version"`
	PublishedAt string                 `json:"publishedAt"`
	Note        string                 `json:"note"`
	Files       map[string]releaseFile `json:"files"`
}

type updateStatus struct {
	Checking    bool   `json:"checking"`
	Downloading bool   `json:"downloading"`
	LastCheck   string `json:"lastCheck,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	Available   bool   `json:"available"`
	Latest      string `json:"latest,omitempty"`
	Progress    int    `json:"progress"`
	Message     string `json:"message,omitempty"`
}

var updateMu sync.Mutex
var updateState updateStatus

// updateClient 构造走代理的 HTTP 客户端。代理地址来自设置页，只用于更新下载。
// 直连时 transport 为 nil，等同于默认行为。
func updateClient(proxyURL string, timeout time.Duration) *http.Client {
	c := &http.Client{Timeout: timeout}
	if proxyURL == "" {
		return c
	}
	pu, err := url.Parse(proxyURL)
	if err != nil || pu.Host == "" {
		return c
	}
	c.Transport = &http.Transport{
		Proxy:                 http.ProxyURL(pu),
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	return c
}

// 更新源列表：raw 优先，ghproxy 兜底。
// MENU_UPDATE_BASE 可覆盖为本地测试源（如 http://127.0.0.1:18099/dist/），便于 E2E 验证。
func updateSources() []string {
	if v := os.Getenv("MENU_UPDATE_BASE"); v != "" {
		return []string{strings.TrimRight(v, "/") + "/"}
	}
	return []string{updateRaw, updateProxy}
}

func fetchUpdateJSON(ctx context.Context, path, proxyURL string, dest any) error {
	var lastErr error
	firstErr := ""
	for i, base := range updateSources() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "family-menu-updater")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")
		resp, err := updateClient(proxyURL, 20*time.Second).Do(req)
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
	err := fetchUpdateJSON(r.Context(), "manifest.json", s.st.GetSettings().ProxyURL, &m)
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
	Phase string `json:"phase"` // pending / applying / applied / failed
	From  string `json:"from"`
	To    string `json:"to"`
	At    string `json:"at"`
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
	if json.Unmarshal(b, &st) != nil {
		return
	}
	// 进程已经成功切换，但前端可能在上一个进程退出后才读到新进程状态。
	// 把落盘的成功阶段恢复为内存状态，避免把空状态误判成“任务提前结束”。
	if st.Phase == "applied" {
		markUpdateApplied(st)
		return
	}
	if st.Phase != "applying" {
		return
	}
	// 记录过 applying 但能走到这里，说明新二进制成功启动了（本进程就是新的）
	if st.To == Version {
		applied := updaterState{Phase: "applied", From: st.From, To: st.To, At: time.Now().Format(time.RFC3339)}
		_ = writeUpdaterState(dataDir, applied)
		markUpdateApplied(applied)
		return
	}
	// 版本对不上：上一版二进制被重新拉起，说明更新失败，清理 staging
	_ = os.Remove(backup)
	_ = os.Remove(stateFile)
	log.Printf("更新回退完成，当前版本 %s", Version)
}

func markUpdateApplied(st updaterState) {
	updateMu.Lock()
	updateState.Checking = false
	updateState.Downloading = false
	updateState.LastError = ""
	updateState.Available = false
	updateState.Latest = st.To
	updateState.Progress = 100
	updateState.Message = "更新完成，正在重启服务"
	updateMu.Unlock()
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

// testProxy 用给定代理试连 GitHub，返回耗时与结果，供设置页「测试」按钮使用。
// 不传 proxy 时测试当前保存的配置。
func (s *Server) testProxy(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProxyURL *string `json:"proxyUrl"`
	}
	if err := decodeBody(w, r, &in); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	proxy := s.st.GetSettings().ProxyURL
	if in.ProxyURL != nil {
		proxy = strings.TrimSpace(*in.ProxyURL)
	}
	if proxy != "" {
		u, err := url.Parse(proxy)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
			jsonErr(w, http.StatusBadRequest, "代理地址格式应为 http://IP:端口 或 socks5://IP:端口")
			return
		}
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateRaw+"manifest.json", nil)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Header.Set("User-Agent", "family-menu-updater")
	resp, err := updateClient(proxy, 25*time.Second).Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		jsonOK(w, map[string]any{
			"ok": false, "ms": elapsed, "proxy": proxy,
			"error": "连不上：" + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	jsonOK(w, map[string]any{
		"ok": resp.StatusCode == http.StatusOK, "status": resp.StatusCode,
		"ms": elapsed, "proxy": proxy,
	})
}

// applyUpdate 下载 → 校验 → 备份 → 原子替换 → 延迟 Exec。
func (s *Server) applyUpdate(w http.ResponseWriter, r *http.Request) {
	// force=true 时跳过「已是最新」判断，强制重装一次（版本相同也重新下载替换）
	var in struct {
		Force bool `json:"force"`
	}
	if r.ContentLength > 0 {
		if err := decodeBody(w, r, &in); err != nil {
			jsonErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}

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

	// 下载、校验和重试可能在网络差时持续几分钟。接口只负责受理任务，
	// 长任务在后台继续跑，避免 http.Server 的写超时把进度和结果一起掐掉。
	jsonStatus(w, http.StatusAccepted, map[string]any{
		"ok": true, "accepted": true, "message": "更新任务已开始",
	})
	go s.runUpdateTask(context.Background(), in.Force)
}

// runUpdateTask 执行一次在线更新。状态通过 /api/update/status 轮询，
// 直到 syscall.Exec 成功替换当前进程为止。
func (s *Server) runUpdateTask(ctx context.Context, force bool) {
	setProgress := func(p int, msg string) {
		updateMu.Lock()
		updateState.Progress = p
		updateState.Message = msg
		updateMu.Unlock()
	}
	fail := func(err error) {
		updateMu.Lock()
		updateState.Downloading = false
		updateState.LastError = err.Error()
		updateState.Message = "更新失败"
		updateMu.Unlock()
	}

	var m releaseManifest
	if err := fetchUpdateJSON(ctx, "manifest.json", s.st.GetSettings().ProxyURL, &m); err != nil {
		setProgress(0, "")
		fail(fmt.Errorf("获取更新信息失败: %w", err))
		return
	}
	updateMu.Lock()
	updateState.Latest = m.Version
	updateState.Available = compareVersion(m.Version, Version) > 0
	updateMu.Unlock()
	if !force && compareVersion(m.Version, Version) <= 0 {
		setProgress(100, "已是最新版本")
		updateMu.Lock()
		updateState.Downloading = false
		updateMu.Unlock()
		return
	}
	f, ok := m.Files[goArchKey()]
	if !ok || f.Name == "" || f.SHA256 == "" {
		setProgress(0, "")
		fail(fmt.Errorf("此架构没有可用更新: %s", goArchKey()))
		return
	}

	dir, staging, backup, stateFile := updatePaths(s.st.Dir())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		setProgress(0, "")
		fail(fmt.Errorf("创建更新目录失败: %w", err))
		return
	}

	// 下载并校验：优先 gzip 版（体积约一半，容器到 GitHub 链路不稳时成功率高），
	// 失败自动换源重试；压缩包校验 gzSha256，解压后再校验二进制 sha256。
	setProgress(10, "正在下载新版本")
	if err := downloadRelease(ctx, f, staging, s.st.GetSettings().ProxyURL, setProgress); err != nil {
		setProgress(0, "")
		fail(err)
		return
	}

	// 备份当前二进制
	setProgress(85, "正在替换")
	binPath, err := os.Executable()
	if err != nil {
		os.Remove(staging)
		setProgress(0, "")
		fail(fmt.Errorf("无法定位当前程序: %w", err))
		return
	}
	// 跟随软链接，确保替换的是真文件
	if resolved, e := filepath.EvalSymlinks(binPath); e == nil {
		binPath = resolved
	}
	orig, err := os.ReadFile(binPath)
	if err != nil {
		os.Remove(staging)
		setProgress(0, "")
		fail(fmt.Errorf("读取当前程序失败，已取消更新: %w", err))
		return
	}
	if err := atomicFile(backup, orig); err != nil {
		os.Remove(staging)
		setProgress(0, "")
		fail(fmt.Errorf("备份当前程序失败，已取消更新: %w", err))
		return
	}
	if err := writeUpdaterState(s.st.Dir(), updaterState{
		Phase: "applying", From: Version, To: m.Version, At: time.Now().Format(time.RFC3339),
	}); err != nil {
		os.Remove(staging)
		setProgress(0, "")
		fail(fmt.Errorf("写更新状态失败: %w", err))
		return
	}
	// 原子替换：staging 在数据卷、binPath 在容器可写层，属于不同文件系统，
	// 直接 rename 会报 "invalid cross-device link"。先复制到目标同目录的临时文件，
	// 再在同目录内 rename——这一步才是原子的。
	tmpInTarget := binPath + ".new"
	if err := copyFileTo(staging, tmpInTarget); err != nil {
		os.Remove(tmpInTarget)
		os.Remove(staging)
		os.Remove(stateFile)
		setProgress(0, "")
		fail(fmt.Errorf("写入新程序失败: %w", err))
		return
	}
	if err := os.Chmod(tmpInTarget, 0o755); err != nil {
		os.Remove(tmpInTarget)
		os.Remove(staging)
		os.Remove(stateFile)
		setProgress(0, "")
		fail(fmt.Errorf("设置执行权限失败: %w", err))
		return
	}
	if err := os.Rename(tmpInTarget, binPath); err != nil {
		os.Remove(tmpInTarget)
		os.Remove(staging)
		os.Remove(stateFile)
		setProgress(0, "")
		fail(fmt.Errorf("替换程序失败: %w", err))
		return
	}
	// 保持 Downloading=true 到 Exec 发生，前端据此知道服务正在切换，
	// 不会把短暂的连接中断误判成更新失败。
	setProgress(100, "更新完成，正在重启服务")
	time.Sleep(600 * time.Millisecond)
	args := os.Args
	env := os.Environ()
	log.Printf("在线更新：正在切换到版本 %s", m.Version)
	if err := syscallExec(binPath, args, env); err != nil {
		log.Printf("在线更新 Exec 失败: %v，尝试回滚", err)
		_ = writeUpdaterState(s.st.Dir(), updaterState{Phase: "failed", From: Version, To: m.Version, At: time.Now().Format(time.RFC3339)})
		rollbackErr := atomicFile(binPath, orig)
		if rollbackErr == nil {
			rollbackErr = os.Chmod(binPath, 0o755)
		}
		if rollbackErr != nil {
			fail(fmt.Errorf("切换新版本失败，且回滚失败: %v；%w", err, rollbackErr))
			return
		}
		fail(fmt.Errorf("切换新版本失败，已恢复旧版本: %w", err))
	}
}

// downloadRelease 把某个架构的产物下载到 staging，优先 gzip。
// 每个源重试 3 次，任意一次成功即返回；两次源都失败则报最后一次错误。
func downloadRelease(ctx context.Context, f releaseFile, staging, proxyURL string, setProgress func(int, string)) error {
	useGz := f.Gz != "" && f.GzSHA256 != ""
	remoteName, wantHash, wantSize := f.Name, f.SHA256, f.Size
	if useGz {
		// 仓库只存 gz，没有未压缩件可回退
		remoteName, wantHash, wantSize = f.Gz, f.GzSHA256, f.GzSize
	}
	tmp := staging + ".dl"
	var lastErr error
	for _, base := range updateSources() {
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
				setProgress(10, fmt.Sprintf("重试第 %d 次", attempt))
			}
			got, err := fetchToFile(ctx, proxyURL, base+remoteName, tmp, wantSize, setProgress)
			if err != nil {
				lastErr = err
				continue
			}
			if got != wantHash {
				lastErr = fmt.Errorf("校验失败：%s 哈希不匹配", remoteName)
				continue
			}
			// 压缩包：解压到 staging 并校验二进制哈希
			if useGz {
				setProgress(82, "正在解压")
				if err := gunzipTo(tmp, staging); err != nil {
					lastErr = err
					continue
				}
				os.Remove(tmp)
				h, size, err := hashFile(staging)
				if err != nil {
					lastErr = err
					continue
				}
				if !strings.EqualFold(h, f.SHA256) || (f.Size > 0 && size != f.Size) {
					lastErr = fmt.Errorf("解压后校验失败：哈希或大小不符")
					continue
				}
			} else if err := os.Rename(tmp, staging); err != nil {
				lastErr = err
				continue
			}
			_ = os.Chmod(staging, 0o755)
			return nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("下载失败")
	}
	// 重试全部失败时清掉半成品，避免下一次更新误用残留文件。
	_ = os.Remove(tmp)
	_ = os.Remove(staging)
	return fmt.Errorf("下载更新失败: %v", lastErr)
}

// fetchToFile 下载到 path，返回 sha256。total 用于进度百分比。
func fetchToFile(ctx context.Context, proxyURL, url, path string, total int64, setProgress func(int, string)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "family-menu-updater")
	resp, err := updateClient(proxyURL, 10*time.Minute).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer out.Close()
	hasher := sha256.New()
	buf := make([]byte, 128*1024)
	var read int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return "", werr
			}
			hasher.Write(buf[:n])
			read += int64(n)
			if total > 0 {
				p := 10 + int(float64(read)/float64(total)*70)
				if p > 80 {
					p = 80
				}
				setProgress(p, fmt.Sprintf("正在下载 %.1f/%.1f MB", float64(read)/1048576, float64(total)/1048576))
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return "", rerr
		}
	}
	if total > 0 && read != total {
		return "", fmt.Errorf("文件不完整：%d/%d 字节", read, total)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func gunzipTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// copyFileTo 跨文件系统复制（数据卷 → 容器可写层），保留执行权限由调用方设置。
func copyFileTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// 便于测试注入：实际实现在 update_exec_unix.go
var syscallExec = execBinary
