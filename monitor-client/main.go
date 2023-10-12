package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/gek64/displayController"
	"github.com/getlantern/systray"
	"github.com/getlantern/systray/example/icon"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

var queryAPI api.QueryAPI
var brightnessItem *systray.MenuItem
var lightLevelItem *systray.MenuItem

// runtime stuff
var brightnessGoal = 0
var currentBrightness = 0
var lastBrightnessChange = time.Time{}

func init() {
	// connect to to InfluxDB
	client := influxdb2.NewClient("http://192.168.0.22:8086", "6kE5U2R5YcHw4L3LZqFjSyqBkVm7Hxv_zCUgnl9IoSQy2Zfadv003AInlh0SmXJCLgvwQix3d7-7Hxr4TGtbkA==")
	queryAPI = client.QueryAPI("Sorensen Cloud")
}

func main() {
	go Runtime()

	// create systray icon
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTitle("Light Wizard")
	systray.SetTooltip("It's working!")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	brightnessItem = systray.AddMenuItem("", "Brightness")
	brightnessItem.Disable()
	lightLevelItem = systray.AddMenuItem("", "Light Level")
	lightLevelItem.Disable()
}

func onExit() {
	os.Exit(0)
}

func Runtime() {
	//runtime variables
	// currentBrightness := 0

	go BrightnessOrchestrator()

	for {
		currentLightLevel := GetCurrentLightLevel()
		// fmt.Printf("Current light level: %f lux\n", currentLightLevel)

		lightLevelItem.SetTitle(fmt.Sprintf("Light Level: %f lux", currentLightLevel))

		brightness := LuxToBrightness(currentLightLevel)

		// if the brightness has changed by more than 5%, set the brightness to the new value
		if brightness != currentBrightness && (brightness-currentBrightness <= -3 || brightness-currentBrightness >= 5) {
			QueueBrightnessChange(brightness)
		} else if brightnessGoal == 0 {
			QueueBrightnessChange(brightness)
		}

		// if the brightness has not changed for 15 seconds, set the brightness to the new value
		if time.Since(lastBrightnessChange) > time.Second*15 && currentBrightness != brightness {
			fmt.Println("brightness change wanted for >15 seconds")
			QueueBrightnessChange(brightness)
		}

		time.Sleep(time.Millisecond * 500)
	}
}

func QueueBrightnessChange(brightness int) {
	brightnessGoal = brightness
	lastBrightnessChange = time.Now()
}

func BrightnessOrchestrator() {
	// get monitors
	compositeMonitors, err := displayController.GetCompositeMonitors()
	if err != nil {
		log.Fatal(err)
	}

	// get current brightness
	currentValue, _, err := displayController.GetVCPFeatureAndVCPFeatureReply(compositeMonitors[0].PhysicalInfo.Handle, displayController.Brightness)
	if err != nil {
		log.Fatal(err)
	}
	currentBrightness = currentValue

	// start loop to move set current brightness to brightness goal in increments of 1
	for {
		if currentBrightness != brightnessGoal {
			delta := 0

			if math.Signbit(float64(brightnessGoal - currentBrightness)) {
				currentBrightness--
				delta = -1
			} else {
				currentBrightness++
				delta = 1
			}

			SetMonitorBrightness(compositeMonitors, delta, currentBrightness-delta)

			fmt.Printf("Setting brightness to %d%%\n", currentBrightness)
			brightnessItem.SetTitle(fmt.Sprintf("Brightness: %d%%", currentBrightness))
		}

		time.Sleep(time.Millisecond * 5)
	}
}

// GetCurrentLightLevel returns the current light level in the living room
func GetCurrentLightLevel() float64 {
	result, err := queryAPI.Query(context.Background(), `from(bucket: "Flat-Prod")
	|> range(start: -30s)
	|> filter(fn: (r) => r["_measurement"] == "ambient_light_level")
	|> filter(fn: (r) => r["_field"] == "intensity")
	|> filter(fn: (r) => r["device"] == "living-room-1")
	|> aggregateWindow(every: 1s, fn: last, createEmpty: false)
	|> last()
	|> yield(name: "last")`)
	if err == nil {
		// Iterate over query response
		for result.Next() {
			// Access dat
			return result.Record().Value().(float64)
		}
		// check for an error
		if result.Err() != nil {
			fmt.Printf("query parsing error: %s\n", result.Err().Error())
		}
	} else {
		panic(err)
	}

	return 0
}

// SetMonitorBrightness sets the brightness of all monitors to the given value
func SetMonitorBrightness(monitors []displayController.CompositeMonitorInfo, delta int, fromBrightness int) {
	for _, compositeMonitor := range monitors {
		// Set the brightness of the current display to current value
		err := displayController.SetVCPFeature(compositeMonitor.PhysicalInfo.Handle, displayController.Brightness, fromBrightness+delta)
		if err != nil {
			fmt.Println(err)
		}
	}
}

// LuxToBrightness converts a lux value to a brightness value
func LuxToBrightness(lux float64) int {
	calculatedBrightness := int(lux / 5)

	if calculatedBrightness > 100 {
		calculatedBrightness = 100
	}

	return calculatedBrightness + 5
}
