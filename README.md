The Wave
========
Poll a cluster of N servers in waves of M &lt; N size.  Uses a plugin system.

Do this: | With this:
--- | ---
<img src="http://angel.elte.hu/wave/fig/demo/simulationMovieLarge.gif" width="350px" label="http://angel.elte.hu/wave/index.cgi?m=models"> | <img src="http://cdn.slashgear.com/wp-content/uploads/2012/10/google-datacenter-tech-13-580x386.jpg" width="350px" label="http://www.slashgear.com/google-data-center-hd-photos-hit-where-the-internet-lives-gallery-17252451/">

## Usage
```go
package main

import (
	"github.com/sbward/the-wave"
	"time"
)

func main() {
	w := wave.New(
		"server-1.internal",
		"server-2.internal",
		// ... Lots of hostnames ...
		"server-87453.internal",
	)
	w.SetConcurrency(10)
	w.SetWaitInterval(time.Duration(300) * time.Second)

	ping := wave.PingPlugin{}
	w.AddPlugin(wave.Plugin(&ping))

	done, err := w.Start()
	if err != nil {
		panic(err)
	}
	<-done

	fmt.Println(ping.Report())
}
```
