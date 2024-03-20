package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/destel/rill/echans"
)

type Measurement struct {
	Date     time.Time
	Temp     float64
	Movement float64
}

func main() {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	err := printTemperatureMovements(context.Background(), "New York", startDate, endDate)
	if err != nil {
		fmt.Println("Error:", err)
	}
}

func printTemperatureMovements(ctx context.Context, city string, startDate, endDate time.Time) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // In case of error or early exit, this ensures all http are canceled

	// Make a channel that emits all the days between startDate and endDate
	days := make(chan echans.Try[time.Time])
	go func() {
		defer close(days)
		for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
			days <- echans.Try[time.Time]{V: date}
		}
	}()

	// Download the temperature for each day in parallel and in order
	measurements := echans.OrderedMap(days, 10, func(date time.Time) (Measurement, error) {
		temp, err := getTemperature(ctx, city, date)
		return Measurement{Date: date, Temp: temp}, err
	})

	// Calculate the temperature movements. Use a single goroutine
	prev := Measurement{Temp: math.NaN()}
	measurements = echans.OrderedMap(measurements, 1, func(m Measurement) (Measurement, error) {
		m.Movement = m.Temp - prev.Temp
		prev = m
		return m, nil
	})

	// Iterate over the measurements and print the movements
	err := echans.ForEach(measurements, 1, func(m Measurement) error {
		fmt.Printf("%s: %.1f°C (movement %+.1f°C)\n", m.Date.Format("2006-01-02"), m.Temp, m.Movement)
		prev = m
		return nil
	})

	return err
}

func getTemperature(ctx context.Context, city string, date time.Time) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	// Simulate a network request
	randomSleep(1000 * time.Millisecond)

	// Basic city hash, to make measurements unique for each city
	var h float64
	for _, c := range city {
		h += float64(c)
	}

	// Simulate a temperature reading, by retuning a pseudo-random, but deterministic value
	temp := 15 - 10*math.Sin(h+float64(date.Unix()))

	return temp, nil
}

func randomSleep(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
