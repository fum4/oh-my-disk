package main

import (
	"fmt"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"io"
	"strings"
	"sync"
	"time"
)

type phaseDisplay struct {
	once                sync.Once
	mu                  sync.Mutex
	p                   scanner.Progress
	started, phaseStart time.Time
	stop, done          chan struct{}
}

func startProgress(w io.Writer) *phaseDisplay {
	d := &phaseDisplay{started: time.Now(), phaseStart: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(d.done)
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		draw := func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			p := d.p
			if p.Phase == "" {
				return
			}
			path := strings.Map(func(r rune) rune {
				if r < 32 || r == 127 {
					return ' '
				}
				return r
			}, p.Path)
			r := []rune(path)
			if len(r) > 90 {
				path = "…" + string(r[len(r)-89:])
			}
			elapsed := time.Since(d.started).Seconds()
			fmt.Fprintf(w, "\r\033[K%s · %d entries · %d folders · %d pending · %.0f entries/s · %.1fs elapsed (phase %.1fs) · %s", p.Phase, p.Entries, p.Directories, p.Pending, float64(p.Entries)/max(elapsed, .001), elapsed, time.Since(d.phaseStart).Seconds(), path)
		}
		for {
			select {
			case <-tick.C:
				draw()
			case <-d.stop:
				draw()
				fmt.Fprintln(w)
				return
			}
		}
	}()
	return d
}
func (d *phaseDisplay) update(p scanner.Progress) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p.Phase != d.p.Phase {
		d.phaseStart = time.Now()
	}
	d.p = p
}
func (d *phaseDisplay) phase(s string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.p.Phase = s
	d.phaseStart = time.Now()
	d.p.Pending = 0
}
func (d *phaseDisplay) close() { d.once.Do(func() { close(d.stop); <-d.done }) }
