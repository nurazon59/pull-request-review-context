package main

import (
	"fmt"
	"os"

	"github.com/nurazon59/go-template"
)

func main() {
	if err := gotemplate.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
