package statemachine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKubeletWatcher_SocketCreation(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-kubelet.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create watcher before socket exists
	watcher, err := NewKubeletWatcher(ctx, socketPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Create socket file after a delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		f, err := os.Create(socketPath)
		if err != nil {
			t.Errorf("Failed to create socket: %v", err)
			return
		}
		f.Close()
	}()

	// Wait for creation event
	select {
	case event := <-watcher.Events():
		if event != EventSocketCreated {
			t.Errorf("Expected EventSocketCreated, got %v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for socket creation event")
	}
}

func TestKubeletWatcher_SocketDeletion(t *testing.T) {
	// Create temporary directory and socket
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-kubelet.sock")

	// Create socket file first
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("Failed to create socket: %v", err)
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create watcher with existing socket
	watcher, err := NewKubeletWatcher(ctx, socketPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Delete socket after a delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := os.Remove(socketPath); err != nil {
			t.Errorf("Failed to remove socket: %v", err)
		}
	}()

	// Wait for deletion event
	select {
	case event := <-watcher.Events():
		if event != EventSocketDeleted {
			t.Errorf("Expected EventSocketDeleted, got %v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for socket deletion event")
	}
}

func TestKubeletWatcher_ContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-kubelet.sock")

	ctx, cancel := context.WithCancel(context.Background())

	watcher, err := NewKubeletWatcher(ctx, socketPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Cancel context
	cancel()

	// Wait for watcher to stop (channels should close)
	select {
	case _, ok := <-watcher.Events():
		if ok {
			t.Error("Expected event channel to be closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for watcher to stop")
	}
}

func TestKubeletWatcher_ExistingSocket(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "existing-socket.sock")

	// Create socket before watcher
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("Failed to create socket: %v", err)
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	watcher, err := NewKubeletWatcher(ctx, socketPath)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Should not error when socket already exists
	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher with existing socket: %v", err)
	}

	// Delete to trigger event
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Remove(socketPath)
	}()

	// Should receive deletion event
	select {
	case event := <-watcher.Events():
		if event != EventSocketDeleted {
			t.Errorf("Expected EventSocketDeleted, got %v", event)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for deletion event")
	}
}
