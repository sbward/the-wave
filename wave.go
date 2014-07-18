package wave

import (
	"errors"
	"flag"
	"log"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"time"
)

func init() {
	cpus := runtime.NumCPU()
	procs := *flag.Int("wave.procs", cpus, "Number of processors to use")
	if procs < 1 || procs > cpus {
		panic("Value for -wave.procs must be between 1 and " + strconv.Itoa(cpus))
	}
	runtime.GOMAXPROCS(procs)
	log.Println("GOMAXPROCS = " + strconv.Itoa(procs))
}

// Create a new Wave. Default implementation is the Wave.
func New(vals ...string) Wave {
	return Wave{
		strings: vals,
	}
}

// Wave concurrently selects some strings, executes a set of Plugins
// for each string, then continues the process for the whole pool of strings.
// Afterward it will pause before repeating, unless it's disabled via SetRepeat.
type Wave struct {
	strings      []string      // List of strings (typically server hostnames).
	concurrency  int           // Number of strings to process simultaneously.
	waitInterval time.Duration // Seconds to wait between consecutive waves.
	repeat       bool          // Set to true to repeat the wave continuously.
	name         string        // A label used for log messages.
	plugins      []Plugin      // Plugins to execute against each string.

	initialized    bool            // True if the wave goroutine was created.
	running        bool            // True if the wave is executing right now.
	waveDone       []chan struct{} // Publishes a signal at the end of waves.
	waveDoneLock   sync.RWMutex    // Guards access to waveDone slice.
	workerControls []chan bool     // Send true to start worker, false to stop.
	workerCtrlLock sync.RWMutex    // Guards access to workerControls slice.
}

/*
	Setters
*/

func (w *Wave) SetConcurrency(c int) error {
	if c < 1 {
		return errors.New("Unable to set wave concurrency below 1")
	}
	w.concurrency = c
	return nil
}

func (w *Wave) SetWaitInterval(t time.Duration) error {
	if t < 0 {
		return errors.New("Unable to set wave wait interval below 0")
	}
	w.waitInterval = t
	return nil
}

func (w *Wave) SetRepeat(r bool) {
	w.repeat = r
}

func (w *Wave) SetName(n string) error {
	if n == "" {
		return errors.New("Name cannot be an empty string")
	}
	w.name = n
	return nil
}

func (w *Wave) SetPlugins(ps ...Plugin) {
	w.plugins = []Plugin{}
	w.AddPlugins(ps...)
}

func (w *Wave) AddPlugins(ps ...Plugin) {
	w.plugins = append(w.plugins, ps...)
}

/*
	Getters
*/

func (w *Wave) Concurrency() int            { return w.concurrency }
func (w *Wave) WaitInterval() time.Duration { return w.waitInterval }
func (w *Wave) Repeat() bool                { return w.repeat }
func (w *Wave) Name() string                { return w.name }

func (w *Wave) Plugins() []Plugin {
	bps := []Plugin{}
	for _, sp := range w.plugins {
		bps = append(bps, sp.(Plugin))
	}
	return bps
}

/*
	Implementation of Wave interface
*/

// Validate properties in case they were set internally and incorrectly.
func (w *Wave) validate() error {
	if err := w.SetConcurrency(w.Concurrency()); err != nil {
		return err
	}
	if err := w.SetWaitInterval(w.WaitInterval()); err != nil {
		return err
	}
	return nil
}

// Start or resume the wave. Provides a channel which signals whenever a
// full wave is completed. All channels created this way will receive a
// signal via fan-out messaging.
// Returns an error if already running or if the configuration is invalid.
func (w *Wave) Start() (<-chan struct{}, error) {
	if err := w.validate(); err != nil {
		return nil, err
	}
	if !w.initialized {
		if w.name == "" {
			// Random name label.
			w.name = "{{ Wave " + strconv.Itoa(rand.Int()) + " }}"
		}
		log.Println(w.name, "Initializing")
		w.waveDone = []chan struct{}{}
		go func() {
			first := true
			// Repeat the wave if configured to do so.
			for ; first || w.repeat; time.Sleep(w.waitInterval) {
				first = false
				log.Println(w.name, "Launching")
				queue := make(chan string, w.concurrency*3)
				wg := &sync.WaitGroup{}

				w.workerCtrlLock.Lock()
				w.workerControls = make([]chan bool, w.concurrency)

				// Create the workers.
				for i := 0; i < w.concurrency; i++ {
					ctrl := make(chan bool, 1)
					w.workerControls = append(w.workerControls, ctrl)
					wg.Add(1)
					go w.worker(queue, ctrl, wg)
				}
				w.workerCtrlLock.Unlock()

				// Feed all of the strings into the queue.
				for _, target := range w.strings {
					log.Println(w.name, "Sending", target)
					queue <- target
				}

				close(queue)
				wg.Wait()
				for _, plugin := range w.plugins {
					plugin.End(w)
				}
				w.waveDoneLock.RLock()
				for _, done := range w.waveDone {
					done <- struct{}{}
				}
				w.waveDoneLock.RUnlock()
			}
		}()
		for _, plugin := range w.plugins {
			plugin.Init(w)
		}
		w.initialized = true
	}
	w.workerCtrlLock.RLock()
	for _, ctrl := range w.workerControls {
		ctrl <- true
	}
	w.workerCtrlLock.RUnlock()
	for _, plugin := range w.plugins {
		plugin.Start(w)
	}
	w.running = true
	done := make(chan struct{})
	w.waveDoneLock.Lock()
	w.waveDone = append(w.waveDone, done)
	w.waveDoneLock.Unlock()
	return (<-chan struct{})(done), nil
}

// Pause the wave, allowing active sessions to finish.
// Returns an error if already paused.
func (w *Wave) Pause() error {
	if !w.initialized {
		return errors.New("The wave cannot be paused as it was never started")
	}
	if !w.running {
		return errors.New("The wave is already paused")
	}
	for _, ctrl := range w.workerControls {
		ctrl <- false
	}
	for _, plugin := range w.plugins {
		plugin.Pause(w)
	}
	w.running = false
	return nil
}

func (w *Wave) worker(queue chan string, ctrl chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(w.name, "Worker launched")
work:
	for {
		select {
		case target, ok := <-queue:
			if !ok {
				log.Println(w.name, "Worker quitting; queue exhausted")
				return // Work queue closed; quit.
			}
			for _, plugin := range w.plugins {
				log.Println(w.name, "Worker processing:", target)
				plugin.Session(w, target)
			}
		case running, ok := <-ctrl:
			if !ok {
				log.Println(w.name, "Worker quitting; control channel closed")
				return // Control channel closed; quit.
			}
			if !running {
				log.Println(w.name, "Worker entering idle mode")
				goto idle // Change state to idling.
			}
		}
	}
idle:
	for {
		select {
		case running, ok := <-ctrl:
			if !ok {
				log.Println(w.name, "Worker quitting; control channel closed")
				return // Control channel closed; quit.
			}
			if running {
				log.Println(w.name, "Worker entering work mode")
				goto work // Change state to working.
			}
		}
	}
}

/*
	Plugins
*/

// A plugin is a set of event hooks which are invoked during a wave.
// Each hook must be implemented, but they can be empty if you want to skip one.
// Plugins are used concurrently by multiple goroutines and must be threadsafe.
type Plugin interface {
	Init(*Wave)            // Called immediately before a full wave is started.
	Pause(*Wave)           // Signals the wave is going into a paused state.
	Start(*Wave)           // Signals the wave is starting or resuming from a paused state.
	End(*Wave)             // Called immediately after a full wave is completed.
	Session(*Wave, string) // Invoked for each string in a Wave.
}
