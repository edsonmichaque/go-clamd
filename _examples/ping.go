package main

import (
	"context"
	"fmt"
	"log"

	"github.com/edsonmichaque/go-clamd"
)

func main() {
	log.Println("Create client")

	client, err := clamd.New(
		&clamd.Options{
			Network: "tcp",
			Address: "localhost:3310",
		},
	)

	log.Println("Pingging")

	resp, err := client.Ping(context.Background())
	if err != nil {
		doPanic("Ping: %v", err)
	}

	fmt.Printf("%v", resp)

	log.Println("Pingging")

	resp2, err := client.Version(context.Background())
	if err != nil {
		doPanic("Ping: %v", err)
	}

	fmt.Printf("%v", resp2)
}

func doPanic(f string, args ...interface{}) {
	panic(fmt.Sprintf(f, args...))
}
