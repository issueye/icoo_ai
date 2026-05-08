package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Println("icoo-ai: use `serve`, `run`, `config`, or `doctor`")
		return nil
	}

	switch args[0] {
	case "serve":
		fmt.Println("icoo-ai serve is not implemented yet")
	case "run":
		fmt.Println("icoo-ai run is not implemented yet")
	case "config":
		fmt.Println("icoo-ai config is not implemented yet")
	case "doctor":
		fmt.Println("icoo-ai doctor is not implemented yet")
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}

	return nil
}
