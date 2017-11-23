package main

import "fmt"

func main() {
	s := make([]byte, 10, 10)
	fmt.Println(s)
	s1 := []byte{1, 2, 3}
	s = s1
	fmt.Println(s)
}
