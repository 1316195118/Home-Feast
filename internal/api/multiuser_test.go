package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"familymenu/internal/store"
)

type bootResponse struct {
	Today   string `json:"today"`
	Meals   []store.MealSlot
	Dishes  []store.Dish
	Members []store.Member
	Plan    store.Plan
	Plans   []store.Plan
}

func fetchBoot(client *http.Client, baseURL string) (bootResponse, error) {
	resp, err := client.Get(baseURL + "/api/bootstrap")
	if err != nil {
		return bootResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return bootResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return bootResponse{}, fmt.Errorf("bootstrap %d: %s", resp.StatusCode, body)
	}
	var out bootResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return bootResponse{}, err
	}
	return out, nil
}

// TestMultiUserStress 覆盖家庭内多设备最典型的同时使用路径：
// 首屏读取、长连接、多人同时点菜，以及写入后的完整快照读取。
func TestMultiUserStress(t *testing.T) {
	st, err := store.Open(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(st, nil, ""))
	defer srv.Close()
	client := srv.Client()

	first, err := fetchBoot(client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Dishes) < 60 || len(first.Members) == 0 || len(first.Meals) == 0 {
		t.Fatalf("测试数据不足: dishes=%d members=%d meals=%d",
			len(first.Dishes), len(first.Members), len(first.Meals))
	}

	const readers = 80
	const streams = 30
	const orders = 60
	const toggles = 120
	errs := make(chan error, readers*2+streams+orders+toggles+4)

	var readersWG sync.WaitGroup
	for i := 0; i < readers; i++ {
		readersWG.Add(1)
		go func() {
			defer readersWG.Done()
			if _, err := fetchBoot(client, srv.URL); err != nil {
				errs <- err
			}
		}()
	}

	sseCtx, cancelSSE := context.WithCancel(context.Background())
	defer cancelSSE()
	var streamsWG sync.WaitGroup
	for i := 0; i < streams; i++ {
		streamsWG.Add(1)
		go func() {
			defer streamsWG.Done()
			req, _ := http.NewRequestWithContext(sseCtx, http.MethodGet, srv.URL+"/api/events", nil)
			resp, err := client.Do(req)
			if err != nil {
				if sseCtx.Err() == nil {
					errs <- err
				}
				return
			}
			defer resp.Body.Close()
			line, err := bufio.NewReader(resp.Body).ReadString('\n')
			if err != nil && sseCtx.Err() == nil {
				errs <- err
				return
			}
			if line == "" && sseCtx.Err() == nil {
				errs <- fmt.Errorf("SSE 未返回握手数据")
				return
			}
			<-sseCtx.Done()
		}()
	}

	var ordersWG sync.WaitGroup
	for i := 0; i < orders; i++ {
		ordersWG.Add(1)
		go func(i int) {
			defer ordersWG.Done()
			payload, _ := json.Marshal(map[string]string{
				"dishId": first.Dishes[i].ID,
				"userId": first.Members[i%len(first.Members)].ID,
				"meal":   first.Meals[0].Key,
				"note":   fmt.Sprintf("并发点菜-%d", i),
			})
			req, _ := http.NewRequest(http.MethodPost,
				srv.URL+"/api/plans/"+first.Today+"/orders", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				errs <- err
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				errs <- fmt.Errorf("并发点菜 %d: %d %s", i, resp.StatusCode, body)
			}
		}(i)
	}

	readersWG.Wait()
	ordersWG.Wait()
	created, err := fetchBoot(client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Plan.Orders) != orders {
		t.Fatalf("切换状态前应有 %d 条订单，实际 %d", orders, len(created.Plan.Orders))
	}

	var mixedReadWG sync.WaitGroup
	for i := 0; i < readers; i++ {
		mixedReadWG.Add(1)
		go func() {
			defer mixedReadWG.Done()
			if _, err := fetchBoot(client, srv.URL); err != nil {
				errs <- err
			}
		}()
	}
	var toggleWG sync.WaitGroup
	for i := 0; i < toggles; i++ {
		toggleWG.Add(1)
		go func(i int) {
			defer toggleWG.Done()
			order := created.Plan.Orders[i%len(created.Plan.Orders)]
			req, _ := http.NewRequest(http.MethodPut,
				srv.URL+"/api/plans/"+first.Today+"/orders/"+order.ID, nil)
			resp, err := client.Do(req)
			if err != nil {
				errs <- err
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("并发切换状态 %d: %d %s", i, resp.StatusCode, body)
			}
		}(i)
	}
	mixedReadWG.Wait()
	toggleWG.Wait()

	final, err := fetchBoot(client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(final.Plan.Orders); got != orders {
		errs <- fmt.Errorf("并发点菜后应有 %d 条订单，实际 %d", orders, got)
	}
	seen := make(map[string]bool, len(final.Plan.Orders))
	for _, order := range final.Plan.Orders {
		if seen[order.ID] {
			errs <- fmt.Errorf("订单 ID 重复: %s", order.ID)
		}
		seen[order.ID] = true
	}

	cancelSSE()
	done := make(chan struct{})
	go func() {
		streamsWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		errs <- fmt.Errorf("取消后仍有 SSE 连接未释放")
	}

	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
