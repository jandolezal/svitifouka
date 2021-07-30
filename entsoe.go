package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// Transparency Platform restul API - user guide
// https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html
const url = "https://transparency.entsoe.eu/api?"

var resMap = map[string]string{
	"B01": "Biomass",
	"B09": "Geothermal",
	"B11": "Hydro Run-of-river and poundage",
	"B15": "Other renewable",
	"B16": "Solar",
	"B17": "Waste",
	"B19": "Wind Onshore",
}

var resList = [7]string{"B01", "B09", "B11", "B15", "B16", "B17", "B19"}

var emojiMap = map[string]string{
	"B01": "🌳",
	"B09": "🌍",
	"B11": "💧",
	"B15": "🌿",
	"B16": "☀️",
	"B17": "🗑️",
	"B19": "🌬️",
}

var params = map[string]string{
	"securityToken": os.Getenv("ENTSOE_TOKEN"),
	"In_Domain":     "10YCZ-CEPS-----N",
	"ProcessType":   "A16",
	"DocumentType":  "A75",
}

// getPastHourInterval prepares timeInterval param for Entsoe API call.
// It returns a string in the following format: 2021-07-07T05%2F2021-07-07T06
func getPastHourInterval() string {
	layout := "2006-01-02T15"
	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	timeInterval := past.Format(layout) + "%2F" + now.Format(layout)
	return timeInterval
}

type Document struct {
	XMLName    xml.Name     `xml:"GL_MarketDocument"`
	TimeSeries []TimeSeries `xml:"TimeSeries"`
}

type TimeSeries struct {
	XMLName    xml.Name   `xml:"TimeSeries"`
	Business   string     `xml:"businessType"`
	MktPSRType MktPSRType `xml:"MktPSRType"`
	Period     Period     `xml:"Period"`
}

type MktPSRType struct {
	XMLName xml.Name `xml:"MktPSRType"`
	PsrType string   `xml:"psrType"`
}

type Period struct {
	XMLName xml.Name `xml:"Period"`
	Point   Point    `xml:"Point"`
}

type Point struct {
	XMLName  xml.Name `xml:"Point"`
	Quantity int      `xml:"quantity"`
}

// getEntsoeData prepares a map from renewable type code to electricity generation in past hour
func getEntsoeData() map[string]int {
	client := &http.Client{}
	// Prepare timeInterval param
	timeInterval := getPastHourInterval()
	params["timeInterval"] = timeInterval
	// fmt.Print(params)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	// Prepare query string for Entsoe API call
	q := req.URL.Query()
	q.Add("securityToken", os.Getenv("ENTSOE_TOKEN"))
	q.Add("In_Domain", "10YCZ-CEPS-----N")
	q.Add("ProcessType", "A16")
	q.Add("DocumentType", "A75")
	q.Add("timeInterval", timeInterval)
	req.URL.RawQuery = q.Encode()
	// fmt.Println(req.URL.String())
	// Call Entsoe
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	defer resp.Body.Close()
	// Parse xml response
	body, err := ioutil.ReadAll(resp.Body)
	var document Document
	xml.Unmarshal(body, &document)
	// fmt.Print(document)
	// Extract only renewable electricity production into a map
	data := make(map[string]int)
	for _, t := range document.TimeSeries {
		for _, code := range resList {
			if t.MktPSRType.PsrType == code {
				data[code] = t.Period.Point.Quantity
			}
		}
	}
	return data
}

func main() {
	data := getEntsoeData()
	fmt.Print(data)
}
