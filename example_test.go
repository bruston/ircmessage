package ircmessage_test

import (
	"fmt"
	"os"

	"github.com/bruston/ircmessage"
)

func Example() {
	scanner := ircmessage.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Message()
		fmt.Print(message)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading from standard input:", err)
	}
}
