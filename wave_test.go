package wave

import (
	"strconv"
	"testing"
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

func TestStringWave(t *testing.T) {
	endpoints := FakeEndpoints()
	numWorkers := 2

	done, err := (&StringWave{
		Strings:      endpoints,
		Concurrency:  numWorkers,
		WaitInterval: 1,
		Plugins: []StringPlugin{
			StringPlugin(&TestPlugin{t}),
		},
	}).Start()

	if err != nil {
		t.Error(err)
	}

	<-done
}

type TestPlugin struct {
	*testing.T
}

func (t *TestPlugin) WaveStart(w Wave) {
	t.T.Log("Plugin: Starting wave")
}
func (t *TestPlugin) WavePause(w Wave) {
	t.T.Log("Plugin: Pausing wave")
}
func (t *TestPlugin) WaveInit(w Wave) {
	t.T.Log("Plugin: Initializing wave")
}
func (t *TestPlugin) WaveEnd(w Wave) {
	t.T.Log("Plugin: Finalizing wave")
}
func (t *TestPlugin) Session(w Wave, target string) {
	t.T.Log("Plugin: Target: " + target)
}
