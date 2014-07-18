package wave

// A plugin is a set of event hooks which are invoked during a wave.
// Each hook must be implemented, but they can be empty if you want to skip one.
// Plugins are used concurrently by multiple goroutines and must be threadsafe.
type Plugin interface {
	Init(*Wave)            // Called immediately before a full wave is started.
	Start(*Wave)           // Triggered when a new wave is starting.
	End(*Wave)             // Triggered when a full wave has completed.
	Pause(*Wave)           // Triggered when a wave is pausing mid-cycle.
	Unpause(*Wave)         // Triggered when a wave is unpausing mid-cycle.
	Session(*Wave, string) // Invoked for each string in a Wave.
}
