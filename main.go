package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/oklog/oklog/pkg/group"
)

const timestampFormat = "15:04:05"

func main() {
	// Initial validation.
	log.SetFlags(0)
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s 'hostname ; du -hs $HOME ; ps aux|grep system|head -n1' host1 [host2 ...]", os.Args[0])
	}

	// Parse the script arg.
	script := os.Args[1]
	log.Printf("running script: %q", script)

	// Parse the host args.
	hosts := os.Args[2:]
	plural := ""
	if len(hosts) > 1 {
		plural = "s"
	}
	log.Printf("%d host%s: %s", len(hosts), plural, strings.Join(hosts, " "))

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
