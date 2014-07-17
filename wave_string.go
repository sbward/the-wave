package wave

import (
	"errors"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// StringWave concurrently selects some strings, executes a set of Plugins
// for each string, then repeats the process for the whole pool of strings.
// Afterward it will pause before repeating, unless it's disabled.
type StringWave struct {
	Strings      []string // List of strings (typically server hostnames).
	Concurrency  int      // Number of strings to process simultaneously.
	WaitInterval int      // Seconds to wait between consecutive waves.
	Repeat       bool     // Set to true to repeat the wave continuously.
	Name         string   // A label used for log messages.

	// Plugins to execute against each string.
	Plugins []StringPlugin

	initialized    bool
	running        bool
	workerControls []chan bool
	workerCtrlLock sync.RWMutex
}

func (w *StringWave) SetConcurrency(c int) error {
	if c < 1 {
		return errors.New("Unable to set wave concurrency below 1")
	}
	w.Concurrency = c
	return nil
}

func (w *StringWave) SetWaitInterval(c int) error {
	if c < 0 {
		return errors.New("Unable to set wave wait interval below 0")
	}
	w.WaitInterval = c
	return nil
}

func (w *StringWave) SetRepeat(r bool) {
	w.Repeat = r
}

// Validate properties in case they were set directly and incorrectly.
func (w *StringWave) validate() error {
	if err := w.SetConcurrency(w.Concurrency); err != nil {
		return err
	}
	if err := w.SetWaitInterval(w.WaitInterval); err != nil {
		return err
	}
	return nil
}

// Do the wave!
func (w *StringWave) Run() error {
	// TODO inject a control channel?  Stop/Start
	return nil
}

func (w *StringWave) Start() error {
	if err := w.validate(); err != nil {
		return err
	}
	if !w.initialized {
		log.Println(w.Name, "Initializing Wave")
		if w.Name == "" {
			// Random name label.
			w.Name = "{{Wave " + strconv.Itoa(rand.Int()) + "}}"
		}
		go func() {
			first := true
			// Repeat the wave if configured to do so.
			for ; first || w.Repeat; time.Sleep(time.Duration(w.WaitInterval) * time.Second) {
				first = false
				log.Println(w.Name, "Launching")
				queue := make(chan string, w.Concurrency*3)
				wg := &sync.WaitGroup{}

				w.workerCtrlLock.Lock()
				w.workerControls = make([]chan bool, w.Concurrency)

				// Create the workers.
				for i := 0; i < w.Concurrency; i++ {
					ctrl := make(chan bool, 1)
					w.workerControls = append(w.workerControls, ctrl)
					wg.Add(1)
					go w.worker(queue, ctrl, wg)
				}
				w.workerCtrlLock.Unlock()

				// Feed all of the strings into the queue.
				for _, target := range w.Strings {
					log.Println(w.Name, "Sending", target)
					queue <- target
				}

				close(queue)
				wg.Wait()
				for _, plugin := range w.Plugins {
					plugin.WaveEnd(Wave(w))
				}
			}
		}()
		for _, plugin := range w.Plugins {
			plugin.WaveInit(Wave(w))
		}
		w.initialized = true
	}
	w.workerCtrlLock.RLock()
	for _, ctrl := range w.workerControls {
		ctrl <- true
	}
	w.workerCtrlLock.RUnlock()
	for _, plugin := range w.Plugins {
		plugin.WaveStart(Wave(w))
	}
	w.running = true
	return nil
}

func (w *StringWave) Pause() error {
	if !w.initialized {
		return errors.New("The wave cannot be paused as it was never started")
	}
	if !w.running {
		return errors.New("The wave is already paused")
	}
	for _, ctrl := range w.workerControls {
		ctrl <- false
	}
	for _, plugin := range w.Plugins {
		plugin.WavePause(Wave(w))
	}
	w.running = false
	return nil
}

func (w *StringWave) worker(queue chan string, ctrl chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(w.Name, "Worker launched")
work:
	for {
		select {
		case target, ok := <-queue:
			if !ok {
				log.Println(w.Name, "Worker quitting; queue exhausted")
				return // Work queue closed; quit.
			}
			for _, plugin := range w.Plugins {
				log.Println(w.Name, "Worker processing:", target)
				plugin.Session(Wave(w), target)
			}
		case running, ok := <-ctrl:
			if !ok {
				log.Println(w.Name, "Worker quitting; control channel closed")
				return // Control channel closed; quit.
			}
			if !running {
				log.Println(w.Name, "Worker entering idle mode")
				goto idle // Change state to idling.
			}
		}
	}
idle:
	for {
		select {
		case running, ok := <-ctrl:
			if !ok {
				log.Println(w.Name, "Worker quitting; control channel closed")
				return // Control channel closed; quit.
			}
			if running {
				log.Println(w.Name, "Worker entering work mode")
				goto work // Change state to working.
			}
		}
	}
}

type StringPlugin interface {
	BasePlugin
	Session(w Wave, target string) // Invoked for each string in a StringWave.
}
