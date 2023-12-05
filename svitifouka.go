/* Twitter bot tweeting renewable electricity production in Czechia as emojis.

Requests data from the Entsoe API, prepares a map from each renewable technology
to its share on renewable electricity production and makes a tweet string
representing the production as 100 emojis with the emoji depending on the technology.

It tweets the string (updates status of twitter.com/sviti_fouka) using the go-twitter library.
*/
package main

import (
	"encoding/xml"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"fmt"
	"context"

	"github.com/michimani/gotwi"
	"github.com/michimani/gotwi/tweet/managetweet"
	"github.com/michimani/gotwi/tweet/managetweet/types"

)

// Transparency Platform restul API - user guide
// https://transparency.entsoe.eu/content/static_content/Static%20content/web%20api/Guide.html
const url = "https://web-api.tp.entsoe.eu/api?"

var technologies = []string{"B01", "B09", "B11", "B12", "B15", "B16", "B19"}

/* Runes representing emoji characters
B01, 🌿, Biomass
B09, 🌍, Geothermal
B11, 💦, Hydro Run-of-river and poundage
B12, 💧, Hydro Water Reservoir
B15, ♻️, Other renewable
B16, ☀️, Solar
B19, 🌬️, Wind Onshore
*/
var runeMap = map[string][]rune{
	"B01": {127807},
	"B09": {127757},
	"B11": {128166},
	"B12": {128167},
	"B15": {9851, 65039},
	"B16": {9728, 65039},
	"B19": {127788, 65039},
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
func getEntsoeData(url string) (map[string]int, error) {
	client := &http.Client{}
	// Prepare timeInterval param
	timeInterval := getPastHourInterval()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Prepare query string for Entsoe API call
	q := req.URL.Query()
	q.Add("securityToken", os.Getenv("ENTSOE_TOKEN"))
	q.Add("In_Domain", "10YCZ-CEPS-----N")
	q.Add("ProcessType", "A16")
	q.Add("DocumentType", "A75")
	q.Add("timeInterval", timeInterval)
	req.URL.RawQuery = q.Encode()
	// Call Entsoe
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Got non-ok response status from Entsoe:" + resp.Status)
	}
	defer resp.Body.Close()
	// Parse xml response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var document Document
	xml.Unmarshal(body, &document)
	// Extract only renewable electricity production into a map
	data := make(map[string]int, len(technologies))
	for _, t := range document.TimeSeries {
		for _, code := range technologies {
			if t.MktPSRType.PsrType == code {
				data[code] = t.Period.Point.Quantity
			}
		}
	}
	return data, nil
}

// Calculate electricity production in percent using the largest remainder method.
// Percetage as integer for the tweet (number of emojis)
// https://en.wikipedia.org/wiki/Largest_remainder_method
func calculatePercentages(data map[string]int, technologies []string) map[string]int {
	// Total production
	total := 0
	for _, v := range data {
		total += v
	}
	// Calculate production in percentages
	percentages := make(map[string]float64, len(data))
	for k, v := range data {
		percentages[k] = float64(v) / float64(total) * 100
	}
	// Floor percentages to integers
	floored := make(map[string]int, len(data))
	for k, v := range percentages {
		floored[k] = int(v)
	}
	// Compute difference of percentages and floored percentages
	remainders := make(map[string]float64, len(data))
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
	sort.Slice(technologies, func(i, j int) bool {
		return remainders[technologies[i]] > remainders[technologies[j]]
	})
	newPercentages := make(map[string]int, len(data))
	for _, resource := range technologies {
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
func prepareTweet(data map[string]int, technologies []string, mapping map[string][]rune) string {
	// Build list of runes representing the emoji characters
	// Sort resources by electricity production (descending)
	runesList := make([]rune, 0)
	sort.Slice(technologies, func(i, j int) bool {
		return data[technologies[i]] > data[technologies[j]]
	})
	for _, res := range technologies {
		count := data[res]
		emojiRunes := mapping[res]
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
	// Split the string into 10 lines with 10 emojis on line
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
	data, err := getEntsoeData(url)
	if err != nil {
		log.Fatal(err)
	}

	// Get share of each renewable technology on electrity production
	percentages := calculatePercentages(data, technologies)

	// Prepare string of emojis representing the production to tweet it
	myTweet := prepareTweet(percentages, technologies, runeMap)

	// Set your Twitter API credentials
	accessToken := os.Getenv("GOTWI_ACCESS_TOKEN")
	accessSecret := os.Getenv("GOTWI_ACCESS_TOKEN_SECRET")

	in := &gotwi.NewClientInput{
		AuthenticationMethod: gotwi.AuthenMethodOAuth1UserContext,
		OAuthToken:           accessToken,
		OAuthTokenSecret:     accessSecret,
	}

	c, err := gotwi.NewClient(in)
	if err != nil {
		fmt.Println(err)
		return
	}

	p := &types.CreateInput{
		Text: gotwi.String(myTweet),
	}

	res, err := managetweet.Create(context.Background(), c, p)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Printf("[%s] %s\n", gotwi.StringValue(res.Data.ID), gotwi.StringValue(res.Data.Text))

}
