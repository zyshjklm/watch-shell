package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

func execute(ctx context.Context, hostname, script string, tabbed chan<- string) error {
	cmd := exec.CommandContext(ctx,
		"ssh", hostname, "--",
		"sh -c 'while true ; do "+script+" ; sleep 5 ; done'",
	)

	outpipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	errpipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	lines := make(chan string)
	go scanlines(outpipe, lines)
	go aggregate(ctx, lines, 250*time.Millisecond, "\t", tabbed)
	go loglines(errpipe)

	return cmd.Run()
}

func scanlines(r io.Reader, dst chan<- string) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		dst <- s.Text()
	}
}

func loglines(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		log.Printf("error: %s", s.Text())
	}
}

func aggregate(ctx context.Context, src <-chan string, debounce time.Duration, delimiter string, dst chan<- string) {
	var (
		flusht = time.NewTimer(debounce)
		flushc <-chan time.Time // initially nil
		buffer []string
	)
	for {
		select {
		case line := <-src:
			buffer = append(buffer, line)
			flusht.Reset(debounce)
			if flushc == nil {
				flushc = flusht.C
			}

		case <-flushc:
			buffer = append([]string{time.Now().Format(timestampFormat)}, buffer...)
			dst <- strings.Join(buffer, delimiter)
			buffer = buffer[:0]
			flushc = nil

		case <-ctx.Done():
			return
		}
	}
}
