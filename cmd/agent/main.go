package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/xexgm/GMAgent/internal/model"
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

	fmt.Printf("正在请求模型：%s\n", *modelID)
	reply, err := client.Chat(ctx, *task)
	if err != nil {
		return err
	}

	fmt.Println(reply)
	return nil
}
