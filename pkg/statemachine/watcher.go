package statemachine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// WatchEvent represents file system events we care about
type WatchEvent int

const (
	EventSocketCreated WatchEvent = iota
	EventSocketDeleted
)

// KubeletWatcher watches the kubelet socket file for changes
type KubeletWatcher struct {
	watcher    *fsnotify.Watcher
	socketPath string
	eventChan  chan WatchEvent
	errorChan  chan error
	ctx        context.Context
}

// NewKubeletWatcher creates a new watcher for the kubelet socket
func NewKubeletWatcher(ctx context.Context, socketPath string) (*KubeletWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	kw := &KubeletWatcher{
		watcher:    watcher,
		socketPath: socketPath,
		eventChan:  make(chan WatchEvent, 10),
		errorChan:  make(chan error, 10),
		ctx:        ctx,
	}

	return kw, nil
}

// Start begins watching the kubelet socket or its parent directory
func (w *KubeletWatcher) Start() error {
	// Check if socket exists
	if _, err := os.Stat(w.socketPath); err == nil {
		// Socket exists, watch it directly
		if err := w.watcher.Add(w.socketPath); err != nil {
			return fmt.Errorf("failed to watch socket: %w", err)
		}
		log.Printf("Watching kubelet socket: %s", w.socketPath)
	} else {
		// Socket doesn't exist, watch parent directory
		parentDir := filepath.Dir(w.socketPath)
		if err := w.watcher.Add(parentDir); err != nil {
			return fmt.Errorf("failed to watch parent directory: %w", err)
		}
		log.Printf("Watching parent directory for kubelet socket: %s", parentDir)
	}

	go w.eventLoop()
	return nil
}

// eventLoop processes fsnotify events and translates them to our event types
func (w *KubeletWatcher) eventLoop() {
	defer close(w.eventChan)
	defer close(w.errorChan)

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only care about events on our specific socket file
			if event.Name != w.socketPath {
				continue
			}

			// Translate fsnotify events to our enum
			if event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("Kubelet socket created: %s", event.Name)
				w.eventChan <- EventSocketCreated
				// Start watching the socket file itself
				w.watcher.Add(w.socketPath)
			} else if event.Op&fsnotify.Remove == fsnotify.Remove ||
				event.Op&fsnotify.Rename == fsnotify.Rename {
				log.Printf("Kubelet socket deleted/renamed: %s (op: %v)", event.Name, event.Op)
				w.eventChan <- EventSocketDeleted
				// Watch parent directory again
				w.watcher.Remove(w.socketPath)
				parentDir := filepath.Dir(w.socketPath)
				w.watcher.Add(parentDir)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
			w.errorChan <- err

		case <-w.ctx.Done():
			log.Println("Watcher context cancelled, stopping")
			return
		}
	}
}

// Events returns the channel for watch events
func (w *KubeletWatcher) Events() <-chan WatchEvent {
	return w.eventChan
}

// Errors returns the channel for watcher errors
func (w *KubeletWatcher) Errors() <-chan error {
	return w.errorChan
}

// Close stops the watcher and cleans up resources
func (w *KubeletWatcher) Close() error {
	return w.watcher.Close()
}
