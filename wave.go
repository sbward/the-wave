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
	runtime.GOMAXPROCS(procs)
	log.Println("GOMAXPROCS = " + strconv.Itoa(procs))
}

const (
	defaultConcurrency  = 10
	defaultWaitInterval = 0
)

// Create a new Wave. Default implementation is the Wave.
func New(vals ...string) Wave {
	return Wave{
		strings:      vals,
		concurrency:  defaultConcurrency,
		waitInterval: defaultWaitInterval,
		name:         "{{ Wave " + strconv.Itoa(rand.Intn(1<<16)) + " }}",
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
	waveMasterCtrl chan bool       // Signals the master routine to stop/go.

	logger *log.Logger // Place to send log messages. Set to nil for no logs.
}

/*
	Setters
*/

func (w *Wave) SetConcurrency(c int)            { w.concurrency = c }
func (w *Wave) SetWaitInterval(t time.Duration) { w.waitInterval = t }
func (w *Wave) SetRepeat(r bool)                { w.repeat = r }
func (w *Wave) SetName(n string)                { w.name = n }

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

// Start or resume the wave. Provides a channel which signals whenever a
// full wave is completed. All channels created this way will receive a
// signal via fan-out messaging.
// Returns an error if already running or if the configuration is invalid.
func (w *Wave) Start() (<-chan struct{}, error) {
	if w.concurrency < 1 {
		return nil, errors.New("Concurrency cannot be below 1")
	}
	if w.waitInterval < 0 {
		return nil, errors.New("Wait interval cannot be below 0")
	}
	if w.name == "" {
		return nil, errors.New("Name cannot be an empty string")
	}
	if !w.initialized {
		w.waveDone = []chan struct{}{}
		w.running = true
		w.initialized = true
		w.waveMasterCtrl = make(chan bool)
	}
	done := make(chan struct{})
	w.waveDoneLock.Lock()
	w.waveDone = append(w.waveDone, done)
	w.waveDoneLock.Unlock()

	go func() {
		// Signal: Init
		log.Println(w.name, "Init")
		for _, plugin := range w.plugins {
			plugin.Init(w)
		}

		first := true
		// Repeat the wave if configured to do so.
		for ; first || w.repeat; time.Sleep(w.waitInterval) {
			first = false
			queue := make(chan string, w.concurrency*3)
			wg := &sync.WaitGroup{}

			w.workerControls = make([]chan bool, w.concurrency)

			// Create the workers.
			var preempt, run bool
			select {
			case cRun, ok := <-w.waveMasterCtrl:
				if !ok {
					log.Println(w.name, "Killed during startup")
					return
				}
				log.Println("Received preemptive master signal:", cRun)
				preempt = true
				run = cRun
				w.running = run
			default:
				// No signal received
			}
			for i := 0; i < w.concurrency; i++ {
				newCtrl := make(chan bool, 1)
				if preempt {
					newCtrl <- run
				}
				w.workerControls[i] = newCtrl
				wg.Add(1)
				go w.worker(queue, newCtrl, wg)
			}

			// Feed all of the strings into the queue.
			go func() {
				for _, target := range w.strings {
					log.Println(w.name, "Sending", target)
					queue <- target
				}
				close(queue)
			}()

			// Convert waitgroup signal into a chan signal
			wait := make(chan struct{})
			go func() {
				wg.Wait()
				wait <- struct{}{}
			}()

			// Wave master goroutine
			// Wait for signals or for work to finish
			go func() {
			master:
				for {
					select {
					case run, ok := <-w.waveMasterCtrl:
						if !ok {
							log.Println(w.name, "Killed during wave")
							for _, ctrl := range w.workerControls {
								close(ctrl)
							}
							// KILLED event?
							// Done signal?
						}
						log.Println("Received master signal:", run)
						for _, ctrl := range w.workerControls {
							ctrl <- run
						}
						w.running = run
					case <-wait:
						w.waveDoneLock.RLock()
						for _, done := range w.waveDone {
							done <- struct{}{}
						}
						w.waveDoneLock.RUnlock()
						// Signal: End
						log.Println(w.name, "End")
						for _, plugin := range w.plugins {
							plugin.End(w)
						}
						break master
					}
				}
			}()

			w.waveMasterCtrl <- true

			// Signal: Start
			log.Println(w.name, "Start")
			for _, plugin := range w.plugins {
				plugin.Start(w)
			}
		}
	}()

	return (<-chan struct{})(done), nil
}

// Pause the wave, allowing active sessions to finish.
// Returns an error if already paused.
func (w *Wave) Pause() error {
	w.waveMasterCtrl <- false

	// Signal: Pause
	log.Println(w.name, "Pause")
	for _, plugin := range w.plugins {
		plugin.Pause(w)
	}

	return nil
}

func (w *Wave) Unpause() error {
	w.waveMasterCtrl <- true

	// Signal: Unpause
	log.Println(w.name, "Unpause")
	for _, plugin := range w.plugins {
		plugin.Unpause(w)
	}

	return nil
}

func (w *Wave) IsPaused() bool {
	return !w.running
}

func (w *Wave) worker(queue chan string, ctrl chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(w.name, "Worker launched")
	select {
	case running, ok := <-ctrl:
		// There was a command waiting in the control channel
		if !ok {
			log.Println(w.name, "Worker quitting before work started; control channel closed")
			// TODO Report state to master
		}
		if running {
			log.Println(w.name, "Worker entering work mode; preemptive start command received")
			// TODO Report state to master
			goto work
		}
	default:
		// No command waiting, continue as usual...
		// TODO Report state to master
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
				// TODO Report state to master
				goto work // Change state to working.
			}
		}
	}
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
				// TODO Report state to master
				goto idle // Change state to idling.
			}
		}
	}
}
