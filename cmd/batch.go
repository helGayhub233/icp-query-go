package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/imxw/icp-query-go/internal/beian"
	"github.com/imxw/icp-query-go/internal/store"
	"github.com/imxw/icp-query-go/internal/task"
	"github.com/spf13/cobra"
)

var (
	batchFile        string
	batchType        string
	batchOutputDir   string
	batchConcurrency int
	batchAutoPage    bool
)

var batchCmd = &cobra.Command{
	Use:   "batch -f <file>",
	Short: "批量查询 ICP 备案",
	Long: `从文件读取关键词列表，并发查询 ICP 备案信息。

示例:
  icpcli batch -f domains.txt -t web
  icpcli batch -f domains.txt -t web --auto-page --concurrency 10
  icpcli batch -f domains.txt -t web --output-dir ./output`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if batchFile == "" {
			return ExitCodeError(2, "请指定关键词文件: -f <file>")
		}

		keywords, err := readKeywords(batchFile)
		if err != nil {
			return ExitCodeError(2, "读取文件失败: %v", err)
		}

		if len(keywords) == 0 {
			return ExitCodeError(2, "关键词文件为空")
		}

		slog.Info("batch query starting", "file", batchFile, "type", batchType, "count", len(keywords), "concurrency", batchConcurrency)

		// Use a copy to avoid mutating shared config
		batchCfg := *cfg
		b := beian.New(&batchCfg)
		db, err := store.New("icp_history.db")
		if err != nil {
			return ExitCodeError(10, "初始化数据库失败: %v", err)
		}
		defer db.Close()

		tm := task.NewManager(b, db)
		taskName := "cli_" + time.Now().Format("20060102_150405")

		ctx := cmd.Context()
		err = tm.Create(ctx, task.CreateRequest{
			Name:        taskName,
			Type:        batchType,
			Keywords:    keywords,
			Concurrency: batchConcurrency,
			PageSize:    26,
			AutoPage:    batchAutoPage,
			OutputDir:   batchOutputDir,
		})
		if err != nil {
			return ExitCodeError(10, "创建任务失败: %v", err)
		}

		// Wait for completion with progress display
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if p, ok := tm.GetProgress(taskName); ok {
					fmt.Fprintf(os.Stderr, "\r进度: %d/%d (成功: %d)", p.Completed, p.Total, p.Success)
				} else {
					// Task finished between ticks
					fmt.Fprintln(os.Stderr)
					goto done
				}
			case <-time.After(600 * time.Millisecond):
				if _, ok := tm.GetProgress(taskName); !ok {
					fmt.Fprintln(os.Stderr)
					goto done
				}
			}
		}

	done:
		// Get final result from DB
		if db != nil {
			detail, err := db.GetBatchTaskDetail(ctx, taskName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "获取任务详情失败: %v\n", err)
			}
			if detail != nil {
				fmt.Fprintf(os.Stderr, "任务完成: %s | 状态: %s | 完成: %d/%d | 成功: %d\n",
					taskName, detail.Status, detail.CompletedCount, detail.TotalCount, detail.SuccessCount)

				if detail.ResultFile != "" {
					fmt.Fprintf(os.Stderr, "结果文件: %s\n", detail.ResultFile)
				}
			}
		}

		return nil
	},
}

func readKeywords(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open keyword file %s: %w", path, err)
	}
	defer f.Close()

	var keywords []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			keywords = append(keywords, line)
		}
	}
	return keywords, scanner.Err()
}

func init() {
	rootCmd.AddCommand(batchCmd)

	batchCmd.Flags().StringVarP(&batchFile, "file", "f", "", "关键词文件路径 (每行一个)")
	batchCmd.Flags().StringVarP(&batchType, "type", "t", "web", "查询类型: web|app|mapp|kapp")
	batchCmd.Flags().IntVarP(&batchConcurrency, "concurrency", "j", 5, "并发数 (最大20)")
	batchCmd.Flags().BoolVar(&batchAutoPage, "auto-page", false, "自动翻页获取全部结果")
	batchCmd.Flags().StringVar(&batchOutputDir, "output-dir", "results", "结果输出目录")

	_ = batchCmd.MarkFlagRequired("file")
}
