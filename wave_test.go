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

func TestLaunch(t *testing.T) {
	hosts := FakeEndpoints()
	count := 0

	h := Launch(10, hosts, func(host string) {
		count++
	})

	<-h.Done()

	if count != len(hosts) {
		t.Error("Expected 10, got", count)
	}
}

func TestContinuous(t *testing.T) {
	tick := make(chan struct{})

	h := Continuous(10, FakeEndpoints(), func(host string) {
		tick <- struct{}{}
	})

	count := 0
	for _ = range tick {
		if count++; count == 100 {
			t.Log("Ticks received:", count)
			h.Stop()
			break
		}
	}
	<-h.Done()
}
