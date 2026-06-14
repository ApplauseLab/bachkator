package factorydaemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

const providerCallTimeout = 30 * time.Second

type providerSession[Client any] struct {
	name    string
	command []string
	dir     string
	connect func(io.Reader, io.Writer) Client
	dial    func(context.Context, Client) error

	mu        sync.Mutex
	connected bool
	client    Client
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	cancel    context.CancelFunc
	stderr    *cappedBuffer
}

func newProviderSession[Client any](
	name string,
	command []string,
	dir string,
	connect func(io.Reader, io.Writer) Client,
	dial func(context.Context, Client) error,
) *providerSession[Client] {
	return &providerSession[Client]{
		name:    name,
		command: command,
		dir:     dir,
		connect: connect,
		dial:    dial,
	}
}

func (s *providerSession[Client]) get(ctx context.Context) (Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var zero Client
	if s.connected {
		return s.client, nil
	}
	sessionErr := func() error {
		command, err := resolveProviderCommand(s.command)
		if err != nil {
			return err
		}
		cmdCtx, cancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(cmdCtx, command[0], command[1:]...)
		cmd.Dir = s.dir
		cmd.Env = providerEnvironment()
		stdin, err := cmd.StdinPipe()
		if err != nil {
			cancel()
			return err
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			_ = stdin.Close()
			cancel()
			return err
		}
		stderrBuf := &cappedBuffer{limit: 64 * 1024}
		cmd.Stderr = stderrBuf
		if err := cmd.Start(); err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			cancel()
			return err
		}
		client := s.connect(stdout, stdin)
		dialCtx, dialCancel := context.WithTimeout(ctx, providerCallTimeout)
		defer dialCancel()
		if err := s.dial(dialCtx, client); err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			cancel()
			return err
		}
		s.cmd = cmd
		s.stdin = stdin
		s.cancel = cancel
		s.stderr = stderrBuf
		s.client = client
		s.connected = true
		return nil
	}()
	if sessionErr != nil {
		s.resetLocked()
		return zero, sessionErr
	}
	return s.client, nil
}

func (s *providerSession[Client]) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetLocked()
}

func (s *providerSession[Client]) resetLocked() {
	s.client = *new(Client)
	s.connected = false
}

func (s *providerSession[Client]) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	s.resetLocked()
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil {
		_ = s.cmd.Wait()
	}
	s.cmd = nil
	s.stdin = nil
	s.stderr = nil
	s.cancel = nil
}

func resolveProviderCommand(command []string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("provider command is empty")
	}
	if command[0] != "bach" {
		return command, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	resolved := append([]string{executable}, command[1:]...)
	return resolved, nil
}

// providerEnvironment builds the environment for trigger provider child
// processes. Providers run in the operator's trust domain — they are
// configured by the same Bachfile author who launched the daemon, and the
// documented provider contract (e.g. the GitHub issue trigger's token_env
// config) requires credentials from the ambient environment. Inherit the
// daemon's full environment instead of a scrubbed allow-list.
func providerEnvironment() []string {
	return os.Environ()
}

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
	mu     sync.Mutex
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	available := b.limit - b.buffer.Len()
	if available > 0 {
		if len(data) > available {
			_, _ = b.buffer.Write(data[:available])
		} else {
			_, _ = b.buffer.Write(data)
		}
	}
	return len(data), nil
}

func (b *cappedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
