package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rivo/tview"
)

type config struct {
	command  string
	args     []string
	suffixes []string
	minWait  time.Duration
}

// conveniently combining strings.Split and strings.TrimSpace
func splitTrimSpace(s, sep string) []string {
	bits := strings.Split(s, sep)
	v := make([]string, len(bits))
	for idx, bit := range bits {
		v[idx] = strings.TrimSpace(bit)
	}
	return v
}

func configure() (config, error) {
	c := config{}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [options] COMMAND [ARGS...]\n", flag.CommandLine.Name())
		fmt.Fprintln(flag.CommandLine.Output(), "\noptions:")
		flag.PrintDefaults()
	}
	flag.DurationVar(&c.minWait, "min-wait", 250*time.Millisecond, "delay command execution after interesting events")
	suff := flag.String("suffixes", "go,mod,sum", "comma-separated list of interesting filename suffixes")
	c.suffixes = splitTrimSpace(*suff, ",")
	flag.Parse()
	if flag.NArg() < 1 {
		return c, fmt.Errorf("a command must be specified")
	}
	args := flag.Args()
	c.command = args[0]
	if len(args) > 1 {
		c.args = args[1:]
	}
	return c, nil
}

type eventFilter struct {
	ops      []fsnotify.Op
	suffixes []string
	cache    map[string]bool
	minWait  time.Duration
}

func newEventFilter(suffixes []string, ops ...fsnotify.Op) eventFilter {
	return eventFilter{
		suffixes: suffixes,
		ops:      ops,
		cache:    make(map[string]bool),
	}
}

func (ef *eventFilter) check(event fsnotify.Event) bool {
	interestingOp := false
	for _, op := range ef.ops {
		interestingOp = interestingOp || (event.Op&op == op)
	}
	if !interestingOp {
		// boring event, bye felicia
		return false
	}
	if interestingName, found := ef.cache[event.Name]; found {
		// we've seen this before
		return interestingName
	}
	suffix := strings.TrimLeft(path.Ext(event.Name), ".")
	for _, s := range ef.suffixes {
		if suffix == s {
			ef.cache[event.Name] = true
			// yep, we like it
			return true
		}
	}
	// boring
	ef.cache[event.Name] = false
	return false
}

func main() {
	var cfg config
	var err error
	if cfg, err = configure(); err != nil {
		fmt.Fprintf(os.Stderr, "configuring: %v\n", err)
		os.Exit(1)
	}
	var watcher *fsnotify.Watcher
	if watcher, err = fsnotify.NewWatcher(); err != nil {
		fmt.Fprintf(os.Stderr, "creating watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()
	filter := newEventFilter(cfg.suffixes, fsnotify.Write, fsnotify.Create)
	app := tview.NewApplication()
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() { app.Draw() })
	done := make(chan bool, 1)
	doit := make(chan bool, 1)
	go func() {
		var last time.Time
		wakeup := time.NewTicker(cfg.minWait / 4)
		waiting := false
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filter.check(event) {
					last = time.Now()
					waiting = true
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			case tick := <-wakeup.C:
				age := tick.Sub(last)
				if waiting && age > cfg.minWait {
					doit <- true
				}
			case <-doit:
				textView.Clear()
				w := textView.BatchWriter()
				cmd := exec.Command(cfg.command, cfg.args...)
				output, err := cmd.CombinedOutput()
				if err != nil {
					switch t := err.(type) {
					case *exec.ExitError:
						fmt.Fprintf(w, "command returned nonzero exit status: %v\n", t.ProcessState.ExitCode())
					default:
						fmt.Fprintf(w, "invoking command: %v\n", err)
					}
				}
				fmt.Fprintln(w, string(output))
				waiting = false
				w.Close()
			}
		}
	}()
	if err := watcher.Add("."); err != nil {
		fmt.Fprintf(os.Stderr, "adding watch path: %v\n", err)
		os.Exit(1)
	}
	if err := app.SetRoot(textView, true).SetFocus(textView).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "setting up terminal UI: %v\n", err)
		os.Exit(1)
	}
	<-done
}
