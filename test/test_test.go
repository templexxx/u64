package test

import (
	"fmt"
	"testing"
)

func showNumber(i int) {
	fmt.Println(i)
}

func TestGoSched(t *testing.T) {

	for i := 0; i < 10; i++ {
		go showNumber(i)
	}

	fmt.Println("Haha")
}
