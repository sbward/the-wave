package wave

import (
	"strconv"
	"testing"
	"time"
)

const (
	startPort = 3000 // Port to start on
	numPorts  = 10   // Number of ports to listen on
)

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

	testPlugin := Plugin(&TestPlugin{t})
	w.SetPlugins(testPlugin)

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

type TestPlugin struct {
	*testing.T
}

func (t *TestPlugin) Start(w *Wave) {
	t.T.Log("Plugin: Starting wave")
}
func (t *TestPlugin) Pause(w *Wave) {
	t.T.Log("Plugin: Pausing wave")
}
func (t *TestPlugin) Init(w *Wave) {
	t.T.Log("Plugin: Initializing wave")
}
func (t *TestPlugin) End(w *Wave) {
	t.T.Log("Plugin: Finalizing wave")
}
func (t *TestPlugin) Session(w *Wave, target string) {
	t.T.Log("Plugin: Target: " + target)
}
