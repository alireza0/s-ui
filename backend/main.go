package main

import (
	"log"
	"os"
	"os/signal"
	"s-ui/app"
	"syscall"
)

func main() {
	app := app.NewApp()

	err := app.Init()
	if err != nil {
		log.Fatal(err)
	}

	err = app.Start()
	if err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			app.RestartApp()
		default:
			app.Stop()
			return
		}
	}
}
