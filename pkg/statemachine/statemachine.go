package statemachine

import (
	"context"
	"log"

	"hailo-device-plugin/pkg/plugin"
)

// State represents the current state of the device plugin
type State int

const (
	StateWaitingForKubelet State = iota
	StateInitializingServer
	StateRegistering
	StateRunning
	StateCleanup
	StateShutdown
)

// String returns the name of the state for logging
func (s State) String() string {
	switch s {
	case StateWaitingForKubelet:
		return "WAITING_FOR_KUBELET"
	case StateInitializingServer:
		return "INITIALIZING_SERVER"
	case StateRegistering:
		return "REGISTERING"
	case StateRunning:
		return "RUNNING"
	case StateCleanup:
		return "CLEANUP"
	case StateShutdown:
		return "SHUTDOWN"
	default:
		return "UNKNOWN"
	}
}

// Config holds configuration for the state machine
type Config struct {
	KubeletSocket string
	PluginSocket  string
	ResourceName  string
	CdiDir        string
}

// StateMachine manages the device plugin lifecycle through states
type StateMachine struct {
	currentState  State
	plugin        *plugin.HailoDevicePlugin
	server        *plugin.Server
	watcher       *KubeletWatcher
	config        *Config
	ctx           context.Context
	cancelFunc    context.CancelFunc
}

// New creates a new state machine
func New(ctx context.Context, config *Config) *StateMachine {
	smCtx, cancel := context.WithCancel(ctx)

	return &StateMachine{
		currentState: StateWaitingForKubelet,
		config:       config,
		ctx:          smCtx,
		cancelFunc:   cancel,
	}
}

// Run executes the state machine main loop
func (sm *StateMachine) Run(monitor interface{}) error {
	log.Println("Starting Hailo device plugin state machine")

	// Create device plugin instance
	sm.plugin = &plugin.HailoDevicePlugin{
		CdiDir:       sm.config.CdiDir,
		SocketPath:   sm.config.PluginSocket,
		ResourceName: sm.config.ResourceName,
	}

	for {
		log.Printf("Current state: %s", sm.currentState)

		select {
		case <-sm.ctx.Done():
			log.Println("Shutdown signal received")
			sm.transition(StateShutdown)
			return sm.handleShutdown()

		default:
			switch sm.currentState {
			case StateWaitingForKubelet:
				if err := sm.handleWaitingForKubelet(); err != nil {
					log.Printf("Error in WAITING_FOR_KUBELET: %v", err)
					// Don't exit, just retry
					continue
				}
				sm.transition(StateInitializingServer)

			case StateInitializingServer:
				if err := sm.handleInitializingServer(); err != nil {
					log.Printf("Failed to initialize server: %v", err)
					sm.transition(StateWaitingForKubelet)
					continue
				}
				sm.transition(StateRegistering)

			case StateRegistering:
				if err := sm.handleRegistering(); err != nil {
					log.Printf("Registration failed: %v", err)
					sm.transition(StateCleanup)
					continue
				}
				sm.transition(StateRunning)

			case StateRunning:
				event := sm.handleRunning()
				if event == EventSocketDeleted {
					log.Println("Kubelet socket deleted, transitioning to cleanup")
					sm.transition(StateCleanup)
				} else {
					// Shutdown event
					sm.transition(StateShutdown)
					return sm.handleShutdown()
				}

			case StateCleanup:
				sm.handleCleanup()
				sm.transition(StateWaitingForKubelet)

			case StateShutdown:
				return sm.handleShutdown()
			}
		}
	}
}

// transition changes the state and logs the transition
func (sm *StateMachine) transition(newState State) {
	log.Printf("State transition: %s → %s", sm.currentState, newState)
	sm.currentState = newState
}

// Shutdown initiates a graceful shutdown
func (sm *StateMachine) Shutdown() {
	log.Println("Initiating graceful shutdown")
	sm.cancelFunc()
}
