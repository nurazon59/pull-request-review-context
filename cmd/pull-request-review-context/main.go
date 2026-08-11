package main

import (
	"fmt"
	"os"

	"github.com/nurazon59/pull-request-review-context"
)

func main() {
	if err := pullrequestreviewcontext.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
