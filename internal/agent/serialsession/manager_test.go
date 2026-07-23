package serialsession

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	hubtermproto "github.com/coolleng2525/hubterm/internal/proto"
)

type fakePort struct {
	mu       sync.Mutex
	writes   bytes.Buffer
	reads    chan []byte
	done     chan struct{}
	closeOne sync.Once
}

type idleEOFPort struct {
	*fakePort
	firstRead sync.Once
	idleRead  chan struct{}
}

func newIdleEOFPort() *idleEOFPort {
	return &idleEOFPort{fakePort: newFakePort(), idleRead: make(chan struct{})}
}

func (p *idleEOFPort) Read(dst []byte) (int, error) {
	idle := false
	p.firstRead.Do(func() {
		idle = true
		close(p.idleRead)
	})
	if idle {
		return 0, io.EOF
	}
	return p.fakePort.Read(dst)
}

func newFakePort() *fakePort {
	return &fakePort{reads: make(chan []byte, 4), done: make(chan struct{})}
}

func (p *fakePort) Read(dst []byte) (int, error) {
	select {
	case data := <-p.reads:
		return copy(dst, data), nil
	case <-p.done:
		return 0, io.EOF
	}
}

func (p *fakePort) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
		return 0, io.ErrClosedPipe
	default:
	}
	return p.writes.Write(data)
}

func (p *fakePort) Close() error {
	p.closeOne.Do(func() { close(p.done) })
	return nil
}

func (p *fakePort) written() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writes.String()
}

func TestManagerStreamsWritesListsAndCloses(t *testing.T) {
	port := newFakePort()
	manager := newManager(func(cfg hubtermproto.SerialConfig) (io.ReadWriteCloser, error) {
		return port, nil
	})
	output := make(chan []byte, 1)
	exited := make(chan error, 1)
	cfg := hubtermproto.DefaultSerialConfig("/dev/ttyUSB0")

	if err := manager.Start("session-1", cfg, func(data []byte) { output <- data }, func(err error) { exited <- err }); err != nil {
		t.Fatal(err)
	}
	if err := manager.Write("session-1", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := port.written(); got != "hello" {
		t.Fatalf("written data = %q", got)
	}
	port.reads <- []byte{0x00, 0xff, 'A'}
	select {
	case got := <-output:
		if !bytes.Equal(got, []byte{0x00, 0xff, 'A'}) {
			t.Fatalf("output changed: %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for serial output")
	}

	sessions := manager.List()
	if len(sessions) != 1 || sessions[0].SessionID != "session-1" || sessions[0].Protocol != "serial" || sessions[0].PortName != cfg.PortName {
		t.Fatalf("unexpected sessions: %+v", sessions)
	}

	if err := manager.Close("session-1"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for close callback")
	}
	if len(manager.List()) != 0 {
		t.Fatal("closed session is still listed")
	}
}

func TestManagerKeepsSessionAfterIdleReadTimeout(t *testing.T) {
	port := newIdleEOFPort()
	manager := newManager(func(cfg hubtermproto.SerialConfig) (io.ReadWriteCloser, error) {
		return port, nil
	})
	output := make(chan []byte, 1)
	if err := manager.Start("session-idle", hubtermproto.DefaultSerialConfig("/dev/cu.usbserial-test"), func(data []byte) {
		output <- data
	}, nil); err != nil {
		t.Fatal(err)
	}
	defer manager.CloseAll()

	select {
	case <-port.idleRead:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for idle read")
	}
	if len(manager.List()) != 1 {
		t.Fatal("idle read timeout closed the serial session")
	}

	port.reads <- []byte("ready")
	select {
	case got := <-output:
		if string(got) != "ready" {
			t.Fatalf("output = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("serial data was not read after an idle timeout")
	}
}

func TestManagerRejectsDuplicateSessionAndPort(t *testing.T) {
	manager := newManager(func(cfg hubtermproto.SerialConfig) (io.ReadWriteCloser, error) { return newFakePort(), nil })
	first := hubtermproto.DefaultSerialConfig("COM3")
	if err := manager.Start("session-1", first, nil, nil); err != nil {
		t.Fatal(err)
	}
	defer manager.CloseAll()

	if err := manager.Start("session-1", hubtermproto.DefaultSerialConfig("COM4"), nil, nil); err == nil {
		t.Fatal("expected duplicate session rejection")
	}
	if err := manager.Start("session-2", first, nil, nil); err == nil {
		t.Fatal("expected duplicate port rejection")
	}
}

func TestManagerOpenFailureDoesNotReservePort(t *testing.T) {
	wantErr := errors.New("permission denied")
	calls := 0
	manager := newManager(func(cfg hubtermproto.SerialConfig) (io.ReadWriteCloser, error) {
		calls++
		if calls == 1 {
			return nil, wantErr
		}
		return newFakePort(), nil
	})
	cfg := hubtermproto.DefaultSerialConfig("COM3")
	if err := manager.Start("session-1", cfg, nil, nil); !errors.Is(err, wantErr) {
		t.Fatalf("Start error = %v", err)
	}
	if err := manager.Start("session-2", cfg, nil, nil); err != nil {
		t.Fatalf("port remained reserved after failure: %v", err)
	}
	manager.CloseAll()
}

func TestManagerCloseAll(t *testing.T) {
	manager := newManager(func(cfg hubtermproto.SerialConfig) (io.ReadWriteCloser, error) { return newFakePort(), nil })
	if err := manager.Start("session-1", hubtermproto.DefaultSerialConfig("COM3"), nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start("session-2", hubtermproto.DefaultSerialConfig("COM4"), nil, nil); err != nil {
		t.Fatal(err)
	}
	manager.CloseAll()
	if len(manager.List()) != 0 {
		t.Fatalf("sessions remain after CloseAll: %+v", manager.List())
	}
}
