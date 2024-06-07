package main

import (
	"flag"
	"fmt"

	"day03es/db"
	"day03es/web"
)

func main() {

	fAuth := flag.Bool("a", false, "Use authorization to get recommendations")
	fSetup := flag.Bool("s", false, "Add data into the database")
	flag.Parse()

	// Set up store
	store := db.NewElasticStore()

	if *fSetup {
		store.CreateIndex("places")
		store.ApplyMapping()
		store.AddData("../materials/data.csv")
	}

	// Create server on port 8888
	err := web.CreateServer(store, *fAuth)
	if err != nil {
		fmt.Printf("Failed to start the server: %s\n", err)
	}
}
