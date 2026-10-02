package main

import (
	"blog-aggregator/internal/config"
	"fmt"
)

func main() {
	configFile, err := config.Read()
	if err != nil {
		return
	}
	configFile.SetUser("lane")
	fmt.Println(config.Read())
}