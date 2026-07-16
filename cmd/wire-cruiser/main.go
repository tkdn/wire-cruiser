package main

import "os"

func main() {
	os.Exit(initializeApp().Run(os.Args[1:]))
}
