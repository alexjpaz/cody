package main

import (
	"fmt"
	"log"

	"example.com/filterpicker/picker"
)

func main() {
	choice, err := picker.Run()
	if err != nil {
		log.Fatal(err)
	}
	// Print the selected/accepted line
	fmt.Println(choice)
}
