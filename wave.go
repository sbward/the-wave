// Package wave implements a concurrent worker pool that can be used to easily
// communicate with a large amount of remote hosts. For example, if you have
// 500 servers, you can sweep over them while keeping at most 20 connections
// open during the sweep.
package wave

import (
	"sync"
)

// Once prepares a wave that will only execute one time. Call Start, Wait, or
// Finish on the returned handle to start the wave.
// Concurrency can be tuned by specifying the number of workers to launch.
// Behavior is configured by providing a callback that is passed one of the
// strings in the vals slice. For remote monitoring, this would probably be a
// hostname.
func Once(concurrency int, vals []string, callback func(string)) *Handle {
	h := newHandle()
	go func() {
		<-h.startChan
		doTheWave(concurrency, vals, callback, h)
		close(h.stopChan)
	}()
	return h
}

// Continuous prepares a wave that will automatically repeat unless stopped
// by a call to Interrupt or Finish on the returned handle. Call Start, Wait, or
// Finish on the returned handle to start the wave.
// Concurrency can be tuned by specifying the number of workers to launch.
// Behavior is configured by providing a callback that is passed one of the
// strings in the vals slice. For remote monitoring, this would probably be a
// hostname.
func Continuous(concurrency int, vals []string, callback func(string)) *Handle {
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

func doTheWave(concurrency int, vals []string, callback func(string), h *Handle) {
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
					val, ok := <-valChan
					if !ok {
						return
					}
					callback(val)
				}
			}
		}()
	}
	wg.Wait()
	h.trigger(h.eachFuncs)
}

// Handle is used to configure and control a wave.
type Handle struct {
	start, interrupt, finish sync.Once

	startChan     chan struct{} // Close to request start
	interruptChan chan struct{} // Close to request interrupt
	finishChan    chan struct{} // Close to request finish
	stopChan      chan struct{} // Close when stopped
	stopFuncs     []func()
	eachFuncs     []func()
	funcsLock     sync.RWMutex // Guards all []func()
}

func newHandle() *Handle {
	return &Handle{
		startChan:     make(chan struct{}),
		interruptChan: make(chan struct{}),
		finishChan:    make(chan struct{}),
		stopChan:      make(chan struct{}),
		stopFuncs:     []func(){},
		eachFuncs:     []func(){},
	}
}

func (h *Handle) trigger(fs []func()) {
	wg := sync.WaitGroup{}
	h.funcsLock.RLock()
	for _, f := range fs {
		wg.Add(1)
		go func() { f(); wg.Done() }()
	}
	h.funcsLock.RUnlock()
	wg.Wait()
}

// Start begins the wave.
func (h *Handle) Start() {
	h.start.Do(func() {
		close(h.startChan)
	})
}

// Interrupt allows running callbacks to finish while preventing the wave from
// continuing. It will block until all processing and callbacks have finished.
func (h *Handle) Interrupt() {
	h.interrupt.Do(func() {
		close(h.interruptChan)
	})
	h.Wait()
}

// Finish will block until the current wave is completed or interrupted.
// After that, no further waves will be started.
// For convenience, Finish also starts the wave if it hasn't started yet. In
// this scenario, the wave will only execute once.
func (h *Handle) Finish() {
	h.Start()
	h.finish.Do(func() {
		close(h.finishChan)
	})
	h.Wait()
}

// Wait blocks until the wave has stopped.
// For convenience, Wait also starts the wave if it hasn't started yet.
func (h *Handle) Wait() {
	<-h.stopChan
}

// OnStop registers a function to be called after the wave has stopped.
// Can be called multiple times to register multiple callbacks.
func (h *Handle) OnStop(f func()) {
	h.funcsLock.Lock()
	h.stopFuncs = append(h.stopFuncs, f)
	h.funcsLock.Unlock()
}

// AfterEach registers a function to be called after each full wave has completed.
// It will not be triggered after an interrupt.
// Can be called multiple times to register multiple callbacks.
func (h *Handle) AfterEach(f func()) {
	h.funcsLock.Lock()
	h.eachFuncs = append(h.eachFuncs, f)
	h.funcsLock.Unlock()
}
