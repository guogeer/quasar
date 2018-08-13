package util

import (
	"encoding/json"
	"fmt"
	"github.com/guogeer/husky/log"
	"io/ioutil"
	"math"
	"net/http"
	"time"
)

// {"code":"0","data":{"ip":"183.14.132.213","country":"中国","area":"","region":"广东","city":"深圳","county":"XX","isp":"电信","country_id":"CN","area_id":"","region_id":"440000","city_id":"440300","county_id":"xx","isp_id":"100017"}}

type TaobaoGeoInfo struct {
	IP     string `json:"ip"`
	Region string `json:"region"`
	City   string `json:"city"`
}

type taobaoGeoArgs struct {
	Code int           `json:"code"`
	Data TaobaoGeoInfo `json:"data"`
}

func GetIPAddress(ip string) string {
	addr := ""
	if ip == "127.0.0.1" {
		return "广东深圳"
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://ip.taobao.com/service/getIpInfo.php?ip=%s", ip))
	if err != nil {
		// log.Errorf("%v", err)
		return addr
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// log.Errorf("%v", err)
		return addr
	}

	var m taobaoGeoArgs
	err = json.Unmarshal(body, &m)
	if err != nil {
		log.Errorf("%v %s", err, body)
	}
	return m.Data.Region + m.Data.City
}

// 通过经纬度计算距离，单位m
func CountDistanceByGPS(lat1, lng1, lat2, lng2 float64) float64 {
	radius := 6371000.0 // 6378137
	rad := math.Pi / 180.0

	lat1 = lat1 * rad
	lng1 = lng1 * rad
	lat2 = lat2 * rad
	lng2 = lng2 * rad

	theta := lng2 - lng1
	dist := math.Acos(math.Sin(lat1)*math.Sin(lat2) + math.Cos(lat1)*math.Cos(lat2)*math.Cos(theta))

	return dist * radius
}
