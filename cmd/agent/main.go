package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/xexgm/GMAgent/internal/model"
	"github.com/xexgm/GMAgent/internal/runtime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	task := flag.String("task", "", "要执行的任务")
	modelID := flag.String("model", model.DefaultModel, "方舟模型 ID")
	flag.Parse()

	if strings.TrimSpace(*task) == "" {
		return fmt.Errorf("请通过 --task 指定任务")
	}

	client, err := model.NewClient(os.Getenv("ARK_API_KEY"), *modelID)
	if err != nil {
		return err
	}

	// 按 Ctrl+C 时取消正在进行的 HTTP 请求。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	repoDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取仓库目录：%w", err)
	}
	fmt.Printf("仓库目录：%s\n", repoDir)
	fmt.Printf("正在请求模型：%s\n", *modelID)
	reply, err := runtime.Run(ctx, client, *task, repoDir, os.Stdout)
	if err != nil {
		return err
	}

	fmt.Println(reply)
	return nil
}
