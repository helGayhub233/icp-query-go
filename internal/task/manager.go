package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/imxw/icp-query-go/internal/store"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// Progress holds the current progress of a running task.
type Progress struct {
	Completed int `json:"completed"`
	Total     int `json:"total"`
	Success   int `json:"success"`
}

// Manager manages batch query tasks.
type Manager struct {
	beian *beian.Beian
	db    *store.Store

	running sync.Map // taskName -> *runningTask
}

type runningTask struct {
	cancel   context.CancelFunc
	progress atomic.Pointer[Progress]
	done     chan struct{}
}

// NewManager creates a new task Manager.
func NewManager(b *beian.Beian, db *store.Store) *Manager {
	return &Manager{
		beian: b,
		db:    db,
	}
}

// CreateRequest is the input for creating a batch task.
type CreateRequest struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`        // web, app, mapp, kapp
	Keywords    []string `json:"keywords"`    // domain names to query
	Concurrency int      `json:"concurrency"` // max concurrent queries (default 5, max 20)
	PageSize    int      `json:"page_size"`   // results per page (default 10, max 26)
	AutoPage    bool     `json:"auto_page"`   // fetch all pages until empty
	OutputDir   string   `json:"output_dir"`  // directory for result JSON files
}

// Create starts a new batch query task.
func (m *Manager) Create(ctx context.Context, req CreateRequest) error {
	if req.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}
	if len(req.Keywords) == 0 {
		return fmt.Errorf("查询关键词不能为空")
	}
	if m.db == nil {
		return fmt.Errorf("任务存储未初始化")
	}

	// Check for duplicate
	if _, loaded := m.running.Load(req.Name); loaded {
		return fmt.Errorf("任务 %q 已在运行中", req.Name)
	}

	serviceType, ok := beian.ParseServiceType(req.Type)
	if !ok {
		return fmt.Errorf("不支持的查询类型: %s (可选: web, app, mapp, kapp)", req.Type)
	}

	if req.Concurrency <= 0 {
		req.Concurrency = 5
	}
	if req.Concurrency > 20 {
		req.Concurrency = 20
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 26 {
		req.PageSize = 26
	}
	if req.OutputDir == "" {
		req.OutputDir = "results"
	}

	// Create DB record
	_, err := m.db.AddBatchTask(ctx, req.Name, req.Type, len(req.Keywords))
	if err != nil {
		return fmt.Errorf("创建任务记录失败: %w", err)
	}

	// Start background execution
	taskCtx, cancel := context.WithCancel(context.Background())
	rt := &runningTask{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	p := &Progress{Total: len(req.Keywords)}
	rt.progress.Store(p)

	m.running.Store(req.Name, rt)

	go m.runTask(taskCtx, rt, req, serviceType)

	return nil
}

// GetProgress returns the current progress of a task.
func (m *Manager) GetProgress(name string) (*Progress, bool) {
	val, ok := m.running.Load(name)
	if !ok {
		return nil, false
	}
	rt, ok := val.(*runningTask)
	if !ok {
		return nil, false
	}
	p := rt.progress.Load()
	if p == nil {
		return &Progress{}, true
	}
	return p, true
}

// Cancel cancels a running task.
func (m *Manager) Cancel(name string) error {
	val, ok := m.running.Load(name)
	if !ok {
		return fmt.Errorf("任务 %q 不存在或已完成", name)
	}
	rt, ok := val.(*runningTask)
	if !ok {
		return fmt.Errorf("任务 %q 状态异常", name)
	}
	rt.cancel()
	return nil
}

// Remove removes a completed task from the running map.
func (m *Manager) Remove(name string) {
	m.running.Delete(name)
}

// runTask executes the batch query in the background.
func (m *Manager) runTask(ctx context.Context, rt *runningTask, req CreateRequest, serviceType beian.ServiceType) {
	defer close(rt.done)
	defer m.running.Delete(req.Name)

	sem := semaphore.NewWeighted(int64(req.Concurrency))
	g, gctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	var allResults []map[string]any

	for i, keyword := range req.Keywords {
		i, keyword := i, keyword

		if gctx.Err() != nil {
			break
		}

		g.Go(func() error {
			if err := sem.Acquire(gctx, 1); err != nil {
				return nil // context cancelled
			}
			defer sem.Release(1)

			results, err := m.queryOne(gctx, keyword, serviceType, req.PageSize, req.AutoPage)
			if err != nil {
				slog.Warn("batch query failed", "keyword", keyword, "error", err)
				m.updateProgress(rt, 1, 0)
				return nil // don't stop on individual failure
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()

			m.updateProgress(rt, 1, 1)
			return nil
		})

		// Small delay between submissions to avoid overwhelming the API
		if i < len(req.Keywords)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	if err := g.Wait(); err != nil {
		slog.Error("batch task group error", "name", req.Name, "error", err)
	}

	// Determine final status
	status := "completed"
	if ctx.Err() != nil {
		status = "cancelled"
	}

	// Save results to JSON file
	var resultFile string
	if len(allResults) > 0 {
		path := filepath.Join(req.OutputDir, req.Name+".json")
		if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
			slog.Error("create results dir failed", "error", err)
		} else {
			data, err := json.MarshalIndent(allResults, "", "  ")
			if err != nil {
				slog.Error("marshal results failed", "error", err)
			} else if err := os.WriteFile(path, data, 0644); err != nil {
				slog.Error("write results file failed", "path", path, "error", err)
			} else {
				resultFile = path
			}
		}
	}

	// Update DB with a detached timeout context
	p := rt.progress.Load()
	completed := 0
	success := 0
	if p != nil {
		completed = p.Completed
		success = p.Success
	}

	finalizeCtx, finalizeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finalizeCancel()

	if err := m.db.UpdateBatchTask(finalizeCtx, req.Name, &completed, &success, &status, &resultFile); err != nil {
		slog.Error("update batch task failed", "name", req.Name, "error", err)
	}
	if err := m.db.FinishBatchTask(finalizeCtx, req.Name); err != nil {
		slog.Error("finish batch task failed", "name", req.Name, "error", err)
	}

	slog.Info("batch task done", "name", req.Name, "status", status,
		"completed", completed, "success", success, "results", len(allResults))
}

func (m *Manager) queryOne(ctx context.Context, keyword string, serviceType beian.ServiceType, pageSize int, autoPage bool) ([]map[string]any, error) {
	var allItems []map[string]any
	page := 1

	for {
		data, err := m.beian.Query(ctx, beian.QueryRequest{
			Name:        keyword,
			ServiceType: serviceType,
			PageNum:     page,
			PageSize:    pageSize,
		})
		if err != nil {
			return allItems, fmt.Errorf("query keyword %q page %d: %w", keyword, page, err)
		}

		code, ok := beian.ResponseCode(data)
		if !ok {
			return allItems, fmt.Errorf("query returned invalid code for keyword %q", keyword)
		}
		if code != 200 && code != 0 {
			return allItems, fmt.Errorf("query returned code %v for keyword %q", code, keyword)
		}

		list, ok := beian.ResponseList(data)
		if !ok || len(list) == 0 {
			break
		}

		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				allItems = append(allItems, m)
			}
		}

		if !autoPage {
			break
		}

		total, ok := beian.ResponseTotal(data)
		if !ok {
			break
		}
		if total <= len(allItems) {
			break
		}
		page++
	}

	return allItems, nil
}

func (m *Manager) updateProgress(rt *runningTask, addCompleted, addSuccess int) {
	for {
		old := rt.progress.Load()
		newP := &Progress{
			Completed: old.Completed + addCompleted,
			Total:     old.Total,
			Success:   old.Success + addSuccess,
		}
		if rt.progress.CompareAndSwap(old, newP) {
			return
		}
	}
}
