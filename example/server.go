package main

import (
	"encoding/json"
	"fmt"
	"github.com/eshm"
	"time"
)

type Test struct {
	Key   int
	Value int
}

func main() {
	sm, err := eshm.NewShareMemory(&eshm.ShConfig{
		Key:  1,
		Size: 1024 * 1024,
	})
	if err != nil {
		panic(err)
	}
	defer sm.Close()

	for i := 0; i < 10000; i++ {
		t := &Test{
			Key:   i,
			Value: i,
		}

		js, _ := json.Marshal(t)
		if err := sm.Append(js); err != nil {
			fmt.Println("err:", err)
		}

		time.Sleep(time.Millisecond * 10)
	}
}
