package wave

import "sync"

func Launch(concurrency int, vals []string, callback func(string)) *handle {
	h := newHandle()
	launchWithHandle(concurrency, vals, callback, h)
	return h
}

func launchWithHandle(concurrency int, vals []string, callback func(string), h *handle) {
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
			for {
				select {
				case _, ok := <-h.stopChan:
					if !ok {
						return
					}
				default:
				}
				select {
				case val, ok := <-valChan:
					if !ok {
						wg.Done()
						return
					}
					callback(val)
				}
			}
		}()
	}
	wg.Wait()
	h.sendDone()
}

func Continuous(concurrency int, vals []string, callback func(string)) *handle {
	h := newHandle()
	go func() {
	loop:
		for {
			select {
			case _, ok := <-h.stopChan:
				if !ok {
					break loop
				}
			default:
				launchWithHandle(concurrency, vals, callback, h)
			}
		}
		h.sendDone()
	}()
	return h
}

type handle struct {
	stop, done    bool
	stopChan      chan struct{}
	doneChans     []chan struct{}
	doneChansLock sync.RWMutex
}

func newHandle() *handle {
	return &handle{
		stopChan:  make(chan struct{}),
		doneChans: make([]chan struct{}, 0),
	}
}

// Stop allows running callbacks to finish while preventing the wave from
// continuing. When all callbacks return, the "Done" event will be triggered.
func (h *handle) Stop() {
	if !h.stop {
		close(h.stopChan)
		h.stop = true
	}
}

// Done returns a channel that will strobe when the wave completes, or after
// callbacks have finished from calling Stop().
// For continuous waves, the channel will strobe after each wave, and after
// callbacks have finished from calling Stop().
func (h *handle) Done() <-chan struct{} {
	doneChan := make(chan struct{}, 1)
	h.doneChansLock.Lock()
	h.doneChans = append(h.doneChans, doneChan)
	h.doneChansLock.Unlock()
	if h.done {
		doneChan <- struct{}{}
	}
	return (<-chan struct{})(doneChan)
}

func (h *handle) sendDone() {
	h.done = true
	h.doneChansLock.RLock()
	for _, done := range h.doneChans {
		select {
		case done <- struct{}{}:
		default:
		}
	}
	h.doneChansLock.RUnlock()
}
