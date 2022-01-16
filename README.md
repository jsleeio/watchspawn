# watchspawn

## what is it?

Watches for file creates and writes in and below the current directory and when
any of them (matching a suffix list) change, runs a command.

Command execution is damped somewhat by a short minimum "idle" time (250ms
default) to ensure duplicate (Vim does this) or batched writes do not cause
extra useless executions.

## usage

Pretty simple. Say you were working on a Go project in your editor --- open
a(nother) terminal and:

    watchspawn go build

That's it!

## options

    $ ./watchspawn -help
    usage: ./watchspawn [options] COMMAND [ARGS...]

    options:
      -min-wait duration
          delay command execution after interesting events (default 250ms)
      -suffixes string
          comma-separated list of interesting filename suffixes (default "go,mod,sum")

## key bindings

Vi key bindings (h/j/k/l/g/G and ^F/^B) scroll output text if required.

## bugs / room for improvement

* commands that don't generate output (like a successful Go build) result in no
  visible feedback, so you can't see if they're finished or not
* `q` to quit doesn't actually work; I need to understand `tview` better.
  Ctrl-C (twice) does work, though.
