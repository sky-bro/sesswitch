package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/toolenv"
)

func watchCommand(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("watch", flag.ContinueOnError)
	limit := flags.Int("limit", 100, "historical session limit; observed sessions are always included")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *limit < 1 || *limit > 1000 {
		return fmt.Errorf("limit must be between 1 and 1000")
	}
	ctx, stop := signal.NotifyContext(toolenv.Context(context.Background()), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return streamSnapshots(ctx, *limit, stdout, loadSnapshot)
}

type snapshotLoader func(context.Context, int) (snapshot, error)

func streamSnapshots(ctx context.Context, limit int, output io.Writer, load snapshotLoader) error {
	store, err := registry.New()
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	// Subscribe before reading the initial snapshot, so concurrent hook writes
	// are queued. Watch directories because writers use atomic rename.
	for _, dir := range store.WatchDirs() {
		if err := watcher.Add(dir); err != nil {
			return err
		}
	}
	initialCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	current, err := load(initialCtx, limit)
	cancel()
	if err != nil {
		return err
	}
	var last string
	emit := func() error {
		data, err := json.Marshal(current)
		if err != nil {
			return err
		}
		if string(data) == last {
			return nil
		}
		if _, err := fmt.Fprintln(output, string(data)); err != nil {
			return err
		}
		last = string(data)
		return nil
	}
	if err := emit(); err != nil {
		return err
	}
	// This timer only coalesces a burst of filesystem events. It never polls.
	var timer *time.Timer
	var pending <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("state watcher: %w", err)
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !recordKey(event.Name) {
				continue
			}
			if pending == nil {
				timer = time.NewTimer(50 * time.Millisecond)
				pending = timer.C
			}
		case <-pending:
			pending = nil
			current, err = projectLocal(store, limit, current)
			if err != nil {
				return err
			}
			if err := emit(); err != nil {
				return err
			}
		}
	}
}
