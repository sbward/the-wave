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
	w.SetConcurrency(2)
	w.SetWaitInterval(time.Second)
	w.SetPlugins(Plugin(&TestPlugin{t}))

	done, err := w.Start()
	if err != nil {
		t.Error(err)
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
