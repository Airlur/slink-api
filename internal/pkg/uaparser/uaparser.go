package uaparser

import (
	"strings"

	"short-link/internal/pkg/logger"

	"github.com/ua-parser/uap-go/uaparser"
)

var parser *uaparser.Parser

// InitUAParser 初始化User-Agent解析器
// regexesPath 是 uap-core/regexes.yaml 文件的路径
func Init(regexesPath string) {
	var err error
	// 【修正点】修复了 regexes-Path 的拼写错误
	parser, err = uaparser.New(regexesPath)
	if err != nil {
		logger.Fatal("初始化UAParser失败", "error", err)
	}
	logger.Info("UAParser 初始化成功")
}

// Parse 解析User-Agent字符串，返回设备类型、操作系统和浏览器信息
func Parse(ua string) (deviceType, osVersion, browser string) {
	if parser == nil {
		return "Unknown", "Unknown", "Unknown"
	}
	client := parser.Parse(ua)

	// 【核心修正点】使用新的API来判断设备类型
	// uap-go 通过 Device.Family 来识别通用设备类型，例如 "iPhone", "iPad", "Generic Smartphone"
	// PC通常没有特定的Family，或者为 "Other"
	deviceFamily := client.Device.Family
	switch {
	// 包含 "Mobile", "iPhone", "iPod", "Android" 等通常被认为是移动设备
	case strings.Contains(deviceFamily, "Mobile"), strings.Contains(deviceFamily, "iPhone"), strings.Contains(deviceFamily, "iPod"), strings.Contains(deviceFamily, "Android"):
		deviceType = "Mobile"
	// "iPad", "Tablet" 等是平板
	case strings.Contains(deviceFamily, "iPad"), strings.Contains(deviceFamily, "Tablet"):
		deviceType = "Tablet"
	// 如果 UA 包含 "Windows NT", "Macintosh" 等典型桌面系统标识，我们判定为PC
	// 这是一个补充判断，因为Device.Family对PC的识别不总是很明确
	case strings.Contains(ua, "Windows NT"), strings.Contains(ua, "Macintosh"):
		deviceType = "PC"
	// 如果Device Family为空或者是"Other"，并且不是移动设备，也可能是PC
	case deviceFamily == "Other" || deviceFamily == "":
		deviceType = "PC"
	default:
		deviceType = "Other"
	}

	osVersion = client.Os.ToString()
	browser = client.UserAgent.Family

	return
}