package main

import (
	"encoding/json"
	"fmt"
	"github.com/eshm"
	"time"
)

func main() {
	sm, err := eshm.GetShareMemory(&eshm.ShConfig{
		Key:  1,
		Size: 1024 * 1024,
	})
	if err != nil {
		panic(err)
	}
	defer sm.Close()

	for {
		value, err := sm.Get()
		if err != nil {
			fmt.Println("err:", err)
			continue
		}

		for _, v := range value {
			t := &Test{}
			_ = json.Unmarshal(v, t)
			fmt.Println("test", t)
		}
		time.Sleep(time.Second)
	}
}
