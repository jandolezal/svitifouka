package main

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

// Transparency Platform restul API - user guide
// https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html
const url = "https://transparency.entsoe.eu/api?"

var resMap = map[string]string{
	"B01": "Biomass",
	"B09": "Geothermal",
	"B11": "Hydro Run-of-river and poundage",
	"B12": "Hydro Water Reservoir",
	"B15": "Other renewable",
	"B16": "Solar",
	"B17": "Waste",
	"B19": "Wind Onshore",
}

// B17 (Waste) removed
var resList = [7]string{"B01", "B09", "B11", "B12", "B15", "B16", "B19"}

var emojiMap = map[string]string{
	"B01": "🌳",
	"B09": "🌍",
	"B11": "💦",
	"B12": "💧",
	"B15": "🌿",
	"B16": "☀️",
	// "B17": "🗑️",
	"B19": "🌬️",
}

var runeMap = map[string][]rune{
	"B01": {127795},
	"B09": {127757},
	"B11": {128166},
	"B12": {128167},
	"B15": {127807},
	"B16": {9728, 65039},
	// "B17": {128465, 65039},
	"B19": {127788, 65039},
}

var dataSample = map[string]int{
	"B01": 251,
	"B11": 133,
	"B15": 266,
	"B16": 433,
	// "B17": 20,
	"B19": 92,
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

// Calculate electricity production in percent using the largest remainder method.
// Percetage as integer for the tweet (number of emojis)
// https://en.wikipedia.org/wiki/Largest_remainder_method
func calculatePercentages(data map[string]int) map[string]int {
	// Total production
	total := 0
	for _, v := range data {
		total += v
	}
	// Calculate production in percentages
	percentages := make(map[string]float64)
	for k, v := range data {
		percentages[k] = float64(v) / float64(total) * 100
	}
	// Floor percentages to integers
	floored := make(map[string]int)
	for k, v := range percentages {
		floored[k] = int(v)
	}
	// Compute difference of percentages and floored percentages
	remainders := make(map[string]float64)
	for k := range percentages {
		remainders[k] = percentages[k] - float64(floored[k])
	}
	// Get difference from the floored total and 100
	totalFloored := 0
	for _, v := range floored {
		totalFloored += v
	}
	diff := 100 - totalFloored
	// Distribute ones to sources with the highest remainder until no more ones to distribute
	resList := resList[:]
	sort.Slice(resList, func(i, j int) bool {
		return remainders[resList[i]] > remainders[resList[j]]
	})
	newPercentages := make(map[string]int)
	for _, resource := range resList {
		if diff > 0 {
			newPercentages[resource] = floored[resource] + 1
			diff -= 1
		} else {
			newPercentages[resource] = floored[resource]
		}
	}
	return newPercentages
}

// Prepare tweet string from the data
// Returns string with a certain number of emojis based on the resource (key in data) and the electricity production (value in data)
func prepareTweet(data map[string]int) string {
	// Build list of runes representing the emoji characters
	// Sort resources by electricity production (descending)
	runesList := make([]rune, 0)
	resList := resList[:]
	sort.Slice(resList, func(i, j int) bool {
		return data[resList[i]] > data[resList[j]]
	})
	for _, res := range resList {
		count := data[res]
		emojiRunes := runeMap[res]
		if len(emojiRunes) == 1 {
			// Append space for length 2 for each emoji
			emojiRunes = append(emojiRunes, 32)
		}
		resRunes := make([]rune, 0)
		for i := 0; i < count; i++ {
			resRunes = append(resRunes, emojiRunes...)
		}
		runesList = append(runesList, resRunes...)
	}
	// Split the string into lines with 10 emojis on line
	// 200 runes respresenting 100 emojis
	// 20 runes per line
	n := 20
	runesLines := make([][]rune, 0)
	for i := 0; i < len(runesList); i = i + n {
		runesLines = append(runesLines, runesList[i:i+n])
	}
	// Build tweet string from the runes
	var tweet string
	for _, line := range runesLines {
		for _, r := range line {
			tweet += string(r)
		}
		tweet += "\n"
	}
	tweet = strings.ReplaceAll(tweet, " ", "")
	return tweet
}

func main() {
	// Get datat from Entsoe API
	data := getEntsoeData()

	// Get share of each renewable technology on electrity production
	percentages := calculatePercentages(data)

	// Prepare string of emojis representing the production to tweet it
	myTweet := prepareTweet(percentages)

	consumerKey := os.Getenv("CONSUMER_KEY")
	consumerSecret := os.Getenv("CONSUMER_SECRET")
	accessToken := os.Getenv("ACCESS_TOKEN")
	accessSecret := os.Getenv("ACCESS_TOKEN_SECRET")

	// Usage according to the go-twitter library
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessSecret)
	httpClient := config.Client(oauth1.NoContext, token)
	// Twitter client
	client := twitter.NewClient(httpClient)
	// Send a Tweet
	_, _, err := client.Statuses.Update(myTweet, nil)
	if err != nil {
		log.Fatal(err)
	}
}
