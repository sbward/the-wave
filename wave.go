package wave

import (
	"sync"
)

// Once prepares a wave that will only execute one time after Start is called
// on the returned handle.
func Once(concurrency int, vals []string, callback func(string)) *handle {
	h := newHandle()
	go func() {
		<-h.startChan
		doTheWave(concurrency, vals, callback, h)
		close(h.stopChan)
	}()
	return h
}

func doTheWave(concurrency int, vals []string, callback func(string), h *handle) {
	valChan := make(chan string, 10)
	go func() {
		for _, val := range vals {
			valChan <- val
		}
		close(valChan)
	}()
	wg := sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-h.interruptChan:
					return
				default:
					select {
					case <-h.interruptChan:
						return
					case val, ok := <-valChan:
						if !ok {
							return
						}
						callback(val)
					}
				}
			}
		}()
	}
	wg.Wait()
	h.trigger(h.eachFuncs)
}

// Continuous prepares a wave that will repeat indefinitely unless interrupted
// by one of the available event triggers. Start must be called on the returned
// handle to begin the wave.
func Continuous(concurrency int, vals []string, callback func(string)) *handle {
	h := newHandle()
	go func() {
		<-h.startChan
		first := true
	loop:
		for {
			select {
			case <-h.interruptChan:
				break loop
			case <-h.finishChan:
				if first {
					doTheWave(concurrency, vals, callback, h)
					first = false
				}
				break loop
			default:
				doTheWave(concurrency, vals, callback, h)
				first = false
			}
		}
		close(h.stopChan)
	}()
	return h
}

type handle struct {
	start, interrupt, finish sync.Once

	startChan     chan struct{} // Close to request start
	interruptChan chan struct{} // Close to request interrupt
	finishChan    chan struct{} // Close to request finish
	stopChan      chan struct{} // Close when stopped
	stopFuncs     []func()
	eachFuncs     []func()
	funcsLock     sync.RWMutex // Guards all []func()
}

func newHandle() *handle {
	return &handle{
		startChan:     make(chan struct{}),
		interruptChan: make(chan struct{}),
		finishChan:    make(chan struct{}),
		stopChan:      make(chan struct{}),
		stopFuncs:     []func(){},
		eachFuncs:     []func(){},
	}
}

func (h *handle) trigger(fs []func()) {
	wg := sync.WaitGroup{}
	h.funcsLock.RLock()
	for _, f := range fs {
		wg.Add(1)
		go func() { f(); wg.Done() }()
	}
	h.funcsLock.RUnlock()
	wg.Wait()
}

// Start begins the wave. It must be called for the wave to proceed.
// For continuous waves, the first call to Start is all that is needed. The wave
// will loop continuously after that.
func (h *handle) Start() {
	h.start.Do(func() {
		close(h.startChan)
	})
}

// Interrupt allows running callbacks to finish while preventing the wave from
// continuing. It will block until all callbacks finish.
func (h *handle) Interrupt() {
	h.interrupt.Do(func() {
		close(h.interruptChan)
	})
	h.Wait()
}

// Finish will block until the current wave is completed, then no further waves
// will be started.
// For convenience, Finish starts the wave if it hasn't started yet.
// Note: if Interrupt is called, Finish will unblock whenever the wave stops,
// whether or not it truly finished an entire wave.  TODO return boolean
func (h *handle) Finish() {
	h.Start()
	h.finish.Do(func() {
		close(h.finishChan)
	})
	h.Wait()
}

// Wait blocks until the wave signals a Stop.
func (h *handle) Wait() {
	<-h.stopChan
}

// OnStop registers a function to be called when the wave has stopped for any
// reason.
func (h *handle) OnStop(f func()) {
	h.funcsLock.Lock()
	h.stopFuncs = append(h.stopFuncs, f)
	h.funcsLock.Unlock()
}

// AfterEach returns a channel that will send after each full wave has completed.
// It will not send unless the whole set of values has been processed.
func (h *handle) AfterEach(f func()) {
	h.funcsLock.Lock()
	h.eachFuncs = append(h.eachFuncs, f)
	h.funcsLock.Unlock()
}
