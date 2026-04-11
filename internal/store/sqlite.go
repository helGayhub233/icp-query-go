package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps SQLite operations for ICP query history.
type Store struct {
	db *sql.DB
}

// New creates and initializes the SQLite store.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db}
	if err := s.init(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	return s, nil
}

func (s *Store) init(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS search_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				search_type TEXT NOT NULL,
				search_keyword TEXT NOT NULL,
				result_count INTEGER DEFAULT 0,
				search_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				result_data TEXT
			)`,
		`CREATE TABLE IF NOT EXISTS batch_task_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				task_name TEXT NOT NULL UNIQUE,
				task_type TEXT NOT NULL,
				total_count INTEGER DEFAULT 0,
				completed_count INTEGER DEFAULT 0,
				success_count INTEGER DEFAULT 0,
				status TEXT DEFAULT 'running',
				result_file TEXT,
				create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				finish_time TIMESTAMP
			)`,
		`CREATE INDEX IF NOT EXISTS idx_search_time ON search_history(search_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_search_type ON search_history(search_type)`,
		`CREATE INDEX IF NOT EXISTS idx_batch_create_time ON batch_task_history(create_time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_batch_status ON batch_task_history(status)`,
	}

	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("init table: %w", err)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Search History ---

// HistoryItem represents a search history record.
type HistoryItem struct {
	ID            int    `json:"id"`
	SearchType    string `json:"search_type"`
	SearchKeyword string `json:"search_keyword"`
	ResultCount   int    `json:"result_count"`
	SearchTime    string `json:"search_time"`
}

// AddHistory adds a search history record.
func (s *Store) AddHistory(ctx context.Context, searchType, keyword string, resultCount int, resultData any) (int64, error) {
	var dataJSON *string
	if resultData != nil {
		b, err := json.Marshal(resultData)
		if err != nil {
			return 0, fmt.Errorf("marshal result data: %w", err)
		}
		s := string(b)
		dataJSON = &s
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO search_history (search_type, search_keyword, result_count, result_data) VALUES (?, ?, ?, ?)`,
		searchType, keyword, resultCount, dataJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("insert history: %w", err)
	}
	return res.LastInsertId()
}

// GetHistory returns paginated search history.
func (s *Store) GetHistory(ctx context.Context, limit, offset int, searchType string) ([]HistoryItem, error) {
	var rows *sql.Rows
	var err error

	if searchType != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, search_type, search_keyword, result_count, search_time
			 FROM search_history WHERE search_type = ? ORDER BY search_time DESC LIMIT ? OFFSET ?`,
			searchType, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, search_type, search_keyword, result_count, search_time
			 FROM search_history ORDER BY search_time DESC LIMIT ? OFFSET ?`,
			limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var items []HistoryItem
	for rows.Next() {
		var h HistoryItem
		if err := rows.Scan(&h.ID, &h.SearchType, &h.SearchKeyword, &h.ResultCount, &h.SearchTime); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		items = append(items, h)
	}
	return items, rows.Err()
}

// GetHistoryDetail returns a single history record with full result data.
func (s *Store) GetHistoryDetail(ctx context.Context, id int64) (map[string]any, error) {
	var h struct {
		ID            int64
		SearchType    string
		SearchKeyword string
		ResultCount   int
		SearchTime    string
		ResultData    sql.NullString
	}

	err := s.db.QueryRowContext(ctx,
		`SELECT id, search_type, search_keyword, result_count, search_time, result_data
			 FROM search_history WHERE id = ?`, id,
	).Scan(&h.ID, &h.SearchType, &h.SearchKeyword, &h.ResultCount, &h.SearchTime, &h.ResultData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query history detail %d: %w", id, err)
	}

	result := map[string]any{
		"id":             h.ID,
		"search_type":    h.SearchType,
		"search_keyword": h.SearchKeyword,
		"result_count":   h.ResultCount,
		"search_time":    h.SearchTime,
	}

	if h.ResultData.Valid {
		var data any
		if err := json.Unmarshal([]byte(h.ResultData.String), &data); err != nil {
			return nil, fmt.Errorf("unmarshal result data for history %d: %w", id, err)
		}
		result["result_data"] = data
	}

	return result, nil
}

// DeleteHistory deletes a history record.
func (s *Store) DeleteHistory(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM search_history WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete history %d: %w", id, err)
	}
	return nil
}

// ClearHistory clears all or type-filtered history.
func (s *Store) ClearHistory(ctx context.Context, searchType string) error {
	if searchType != "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM search_history WHERE search_type = ?`, searchType)
		if err != nil {
			return fmt.Errorf("clear history by type %s: %w", searchType, err)
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM search_history`)
	if err != nil {
		return fmt.Errorf("clear all history: %w", err)
	}
	return nil
}

// GetHistoryCount returns total history count.
func (s *Store) GetHistoryCount(ctx context.Context, searchType string) (int, error) {
	var count int
	var err error
	if searchType != "" {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM search_history WHERE search_type = ?`, searchType).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM search_history`).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("count history: %w", err)
	}
	return count, nil
}

// --- Batch Tasks ---

// BatchTask represents a batch task history record.
type BatchTask struct {
	ID             int64  `json:"id"`
	TaskName       string `json:"task_name"`
	TaskType       string `json:"task_type"`
	TotalCount     int    `json:"total_count"`
	CompletedCount int    `json:"completed_count"`
	SuccessCount   int    `json:"success_count"`
	Status         string `json:"status"`
	ResultFile     string `json:"result_file"`
	CreateTime     string `json:"create_time"`
	UpdateTime     string `json:"update_time"`
	FinishTime     string `json:"finish_time"`
}

// AddBatchTask creates a new batch task record.
func (s *Store) AddBatchTask(ctx context.Context, taskName, taskType string, totalCount int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO batch_task_history (task_name, task_type, total_count, status) VALUES (?, ?, ?, 'running')`,
		taskName, taskType, totalCount)
	if err != nil {
		return 0, fmt.Errorf("add batch task %s: %w", taskName, err)
	}
	return res.LastInsertId()
}

// UpdateBatchTask updates a batch task record.
func (s *Store) UpdateBatchTask(ctx context.Context, taskName string, completed, success *int, status, resultFile *string) error {
	var setClauses string
	var args []any

	if completed != nil {
		setClauses += "completed_count = ?, "
		args = append(args, *completed)
	}
	if success != nil {
		setClauses += "success_count = ?, "
		args = append(args, *success)
	}
	if status != nil {
		setClauses += "status = ?, "
		args = append(args, *status)
	}
	if resultFile != nil {
		setClauses += "result_file = ?, "
		args = append(args, *resultFile)
	}
	setClauses += "update_time = ?"
	args = append(args, time.Now().Format("2006-01-02 15:04:05"))

	if len(args) == 1 { // only update_time
		return nil
	}

	args = append(args, taskName)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf("UPDATE batch_task_history SET %s WHERE task_name = ?", setClauses), args...)
	if err != nil {
		return fmt.Errorf("update batch task %s: %w", taskName, err)
	}
	return nil
}

// FinishBatchTask sets the finish_time for a batch task.
func (s *Store) FinishBatchTask(ctx context.Context, taskName string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := s.db.ExecContext(ctx,
		`UPDATE batch_task_history SET finish_time = ?, update_time = ? WHERE task_name = ?`,
		now, now, taskName)
	if err != nil {
		return fmt.Errorf("finish batch task %s: %w", taskName, err)
	}
	return nil
}

// GetBatchTasks returns paginated batch tasks.
func (s *Store) GetBatchTasks(ctx context.Context, limit, offset int, status string) ([]BatchTask, error) {
	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, task_name, task_type, total_count, completed_count, success_count,
				        status, result_file, create_time, update_time, finish_time
				 FROM batch_task_history WHERE status = ? ORDER BY create_time DESC LIMIT ? OFFSET ?`,
			status, limit, offset)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, task_name, task_type, total_count, completed_count, success_count,
				        status, result_file, create_time, update_time, finish_time
				 FROM batch_task_history ORDER BY create_time DESC LIMIT ? OFFSET ?`,
			limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("query batch tasks: %w", err)
	}
	defer rows.Close()

	var tasks []BatchTask
	for rows.Next() {
		var t BatchTask
		var finishTime sql.NullString
		var resultFile sql.NullString
		if err := rows.Scan(&t.ID, &t.TaskName, &t.TaskType, &t.TotalCount, &t.CompletedCount,
			&t.SuccessCount, &t.Status, &resultFile, &t.CreateTime, &t.UpdateTime, &finishTime); err != nil {
			return nil, fmt.Errorf("scan batch task row: %w", err)
		}
		if resultFile.Valid {
			t.ResultFile = resultFile.String
		}
		if finishTime.Valid {
			t.FinishTime = finishTime.String
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetBatchTaskDetail returns a single batch task.
func (s *Store) GetBatchTaskDetail(ctx context.Context, taskName string) (*BatchTask, error) {
	var t BatchTask
	var finishTime, resultFile sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT id, task_name, task_type, total_count, completed_count, success_count,
			        status, result_file, create_time, update_time, finish_time
			 FROM batch_task_history WHERE task_name = ?`, taskName,
	).Scan(&t.ID, &t.TaskName, &t.TaskType, &t.TotalCount, &t.CompletedCount,
		&t.SuccessCount, &t.Status, &resultFile, &t.CreateTime, &t.UpdateTime, &finishTime)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query batch task %s: %w", taskName, err)
	}

	if resultFile.Valid {
		t.ResultFile = resultFile.String
	}
	if finishTime.Valid {
		t.FinishTime = finishTime.String
	}
	return &t, nil
}

// DeleteBatchTask deletes a batch task and its result file.
func (s *Store) DeleteBatchTask(ctx context.Context, taskName string) error {
	detail, err := s.GetBatchTaskDetail(ctx, taskName)
	if err != nil {
		return fmt.Errorf("lookup batch task %s for deletion: %w", taskName, err)
	}
	if detail != nil && detail.ResultFile != "" {
		if rmErr := os.Remove(detail.ResultFile); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("remove result file %s: %w", detail.ResultFile, rmErr)
		}
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM batch_task_history WHERE task_name = ?`, taskName)
	if err != nil {
		return fmt.Errorf("delete batch task %s: %w", taskName, err)
	}
	return nil
}

// GetBatchTasksCount returns total batch task count.
func (s *Store) GetBatchTasksCount(ctx context.Context, status string) (int, error) {
	var count int
	var err error
	if status != "" {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM batch_task_history WHERE status = ?`, status).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM batch_task_history`).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("count batch tasks: %w", err)
	}
	return count, nil
}
