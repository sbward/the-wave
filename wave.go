package wave

import (
	"flag"
	"log"
	"runtime"
	"strconv"
)

func init() {
	cpus := runtime.NumCPU()
	procs := *flag.Int("wave.procs", cpus, "Number of processors to use")
	if procs < 1 || procs > cpus {
		panic("Value for -wave.procs must be between 1 and " + strconv.Itoa(cpus))
	}
	runtime.GOMAXPROCS(procs)
	log.Println("GOMAXPROCS = " + strconv.Itoa(procs))
}

type Wave interface {
	SetConcurrency(int) error
	SetWaitInterval(int) error
	SetRepeat(bool)

	// Start or resume the wave.
	// Returns an error if already running or if the configuration is invalid.
	Start() error

	// Pause the wave, allowing active sessions to finish.
	// Returns an error if already paused.
	Pause() error
}

// A plugin is a set of event hooks which are invoked during a wave.
// Each hook must be implemented, but they can be empty if you want to skip one.
// Plugins are used concurrently by multiple goroutines and must be threadsafe.
type BasePlugin interface {
	WaveInit(Wave)  // Called immediately before a full wave is started.
	WavePause(Wave) // Signals the wave is going into a paused state.
	WaveStart(Wave) // Signals the wave is starting or resuming from a paused state.
	WaveEnd(Wave)   // Called immediately after a full wave is completed.
}
