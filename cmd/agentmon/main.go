package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/heroku/agentmon"
)

var showVersion = flag.Bool("version", false, "print version string")

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("agentmon/v%s (built w/%s)\n", agentmon.VERSION, runtime.Version())
	}

	fmt.Println("Hello, world")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)

	// TODO: Do something with a final flush on TERM.

}
