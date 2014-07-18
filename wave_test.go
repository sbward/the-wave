package wave

import (
	"strconv"
	"testing"
	"time"
)

/*
	TestPlugin helper
*/

type TestPlugin struct {
	*testing.T
	PauseOnStart bool
	PauseInvoked chan struct{} // Signals after Pause() is called by this plugin.
}

func (p *TestPlugin) Start(w *Wave) {
	p.T.Log("Plugin: Starting a wave")
	if p.PauseOnStart {
		if err := w.Pause(); err != nil {
			p.T.Error("Pause not expected to return an error, but received:", err)
		}
		p.PauseInvoked <- struct{}{}
	}
}

func (p *TestPlugin) Init(w *Wave)    { p.T.Log("Plugin: Initialized wave") }
func (p *TestPlugin) Pause(w *Wave)   { p.T.Log("Plugin: Pausing wave") }
func (p *TestPlugin) Unpause(w *Wave) { p.T.Log("Plugin: Unpausing wave") }
func (p *TestPlugin) End(w *Wave)     { p.T.Log("Plugin: Finished a wave") }

func (p *TestPlugin) Session(w *Wave, target string) {
	p.T.Log("Plugin: Target: " + target)
}

const (
	startPort = 3000 // Port to start on
	numPorts  = 10   // Number of ports to listen on
)

// FakeEndpoints creates a list of fake network port names.
// It is configured with the `startPort` and `numPorts` constants.
func FakeEndpoints() []string {
	var endpoints []string
	for i := 0; i < numPorts; i++ {
		port := ":" + strconv.Itoa(startPort+i)
		endpoints = append(endpoints, port)
	}
	return endpoints
}

func TestNormalWave(t *testing.T) {
	w := New(FakeEndpoints()...)

	// Cover some error handlers and setters.
	w.SetConcurrency(0)
	if _, err := w.Start(); err == nil {
		t.Error("Concurrency accepted 0")
		return
	}
	w.SetConcurrency(2)
	w.SetWaitInterval(time.Duration(-1))
	if _, err := w.Start(); err == nil {
		t.Error("WaitInterval accepted -1")
		return
	}
	w.SetWaitInterval(time.Second)
	w.SetName("")
	if _, err := w.Start(); err == nil {
		t.Error("Name accepted empty string")
		return
	}
	w.SetName("Test")
	w.SetRepeat(false)

	testPlugin := TestPlugin{
		T: t,
	}
	w.SetPlugins(Plugin(&testPlugin))

	// Getter assertions.
	if r := w.Repeat(); r != false {
		t.Error("Repeat value was unexpected:", r)
		return
	}
	if n := w.Name(); n != "Test" {
		t.Error("Name value was unexpected:", n)
		return
	}
	if l := len(w.Plugins()); l != 1 {
		t.Error("Plugins length was unexpected:", l)
	}

	done, err := w.Start()
	if err != nil {
		t.Error("Expected no errors, but received:", err)
		return
	}
	<-done
}

func TestPausingWave(t *testing.T) {
	w := New(FakeEndpoints()...)
	w.SetConcurrency(2)

	testPlugin := TestPlugin{
		T:            t,
		PauseOnStart: true,
		PauseInvoked: make(chan struct{}, 1),
	}
	w.SetPlugins(Plugin(&testPlugin))

	done, err := w.Start()
	if err != nil {
		t.Error("Expected no errors, but received:", err)
		return
	}

	<-testPlugin.PauseInvoked

	if !w.IsPaused() {
		t.Error("Expected wave to report it was paused, but it didn't")
		return
	}

	if err := w.Unpause(); err != nil {
		t.Error("Unexpected error:", err)
		return
	}
	<-done
}
