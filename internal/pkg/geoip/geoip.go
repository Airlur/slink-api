package geoip

import (
	"slink-api/internal/pkg/logger"

	"github.com/ip2location/ip2location-go/v9"
)

var db *ip2location.DB

func Init(dbPath string) {
	var err error
	db, err = ip2location.OpenDB(dbPath)
	if err != nil {
		logger.Fatal("failed to initialize GeoIP database", "error", err, "path", dbPath)
	}
	logger.Info("GeoIP database initialized")
}

// Parse returns country, province and city for the provided IP.
func Parse(ipStr string) (country, province, city string) {
	country = "Unknown"
	province = "Unknown"
	city = "Unknown"

	if db == nil {
		return
	}

	results, err := db.Get_all(ipStr)
	if err != nil {
		return
	}

	if results.Country_long != "" && results.Country_long != "-" {
		country = results.Country_long
	}
	if results.Region != "" && results.Region != "-" {
		province = results.Region
	} else if country != "Unknown" {
		province = country
	}
	if results.City != "" && results.City != "-" {
		city = results.City
	}

	return
}

func ParseRegion(ipStr string) string {
	country, province, _ := Parse(ipStr)
	if province != "Unknown" {
		return province
	}
	if country != "Unknown" {
		return country
	}
	return "Unknown"
}
