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

func TestOnce(t *testing.T) {
	hosts := FakeEndpoints()
	count := 0

	w := Once(10, hosts, func(host string) {
		count++
	})

	w.AfterEach(func() {
		if count != len(hosts) {
			t.Error("Expected 10, got", count)
		}
	})

	w.Finish()
}

func TestContinuousInterrupt(t *testing.T) {
	tick := make(chan struct{})
	waves := 10

	w := Continuous(10, FakeEndpoints(), func(host string) {
		tick <- struct{}{}
	})

	count := 0
	countWaves := 0

	w.AfterEach(func() {
		countWaves++
	})
	w.OnStop(func() {
		t.Log("Waves counted:", count)
	})

	go func() {
		for {
			<-tick
			if count++; count == (numPorts * waves) {
				t.Log("Ticks received:", count)
				w.Interrupt()
				break
			}
		}
	}()

	w.Start()
	w.Wait() // Interrupt will kick in later
}

func TestContinuousOnceFinish(t *testing.T) {
	tick := make(chan struct{})

	w := Continuous(10, FakeEndpoints(), func(host string) {
		tick <- struct{}{}
	})

	count := 0
	countWaves := 0

	w.AfterEach(func() {
		countWaves++
	})
	w.OnStop(func() {
		t.Log("Waves counted:", count)
	})

	go func() {
		for {
			<-tick
			if count++; count == numPorts {
				t.Log("Ticks received:", count)
				break
			}
		}
	}()

	w.Finish()
}
