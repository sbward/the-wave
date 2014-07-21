# The Wave [![Build Status](https://drone.io/github.com/sbward/the-wave/status.png)](https://drone.io/github.com/sbward/the-wave/latest) [![Coverage Status](https://coveralls.io/repos/sbward/the-wave/badge.png?branch=master)](https://coveralls.io/r/sbward/the-wave?branch=master)

Package wave implements a concurrent worker pool that can be used to easily communicate with a large amount of remote hosts.

[GoDoc API Documentation](http://godoc.org/gopkg.in/sbward/the-wave.v1)

Do this: | With this:
--- | ---
<img src="http://angel.elte.hu/wave/fig/demo/simulationMovieLarge.gif" width="350px" label="http://angel.elte.hu/wave/index.cgi?m=models"> | <img src="http://cdn.slashgear.com/wp-content/uploads/2012/10/google-datacenter-tech-13-580x386.jpg" width="350px" label="http://www.slashgear.com/google-data-center-hd-photos-hit-where-the-internet-lives-gallery-17252451/">

## Usage
### Single Wave
```go
package main

import "gopkg.in/sbward/the-wave.v1"

func main() {
	hosts := []string{
		"server-1.internal",
		"server-2.internal",
		// ... Lots of hostnames ...
		"server-87453.internal",
	}

	h := wave.Once(10, hosts, func(host string) {
		// Gather metrics from host.
	})

	h.Finish() // Starts the wave and blocks until it finishes.
}
```
### Continuous Wave
```go
package main

import (
	"gopkg.in/sbward/the-wave.v1"
	"log"
)

func main() {
	hosts := []string{
		"server-1.internal",
		"server-2.internal",
		// ... Lots of hostnames ...
		"server-87453.internal",
	}

	h := wave.Continuous(10, hosts, func(host string) {
		// Gather metrics from host.
	})

	h.AfterEach(func() {
		// Pause for 15 seconds between waves.
		time.Sleep(time.Duration(15) * time.Second)
	})

	h.OnStop(func() {
		log.Println("Monitoring has stopped")
	})

	h.Start()

	// Run continuously for 5 mins.
	time.Sleep(time.Duration(5) * time.Minute)

	h.Finish()
}
```
