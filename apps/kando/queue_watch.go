package main

import (
	"sync/atomic"
	"time"
)

// queueJump is how far the playhead must move between two of the player's reports, about a quarter of a second apart while it plays, to count as a jump the film being made gives way to (media.Watcher).
const queueJump = 3 * time.Second

// queueWatch is where the film the running job makes is watched (media.Watcher): where it will be watched from (core.Job.ResumeAt) until it is opened, then the open film's playhead, which the player's reports move (queuePlaying). Make reads it on its own goroutine.
type queueWatch struct {
	at     atomic.Int64
	jumped chan struct{}
}

func newQueueWatch(at time.Duration) *queueWatch {
	w := &queueWatch{jumped: make(chan struct{}, 1)}
	w.at.Store(int64(at))
	return w
}

func (w *queueWatch) Playhead() time.Duration { return time.Duration(w.at.Load()) }

func (w *queueWatch) Jumped() <-chan struct{} { return w.jumped }

// move puts the playhead at at, and says it jumped when it moved further than playing moves it.
func (w *queueWatch) move(at time.Duration) {
	was := time.Duration(w.at.Swap(int64(at)))
	if d := at - was; d > queueJump || d < -queueJump {
		select {
		case w.jumped <- struct{}{}:
		default:
		}
	}
}

// queuePlaying follows the open film's playhead into the running job's watch when the open film is the one it makes, so the lines nearest where it is watched are made first. It changes nothing a binding reads.
func (a *app) queuePlaying() bool {
	if w, p := a.queue.watch, a.player.play; w != nil && p != nil && a.openKey != "" && a.openKey == a.queue.active {
		w.move(p.head)
	}
	return false
}
