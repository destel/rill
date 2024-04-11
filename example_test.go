package rill_test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/destel/rill"
)

func Example_forEach() {
	items := rill.FromSlice([]string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9", "item10"}, nil)

	err := rill.ForEach(items, 3, func(item string) error {
		randomSleep(1 * time.Second) // emulate long processing
		fmt.Println(item)
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}

func randomSleep(max time.Duration) {
	time.Sleep(time.Duration(rand.Intn(int(max))))
}
