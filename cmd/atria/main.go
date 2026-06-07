package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/user/atria/internal/config"
	"github.com/user/atria/internal/crypto"
	"github.com/user/atria/internal/database"
	"github.com/user/atria/internal/migration"
	"github.com/user/atria/internal/server"
	"github.com/user/atria/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "version":
		fmt.Println(version.Info())
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Atria - 轻量级自托管 MTProto Session 管理面板")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  atria serve     启动 Web 服务")
	fmt.Println("  atria version   显示版本信息")
	fmt.Println("  atria help      显示帮助信息")
}

func runServe() {
	cfg := config.Load()

	// 初始化日志
	logOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logOpts))
	slog.SetDefault(logger)

	slog.Info("启动 Atria",
		"version", version.Short(),
		"host", cfg.Host,
		"port", cfg.Port,
	)

	// 校验配置
	if err := cfg.Validate(); err != nil {
		slog.Error("配置校验失败", "error", err)
		os.Exit(1)
	}

	// 创建数据目录
	if err := cfg.EnsureDirs(); err != nil {
		slog.Error("创建数据目录失败", "error", err)
		os.Exit(1)
	}

	// 加载或生成加密密钥
	key, err := crypto.LoadOrCreateKey(cfg.SecretKey, cfg.SecretKeyFile)
	if err != nil {
		slog.Error("加载加密密钥失败", "error", err)
		os.Exit(1)
	}
	slog.Info("加密密钥已就绪")

	// 初始化数据库
	db, err := database.Init(cfg.DatabaseDriver, cfg.DatabaseDSN)
	if err != nil {
		slog.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}

	// 执行数据库结构迁移
	if err := database.AutoMigrate(db); err != nil {
		slog.Error("数据库结构迁移失败", "error", err)
		os.Exit(1)
	}

	// 执行版本化数据迁移
	if err := migration.Run(db, key); err != nil {
		slog.Error("数据迁移失败，程序无法启动", "error", err)
		os.Exit(1)
	}

	// 创建并启动服务器
	srv := server.New(cfg, db, key)
	if err := srv.Run(); err != nil {
		slog.Error("服务器错误", "error", err)
		os.Exit(1)
	}
}
