package main

import "log"

func main() {
	store, err := NewPostgressStore()
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Init(); err != nil {
		log.Fatal(err)
	}

	server := NewApiserver(":3000", store)
	server.Run()
}
