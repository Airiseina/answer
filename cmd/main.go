package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	fmt.Println("🚀 正在初始化配置...")
	//1. 设置浏览器启动选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),                                // 显示浏览器窗口
		chromedp.Flag("start-maximized", true),                          // 窗口最大化
		chromedp.Flag("disable-blink-features", "AutomationControlled"), // 隐藏自动化特征
		chromedp.Flag("mute-audio", true),                               //静音
	)
	//创建浏览器分配器上下文
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	// 2. 创建浏览器实例上下文
	ctx, cancelCtx := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancelCtx()
	// 设置全局超时 60 秒
	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()
	var firstResultText string
	var pageTitle string
	fmt.Println("🌐 正在启动 Chrome 浏览器...")
	// 3. 核心：执行自动化动作序列 (类似按顺序写剧本)
	err := chromedp.Run(ctx,
		// 动作 1：导航到百度首页
		chromedp.Navigate("https://www.baidu.com"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Println("📍 已打开百度首页")
			return nil
		}),
		// 动作 2：等待页面加载
		chromedp.WaitReady("body"),
		chromedp.Sleep(1*time.Second),
		// 动作 3：等待搜索框可见并输入
		chromedp.WaitVisible(`chat-textarea`, chromedp.ByID),
		// 动作 3：向搜索框发送按键，模拟打字输入
		chromedp.SendKeys(`chat-textarea`, "Golang chromedp 教程", chromedp.ByID),
		// 动作 4：点击“百度一下”搜索按钮 (百度的搜索按钮 HTML ID 是 su)
		chromedp.Click(`chat-submit-button`, chromedp.ByID),
		// 动作 5：等待搜索结果列表加载出来 (结果列表区域的 ID 是 content_left)
		chromedp.WaitVisible(`content_left`, chromedp.ByID),
		// 【可选】故意睡 2 秒，让你有时间用肉眼看清浏览器里的搜索结果
		chromedp.Sleep(2*time.Second),
		// 动作 6：提取第一条搜索结果的标题文本
		// CSS 选择器 `#content_left h3 a` 表示：找到 id 为 content_left 的元素下，第一个 h3 标签里的 a 标签
		chromedp.Text(`#content_left h3 a`, &firstResultText, chromedp.ByQuery),
	)
	// 检查是否有报错（比如网页打不开、元素找不到等）
	if err != nil {
		log.Fatal("❌ 执行出错:", err)
	}
	// 4. 打印结果
	fmt.Println("✅ 自动化操作完成！")
	fmt.Println("📄 页面标题:", pageTitle)
	fmt.Println("🎯 第一条搜索结果:", firstResultText)
}
