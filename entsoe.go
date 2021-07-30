package main

import (
	"fmt"
	"os"
	"time"
)

// Transparency Platform restul API - user guide
// https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html
const url = "https://transparency.entsoe.eu/api?"

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

func main() {
	timeInterval := getPastHourInterval()
	params["timeInterval"] = timeInterval
	fmt.Print(params)
}
