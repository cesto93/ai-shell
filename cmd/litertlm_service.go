package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type LitertLMService struct {
	cmd  *exec.Cmd
	port string
}

func NewLitertLMService(port string) *LitertLMService {
	return &LitertLMService{port: port}
}

func (s *LitertLMService) Start() error {
	if s.IsRunning() {
		return nil
	}

	s.cmd = exec.Command("litert-lm", "serve", "--port", s.port, "--api", "openai")
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start litert-lm service: %w", err)
	}

	if err := s.waitForReady(); err != nil {
		s.Stop()
		return err
	}

	return nil
}

func (s *LitertLMService) Stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() {
			s.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			s.cmd.Process.Kill()
			s.cmd.Wait()
		}
		s.cmd = nil
	}
}

func (s *LitertLMService) IsRunning() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	err := s.cmd.Process.Signal(os.Signal(nil))
	return err == nil
}

func (s *LitertLMService) waitForReady() error {
	url := fmt.Sprintf("http://localhost:%s/v1/models", s.port)
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
	}
	return fmt.Errorf("litert-lm service failed to start on port %s within 30s", s.port)
}

func IsPortAvailable(port string) bool {
	addr := fmt.Sprintf("localhost:%s", port)
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

func extractPort(baseURL string) string {
	_, port, err := net.SplitHostPort(baseURL)
	if err != nil {
		return "9379"
	}
	return port
}
