package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/oklog/oklog/pkg/group"
)

func render(ctx context.Context, rows []<-chan string, dst io.Writer) error {
	// Cache the latest update from each row in a buffer.
	buffers := make([]syncBuffer, len(rows))

	// Whenever any buffer changes, we'll want to repaint.
	// Here's a channel to manage that signaling.
	repaintc := make(chan struct{}, 1) // recv
	repaintf := func() {               // send
		select {
		case repaintc <- struct{}{}:
		default: // best-effort
		}
	}

	// Control everything here with a group.
	var g group.Group

	// Actors to update individual buffers.
	for i := range rows {
		src, dst := rows[i], &buffers[i]
		subctx, cancel := context.WithCancel(ctx)
		g.Add(func() error {
			return update(subctx, src, dst, repaintf)
		}, func(error) {
			cancel()
		})
	}

	// An actor to receive repaint signals and render out.
	{
		subctx, cancel := context.WithCancel(ctx)
		g.Add(func() error {
			return renderLoop(subctx, repaintc, buffers, dst)
		}, func(error) {
			cancel()
		})
	}

	// That should do it.
	return g.Run()
}

func update(ctx context.Context, src <-chan string, dst io.Writer, repaint func()) error {
	for {
		select {
		case s := <-src:
			fmt.Fprint(dst, s)
			repaint()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func renderLoop(ctx context.Context, repaint <-chan struct{}, buffers []syncBuffer, dst io.Writer) error {
	for {
		select {
		case <-repaint:
			println("\033[H\033[2J")
			renderOnce(buffers, dst)
			fmt.Fprintf(dst, "%s\n", time.Now().Format(timestampFormat))
			fmt.Fprintln(dst)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func renderOnce(buffers []syncBuffer, dst io.Writer) error {
	tw := tabwriter.NewWriter(dst, 0, 0, 2, ' ', 0)
	for i := range buffers {
		fmt.Fprintln(tw, (&buffers[i]).String())
	}
	return tw.Flush()
}

type syncBuffer struct {
	mtx sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	b.buf.Reset()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mtx.Lock()
	defer b.mtx.Unlock()
	return b.buf.String()
}
