package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/parser"
	"github.com/tcontardo/prettylogs/internal/record"
	"github.com/tcontardo/prettylogs/internal/store"
	"github.com/tcontardo/prettylogs/internal/tui"
	"github.com/tcontardo/prettylogs/internal/wrapper"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "prettylogs: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	format := flag.String("format", "", "force log format: json, logfmt, syslog, plain")
	configPath := flag.String("config", "", "path to YAML config (default ~/.prettyLogs/config)")
	forceInput := flag.Bool("input", false, "read logs from stdin")
	flag.Usage = usage
	flag.Parse()

	p, err := parser.ForFormat(*format)
	if err != nil {
		return err
	}
	cfg := config.Load(*configPath)

	stat, err := os.Stdin.Stat()
	if err != nil {
		return err
	}
	piped := stat.Mode()&os.ModeCharDevice == 0
	args := flag.Args()

	useStdin := *forceInput || (piped && len(args) == 0)
	if !useStdin && len(args) == 0 {
		usage()
		os.Exit(2)
	}

	logCh := make(chan record.Record, 256)
	var w *wrapper.Wrapper

	if useStdin {
		go func() {
			wrapper.StreamReader(os.Stdin, p, logCh)
			close(logCh)
		}()
	} else {
		w, err = wrapper.StartCommand(args[0], args[1:], p, logCh)
		if err != nil {
			return fmt.Errorf("start command: %w", err)
		}
	}

	model := tui.New(store.New(store.DefaultMaxSize), logCh, cfg, w)
	prog := tea.NewProgram(model)
	if _, err := prog.Run(); err != nil {
		if w != nil {
			w.Abandon()
			_ = w.Stop()
		}
		return err
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `prettylogs — interactive log viewer

Usage:
  prettylogs [flags] <command> [args...]
  prettylogs --input
  command | prettylogs

Flags:
  --format string   force format: json, logfmt, syslog, plain
  --config string   YAML config path (default ~/.prettyLogs/config)
  --input           read logs from stdin

Examples:
  prettylogs nx serve backend-service-api
  prettylogs npm run dev
  cat app.log | prettylogs
  prettylogs --format json npm run start
`)
}
