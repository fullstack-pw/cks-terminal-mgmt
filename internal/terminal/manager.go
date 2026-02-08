package terminal

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	sshKeyPath string
	sshUser    string
	sessions   map[int]*ttydProcess
	mu         sync.Mutex
	portStart  int
	portEnd    int
	nextPort   int
}

type ttydProcess struct {
	cmd       *exec.Cmd
	port      int
	vmIP      string
	createdAt time.Time
}

func NewManager(sshKeyPath, sshUser string) *Manager {
	return &Manager{
		sshKeyPath: sshKeyPath,
		sshUser:    sshUser,
		sessions:   make(map[int]*ttydProcess),
		portStart:  10000,
		portEnd:    20000,
		nextPort:   10000,
	}
}

func (m *Manager) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	vmIP := r.URL.Query().Get("vmIP")
	if vmIP == "" {
		http.Error(w, "vmIP query parameter required", http.StatusBadRequest)
		return
	}

	if net.ParseIP(vmIP) == nil {
		http.Error(w, "invalid vmIP", http.StatusBadRequest)
		return
	}

	port, err := m.spawnTTYD(vmIP)
	if err != nil {
		log.Printf("Failed to spawn ttyd for %s: %v", vmIP, err)
		http.Error(w, "failed to create terminal session", http.StatusInternalServerError)
		return
	}

	if err := m.waitForTTYD(port); err != nil {
		log.Printf("ttyd not ready on port %d: %v", port, err)
		http.Error(w, "terminal session not ready", http.StatusServiceUnavailable)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/s/%d/", port), http.StatusFound)
}

func (m *Manager) HandleSession(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/s/"), "/", 2)
	if len(parts) == 0 {
		http.Error(w, "invalid session path", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	_, exists := m.sessions[port]
	m.mu.Unlock()

	if !exists {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))

	if isWebSocketUpgrade(r) {
		wsTarget, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d%s", port, r.URL.Path))
		m.proxyWebSocket(w, r, wsTarget)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = r.URL.Path
		req.URL.RawQuery = r.URL.RawQuery
		req.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("X-Frame-Options")
		return nil
	}

	proxy.ServeHTTP(w, r)
}

func (m *Manager) spawnTTYD(vmIP string) (int, error) {
	port := m.allocatePort()

	args := []string{
		"--writable",
		"--port", fmt.Sprintf("%d", port),
		"--once",
		"--interface", "127.0.0.1",
		"--base-path", fmt.Sprintf("/s/%d", port),
		"-t", "disableLeaveAlert=true",
		"-t", "disableResizeOverlay=true",
		"-t", "titleFixed=CKS Terminal",
		"-t", "fontSize=14",
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", m.sshKeyPath,
		fmt.Sprintf("%s@%s", m.sshUser, vmIP),
	}

	cmd := exec.Command("ttyd", args...)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start ttyd: %w", err)
	}

	m.mu.Lock()
	m.sessions[port] = &ttydProcess{
		cmd:       cmd,
		port:      port,
		vmIP:      vmIP,
		createdAt: time.Now(),
	}
	m.mu.Unlock()

	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("ttyd process on port %d exited: %v", port, err)
		} else {
			log.Printf("ttyd process on port %d exited cleanly", port)
		}
		m.mu.Lock()
		delete(m.sessions, port)
		m.mu.Unlock()
	}()

	log.Printf("Spawned ttyd on port %d for VM %s", port, vmIP)
	return port, nil
}

func (m *Manager) allocatePort() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := 0; i < m.portEnd-m.portStart; i++ {
		port := m.nextPort
		m.nextPort++
		if m.nextPort >= m.portEnd {
			m.nextPort = m.portStart
		}
		if _, exists := m.sessions[port]; !exists {
			return port
		}
	}
	return m.nextPort
}

func (m *Manager) waitForTTYD(port int) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("ttyd not ready after 5s on port %d", port)
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for port, proc := range m.sessions {
		if proc.cmd.Process != nil {
			log.Printf("Killing ttyd on port %d", port)
			proc.cmd.Process.Kill()
		}
	}
}

func (m *Manager) ActiveSessions() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func isWebSocketUpgrade(r *http.Request) bool {
	for _, v := range r.Header["Upgrade"] {
		if v == "websocket" {
			return true
		}
	}
	return false
}

func (m *Manager) proxyWebSocket(w http.ResponseWriter, r *http.Request, target *url.URL) {
	targetAddr := target.Host

	destConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		http.Error(w, "could not connect to terminal", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		destConn.Close()
		return
	}

	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		destConn.Close()
		return
	}

	proxyReq := r.Clone(r.Context())
	proxyReq.URL.Scheme = "ws"
	proxyReq.URL.Host = targetAddr
	proxyReq.URL.Path = target.Path
	proxyReq.URL.RawQuery = r.URL.RawQuery
	proxyReq.Host = targetAddr
	proxyReq.RequestURI = proxyReq.URL.RequestURI()

	if err := proxyReq.Write(destConn); err != nil {
		clientConn.Close()
		destConn.Close()
		return
	}

	if clientBuf.Reader.Buffered() > 0 {
		buffered := make([]byte, clientBuf.Reader.Buffered())
		clientBuf.Read(buffered)
		destConn.Write(buffered)
	}

	done := make(chan struct{})
	go func() {
		io.Copy(destConn, clientConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, destConn)
		done <- struct{}{}
	}()

	<-done
	clientConn.Close()
	destConn.Close()
}
