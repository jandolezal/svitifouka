package main

import (
	"reflect"
	"testing"
)

var sampleData = map[string]int{
	"B01": 247,
	"B09": 0,
	"B11": 122,
	"B12": 126,
	"B15": 261,
	"B16": 867,
	"B19": 24,
}

var wantedPercentage = map[string]int{
	"B01": 15,
	"B09": 0,
	"B11": 7,
	"B12": 8,
	"B15": 16,
	"B16": 53,
	"B19": 1,
}

var wantedTweet = "☀️☀️☀️☀️☀️☀️☀️☀️☀️☀️\n☀️☀️☀️☀️☀️☀️☀️☀️☀️☀️\n☀️☀️☀️☀️☀️☀️☀️☀️☀️☀️\n☀️☀️☀️☀️☀️☀️☀️☀️☀️☀️\n☀️☀️☀️☀️☀️☀️☀️☀️☀️☀️\n☀️☀️☀️🌿🌿🌿🌿🌿🌿🌿\n🌿🌿🌿🌿🌿🌿🌿🌿🌿🌳\n🌳🌳🌳🌳🌳🌳🌳🌳🌳🌳\n🌳🌳🌳🌳💧💧💧💧💧💧\n💧💧💦💦💦💦💦💦💦🌬️\n"

func TestCalculatePercentages(t *testing.T) {
	got := calculatePercentages(sampleData)
	eq := reflect.DeepEqual(got, wantedPercentage)
	if eq {
		t.Log("The maps are equal.")
	} else {
		t.Errorf("%v\n%v\n", got, wantedPercentage)
	}
}

func TestPrepareTwet(t *testing.T) {
	got := prepareTweet(wantedPercentage)
	if got != wantedTweet {
		t.Errorf("%v\n%v\n", got, wantedTweet)
	}

}
