package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/oklog/oklog/pkg/group"
)

const timestampFormat = "15:04:05"

func main() {
	log.SetFlags(0)
	usage := fmt.Sprintf("usage: %s command [command ...] -- host1 [host2 ...]", os.Args[0])

	// Find the -- split.
	var split int
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--" {
			split = i
			break
		}
	}
	if split == 0 {
		log.Fatal(usage)
	}

	// Print the script.
	script := os.Args[1:split]
	if len(script) <= 0 {
		log.Fatal(usage)
	}
	log.Printf("running %d-element script", len(script))

	// Print the hosts.
	hosts := os.Args[split+1:]
	if len(hosts) <= 0 {
		log.Fatal(usage)
	}
	plural := "s"
	if len(hosts) == 1 {
		plural = ""
	}
	log.Printf("running against %d host%s", len(hosts), plural)

	// Each host will output to a channel, a row in the table.
	outputs := make([]chan string, len(hosts))
	for i := range outputs {
		outputs[i] = make(chan string)
	}

	// Create the run group, with the ctrl-C handler.
	var g group.Group
	{
		cancel := make(chan struct{})
		g.Add(func() error {
			return interrupt(cancel)
		}, func(error) {
			close(cancel)
		})
	}

	// Launch each host's execute func as an actor.
	for i := range hosts {
		host, output := hosts[i], outputs[i]
		ctx, cancel := context.WithCancel(context.Background())
		g.Add(func() error {
			return execute(ctx, host, script, output)
		}, func(error) {
			cancel()
		})
	}

	// Launch the rendering actor.
	{
		recvoutputs := make([]<-chan string, len(outputs))
		for i := range outputs {
			recvoutputs[i] = outputs[i]
		}
		ctx, cancel := context.WithCancel(context.Background())
		g.Add(func() error {
			return render(ctx, recvoutputs, os.Stdout)
		}, func(error) {
			cancel()
		})
	}

	// Run.
	log.Print("connecting and starting...")
	log.Print(g.Run())
}

func interrupt(cancel <-chan struct{}) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-c:
		return fmt.Errorf("received signal %s", sig)
	case <-cancel:
		return errors.New("canceled")
	}
}
