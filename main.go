//go:generate protoc --go_out=. applewloc.proto
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gnomegl/bssidx/pb"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func main() {
	mapFlag := flag.Bool("m", false, "Show location on OpenStreetMap via geojson.io")
	flag.Bool("map", false, "Show location on OpenStreetMap via geojson.io")
	allFlag := flag.Bool("a", false, "Show all results")
	flag.Bool("all", false, "Show all results")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: bssidx [flags] BSSID")
		os.Exit(1)
	}

	bssid := flag.Arg(0)
	fmt.Printf("Searching for location of BSSID: %s\n", bssid)

	wloc := &pb.AppleWLoc{
		WifiDevices:        []*pb.WifiDevice{{Bssid: proto.String(bssid)}},
		UnknownValue1:      proto.Int32(0),
		ReturnSingleResult: proto.Int32(1),
	}

	data, _ := proto.Marshal(wloc)
	payload := append([]byte{0, 1, 0, 5}, []byte("en_US")...)
	payload = append(payload, []byte{0, 19}...)
	payload = append(payload, []byte("com.apple.locationd")...)
	payload = append(payload, []byte{0, 10}...)
	payload = append(payload, []byte("8.1.12B411")...)
	payload = append(payload, []byte{0, 0, 0, 1, 0, 0, 0, byte(len(data))}...)
	payload = append(payload, data...)

	req, _ := http.NewRequest("POST", "https://gs-loc.apple.com/clls/wloc", bytes.NewReader(payload))
	req.Header.Set("User-Agent", "locationd/1753.17 CFNetwork/889.9 Darwin/17.2.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respData := buf.Bytes()[10:]

	result := &pb.AppleWLoc{}
	proto.Unmarshal(respData, result)

	found := false
	for _, device := range result.WifiDevices {
		if device.Location == nil {
			continue
		}

		lat := float64(*device.Location.Latitude) * 1e-8
		lon := float64(*device.Location.Longitude) * 1e-8

		if lat == -180.0 && lon == -180.0 {
			continue
		}

		mac := formatBSSID(*device.Bssid)
		if !*allFlag && !strings.EqualFold(mac, bssid) {
			continue
		}

		if found {
			fmt.Println()
		}

		fmt.Printf("BSSID: %s\n", mac)
		fmt.Printf("Latitude: %f\n", lat)
		fmt.Printf("Longitude: %f\n", lon)

		if *mapFlag {
			url := createGeoJSONURL(lat, lon, mac)
			exec.Command("xdg-open", url).Start()
		}

		found = true
	}

	if !found {
		fmt.Println("The BSSID was not found.")
		os.Exit(1)
	}
}

func formatBSSID(bssid string) string {
	parts := strings.Split(bssid, ":")
	for i := range parts {
		if len(parts[i]) == 1 {
			parts[i] = "0" + parts[i]
		}
	}
	return strings.Join(parts, ":")
}

func createGeoJSONURL(lat, lon float64, bssid string) string {
	geoJSON := map[string]interface{}{
		"type": "FeatureCollection",
		"features": []map[string]interface{}{
			{
				"type": "Feature",
				"geometry": map[string]interface{}{
					"type":        "Point",
					"coordinates": []float64{lon, lat}, 
				},
				"properties": map[string]interface{}{
					"title":         bssid,
					"description":   fmt.Sprintf("BSSID: %s\nLocation: %.6f, %.6f", bssid, lat, lon),
					"marker-color":  "#ff0000",
					"marker-size":   "medium",
					"marker-symbol": "star",
				},
			},
		},
	}

	jsonData, _ := json.Marshal(geoJSON)
	encodedData := url.QueryEscape(string(jsonData))
	return fmt.Sprintf("https://geojson.io/#data=data:application/json,%s", encodedData)
}
