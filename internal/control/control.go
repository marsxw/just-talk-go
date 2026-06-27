package control

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Handler receives control commands from CLI clients.
type Handler struct {
	Toggle func() error
	Start  func() error
	Stop   func() error
	Status func() (string, error)
}

// Server listens on a Unix socket for recording control commands.
type Server struct {
	path    string
	handler Handler
	ln      net.Listener
	wg      sync.WaitGroup
}

func NewServer(path string, handler Handler) *Server {
	return &Server{path: path, handler: handler}
}

func (s *Server) Start(ctx context.Context) error {
	_ = os.Remove(s.path)
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create control socket dir: %w", err)
	}
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("listen control socket %s: %w", s.path, err)
	}
	s.ln = ln
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if ctx.Err() != nil {
					return
				}
				continue
			}
			go s.handleConn(conn)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	return nil
}

func (s *Server) Stop() {
	if s.ln != nil {
		_ = s.ln.Close()
	}
	s.wg.Wait()
	_ = os.Remove(s.path)
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		return
	}
	line := strings.TrimSpace(sc.Text())
	resp := s.dispatch(line)
	_, _ = fmt.Fprintf(conn, "%s\n", resp)
}

func (s *Server) dispatch(cmd string) string {
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "toggle":
		if s.handler.Toggle == nil {
			return "error: toggle not supported"
		}
		if err := s.handler.Toggle(); err != nil {
			return "error: " + err.Error()
		}
		return "ok"
	case "start":
		if s.handler.Start == nil {
			return "error: start not supported"
		}
		if err := s.handler.Start(); err != nil {
			return "error: " + err.Error()
		}
		return "ok"
	case "stop":
		if s.handler.Stop == nil {
			return "error: stop not supported"
		}
		if err := s.handler.Stop(); err != nil {
			return "error: " + err.Error()
		}
		return "ok"
	case "status":
		if s.handler.Status == nil {
			return "error: status not supported"
		}
		status, err := s.handler.Status()
		if err != nil {
			return "error: " + err.Error()
		}
		return status
	case "ping":
		return "ok"
	default:
		return "error: unknown command"
	}
}

// Send connects to the control socket and sends one command.
func Send(path, command string) (string, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return "", fmt.Errorf("connect control socket %s: %w (is just-talk running?)", path, err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
		return "", fmt.Errorf("send command: %w", err)
	}
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		return "", fmt.Errorf("no response from just-talk")
	}
	resp := strings.TrimSpace(sc.Text())
	if strings.HasPrefix(resp, "error:") {
		return resp, fmt.Errorf("%s", strings.TrimPrefix(resp, "error:"))
	}
	return resp, nil
}
