package geoip

import (
	"short-link/internal/pkg/logger"

	"github.com/ip2location/ip2location-go/v9"
)

var db *ip2location.DB

// InitGeoIP 初始化IP地理位置数据库
// dbPath 是 IP2Location .BIN 数据库文件的路径
func Init(dbPath string) {
	var err error
	db, err = ip2location.OpenDB(dbPath)
	if err != nil {
		logger.Fatal("初始化GeoIP数据库失败", "error", err, "path", dbPath)
	}
	logger.Info("GeoIP 数据库初始化成功")
}

// Parse 解析IP地址，返回 省份和城市 信息
func Parse(ipStr string) (province, city string) {
	// 默认值
	province = "Unknown"
	city = "Unknown"

	if db == nil {
		return
	}

	results, err := db.Get_all(ipStr)
	if err != nil {
		return
	}

	// 优先获取省份（Region）
	if results.Region != "" && results.Region != "-" {
		province = results.Region
	} else if results.Country_long != "" && results.Country_long != "-" {
		// 如果没有省份信息（例如某些国家直属市），则使用国家作为province
		province = results.Country_long
	}
	
	// 获取城市（City）
	if results.City != "" && results.City != "-" {
		city = results.City
	}

	return
}

// ParseRegion 解析IP地址，返回 省份/国家 信息
func ParseRegion(ipStr string) (region string) {
	if db == nil {
		return "Unknown"
	}

	results, err := db.Get_all(ipStr)
	if err != nil {
		return "Unknown"
	}

	// 优先获取省份（Region），如果没有则获取国家（Country_long）
	if results.Region != "" && results.Region != "-" {
		region = results.Region
	} else if results.Country_long != "" && results.Country_long != "-" {
		region = results.Country_long
	} else {
		region = "Unknown"
	}
	
	return
}

