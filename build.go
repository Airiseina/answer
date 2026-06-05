package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var projectRoot string

func init() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("无法获取工作目录:", err)
		os.Exit(1)
	}
	projectRoot = dir
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	// Docker
	case "infra-up":
		dockerCompose("up", "-d", "mysql", "postgres", "qdrant", "garnet", "kafka-1", "kafka-2", "etcd", "seaweedfs-master", "seaweedfs-volume", "seaweedfs-filer", "traefik", "jaeger", "otel-collector", "prometheus", "grafana", "searxng", "clickhouse", "meilisearch", "mem0-postgres")
	case "infra-down":
		dockerCompose("down")
	case "docker-up":
		dockerCompose("up", "-d")
	case "docker-down":
		dockerCompose("down")
	case "docker-down-v":
		dockerCompose("down", "-v")
	case "docker-build":
		dockerCompose("build")
	case "docker-logs":
		dockerCompose("logs", "-f")
	case "docker-ps":
		dockerCompose("ps")

	// Go 编译
	case "build":
		buildAll()
	case "build-api-gateway":
		goBuild("bin/api_gateway", "./api_gateway/cmd/")
	case "build-msg-gateway":
		goBuild("bin/msg_gateway", "./msg_gateway/cmd/")
	case "build-user-service":
		goBuild("bin/user_service", "./kitex_service/user_service/")
	case "build-chat-service":
		goBuild("bin/chat_service", "./kitex_service/chat_service/")
	case "build-group-service":
		goBuild("bin/group_service", "./kitex_service/group_service/")
	case "build-bot-service":
		goBuild("bin/bot_service", "./kitex_service/bot_service/")
	case "build-work-service":
		goBuild("bin/work_service", "./kitex_service/work_service/")
	case "build-knowledge-service":
		goBuild("bin/knowledge_service", "./kitex_service/knowledge_service/")

	// Go 代码质量
	case "fmt":
		runGo("fmt", "./...")
	case "vet":
		runGo("vet", "./...")
	case "lint":
		run("golangci-lint", "run", "./...")

	// 本地运行 Go 服务
	case "run-api-gateway":
		runGoInDir(filepath.Join(projectRoot, "api_gateway"), "run", "cmd/main.go")
	case "run-msg-gateway":
		runGoInDir(filepath.Join(projectRoot, "msg_gateway"), "run", "cmd/main.go")
	case "run-user-service":
		runGoInDir(filepath.Join(projectRoot, "kitex_service/user_service"), "run", "main.go")
	case "run-chat-service":
		runGoInDir(filepath.Join(projectRoot, "kitex_service/chat_service"), "run", "main.go")
	case "run-group-service":
		runGoInDir(filepath.Join(projectRoot, "kitex_service/group_service"), "run", "main.go")
	case "run-bot-service":
		runGoInDir(filepath.Join(projectRoot, "kitex_service/bot_service"), "run", "main.go")
	case "run-work-service":
		runGoInDir(filepath.Join(projectRoot, "kitex_service/work_service"), "run", "main.go")
	case "run-knowledge-service":
		runGoInDir(filepath.Join(projectRoot, "kitex_service/knowledge_service"), "run", "main.go")

	// MCP Server
	case "mcp-install":
		pipInstall(filepath.Join("mcp-servers", "mem0"), "requirements.txt")
		pipInstall(filepath.Join("mcp-servers", "knowledge"), "requirements.txt")
		pipInstall(filepath.Join("mcp-servers", "searxng"), "requirements.txt")
		run("pip", "install", "mcp_weather_server")
		run("pip", "install", "MCP-timeserver")
	case "mcp-run-mem0":
		runInDir(filepath.Join(projectRoot, "mcp-servers", "mem0"), "python", "-m", "mem0_mcp_server", "--mode", "sse", "--host", "0.0.0.0", "--port", "8000")
	case "mcp-run-knowledge":
		runInDir(filepath.Join(projectRoot, "mcp-servers", "knowledge"), "python", "-m", "knowledge", "--mode", "sse", "--host", "0.0.0.0", "--port", "8001")
	case "mcp-run-searxng":
		runInDir(filepath.Join(projectRoot, "mcp-servers", "searxng"), "python", "-m", "searxng", "--mode", "sse", "--host", "0.0.0.0", "--port", "8001")
	case "mcp-run-weather":
		run("python", "-m", "mcp_weather_server", "--mode", "sse", "--host", "0.0.0.0", "--port", "8080")
	case "mcp-run-timeserver":
		run("supergateway", "--stdio", `python -c "from mcp_timeserver import main; main()"`, "--port", "8000", "--ssePath", "/sse", "--messagePath", "/message")

	// 清理
	case "clean":
		clean()

	// 帮助
	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Printf("未知命令: %s\n运行 'go run build.go help' 查看可用命令\n", cmd)
		os.Exit(1)
	}
}

// ========== Docker ==========

func dockerCompose(args ...string) {
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = filepath.Join(projectRoot, "deploy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("docker compose %s 失败: %v\n", strings.Join(args, " "), err)
		os.Exit(1)
	}
}

// ========== Go 编译 ==========

func buildAll() {
	services := []struct {
		output string
		pkg    string
	}{
		{"bin/api_gateway", "./api_gateway/cmd/"},
		{"bin/msg_gateway", "./msg_gateway/cmd/"},
		{"bin/user_service", "./kitex_service/user_service/"},
		{"bin/chat_service", "./kitex_service/chat_service/"},
		{"bin/group_service", "./kitex_service/group_service/"},
		{"bin/bot_service", "./kitex_service/bot_service/"},
		{"bin/work_service", "./kitex_service/work_service/"},
		{"bin/knowledge_service", "./kitex_service/knowledge_service/"},
	}

	// 确保 bin 目录存在
	if err := os.MkdirAll(filepath.Join(projectRoot, "bin"), 0o755); err != nil {
		fmt.Println("创建 bin 目录失败:", err)
		os.Exit(1)
	}

	for _, s := range services {
		fmt.Printf("编译 %s ...\n", s.output)
		goBuild(s.output, s.pkg)
	}
	fmt.Println("所有服务编译完成")
}

func goBuild(output, pkg string) {
	// Windows 下自动加 .exe 后缀
	if runtime.GOOS == "windows" && !strings.HasSuffix(output, ".exe") {
		output += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", output, pkg)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("编译 %s 失败: %v\n", output, err)
		os.Exit(1)
	}
}

// ========== Go 运行 ==========

func runGo(args ...string) {
	cmd := exec.Command("go", args...)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("go %s 失败: %v\n", strings.Join(args, " "), err)
		os.Exit(1)
	}
}

func runGoInDir(dir string, args ...string) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("go %s 失败: %v\n", strings.Join(args, " "), err)
		os.Exit(1)
	}
}

// ========== 通用命令 ==========

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s %s 失败: %v\n", name, strings.Join(args, " "), err)
		os.Exit(1)
	}
}

func runInDir(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s %s 失败: %v\n", name, strings.Join(args, " "), err)
		os.Exit(1)
	}
}

// ========== MCP ==========

func pipInstall(dir, requirements string) {
	cmd := exec.Command("pip", "install", "-r", requirements)
	cmd.Dir = filepath.Join(projectRoot, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("pip install -r %s 失败: %v\n", requirements, err)
		os.Exit(1)
	}
}

// ========== 清理 ==========

func clean() {
	binDir := filepath.Join(projectRoot, "bin")
	if err := os.RemoveAll(binDir); err != nil {
		fmt.Println("清理 bin/ 失败:", err)
		os.Exit(1)
	}
	fmt.Println("已清理 bin/")
}

// ========== 帮助 ==========

func printHelp() {
	fmt.Println("Answer IM 构建脚本")
	fmt.Println()
	fmt.Println("用法: go run build.go <命令>")
	fmt.Println()
	fmt.Println("Docker:")
	fmt.Println("  infra-up              仅启动基础设施 (DB/Kafka/Qdrant/etcd/ClickHouse/Meilisearch/…)")
	fmt.Println("  infra-down            停止基础设施")
	fmt.Println("  docker-up             启动完整 Docker 环境")
	fmt.Println("  docker-down           停止 Docker 环境")
	fmt.Println("  docker-down-v         停止并清理数据卷")
	fmt.Println("  docker-build          构建所有 Docker 镜像")
	fmt.Println("  docker-logs           查看所有服务日志")
	fmt.Println("  docker-ps             查看运行状态")
	fmt.Println()
	fmt.Println("Go:")
	fmt.Println("  build                 编译所有 Go 服务到 bin/")
	fmt.Println("  build-api-gateway     编译 API 网关")
	fmt.Println("  build-msg-gateway     编译消息网关")
	fmt.Println("  build-user-service    编译用户服务")
	fmt.Println("  build-chat-service    编译聊天服务")
	fmt.Println("  build-group-service   编译群组服务")
	fmt.Println("  build-bot-service     编译 Bot 服务")
	fmt.Println("  build-work-service    编译工作服务")
	fmt.Println("  build-knowledge-service 编译知识库服务")
	fmt.Println("  fmt                   格式化代码")
	fmt.Println("  vet                   运行 go vet")
	fmt.Println("  lint                  运行 golangci-lint")
	fmt.Println("  run-api-gateway       启动 API 网关")
	fmt.Println("  run-msg-gateway       启动消息网关")
	fmt.Println("  run-user-service      启动用户服务")
	fmt.Println("  run-chat-service      启动聊天服务")
	fmt.Println("  run-group-service     启动群组服务")
	fmt.Println("  run-bot-service       启动 Bot 服务")
	fmt.Println("  run-work-service      启动工作服务")
	fmt.Println("  run-knowledge-service 启动知识库服务")
	fmt.Println()
	fmt.Println("MCP:")
	fmt.Println("  mcp-install           安装所有 MCP Server 依赖")
	fmt.Println("  mcp-run-mem0          启动 Mem0 记忆服务")
	fmt.Println("  mcp-run-knowledge     启动知识库 MCP")
	fmt.Println("  mcp-run-searxng       启动 SearXNG MCP")
	fmt.Println("  mcp-run-weather       启动天气服务")
	fmt.Println("  mcp-run-timeserver    启动时间服务")
	fmt.Println()
	fmt.Println("  clean                 清理编译产物")
	fmt.Println("  help                  显示此帮助信息")
}
