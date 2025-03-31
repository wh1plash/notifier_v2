package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthdm/hollywood/actor"
	"github.com/wh1plash/notifier/receivers"
)

func main() {
	e, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		panic(err)
	}

	pid := e.Spawn(receivers.NewServer, "server", actor.WithID("system"))

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	<-sigch
	fmt.Println()
	<-e.Poison(pid).Done()

}
