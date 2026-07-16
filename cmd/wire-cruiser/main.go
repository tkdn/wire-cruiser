package main

import "os"

func main() {
	os.Exit(NewApp(os.Stdout, os.Stderr).Run(os.Args[1:]))
}
