package main

import "fmt"
import "time"

func main() {
	start := float64(time.Now().UnixNano()) / (1000 * 1000 * 1000)
	var a float64 = 1
	for i := 0; i < 1000; i++ {
		a = a / 2.0 + a / 3.0
	}
	fmt.Println(a)
	finish := float64(time.Now().UnixNano()) / (1000 * 1000 * 1000)
	fmt.Println(finish - start)
}
