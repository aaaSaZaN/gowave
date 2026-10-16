package nodessh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"
	"gowave/internal/database"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type control struct {
	Type      string   `json:"t"`
	ID        uint64   `json:"id"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Username  string   `json:"username"`
	Cols      int      `json:"cols"`
	Rows      int      `json:"rows"`
	Keys      []string `json:"keys"`
	Signature string   `json:"signature"`
	Accept    bool     `json:"accept"`
	Message   string   `json:"message"`
}
type terminal struct {
	ctx     context.Context
	ws      *websocket.Conn
	pending sync.Map
	next    atomic.Uint64
	mu      sync.Mutex
	session *ssh.Session
	stdin   io.WriteCloser
	opened  bool
	last    atomic.Int64
}

func (t *terminal) send(value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(t.ctx, 15*time.Second)
	defer cancel()
	return t.ws.Write(ctx, websocket.MessageText, data)
}
func (t *terminal) ask(kind string, fields map[string]interface{}) (control, error) {
	id := t.next.Add(1)
	fields["t"], fields["id"] = kind, id
	reply := make(chan control, 1)
	t.pending.Store(id, reply)
	defer t.pending.Delete(id)
	if err := t.send(fields); err != nil {
		return control{}, err
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case <-t.ctx.Done():
		return control{}, t.ctx.Err()
	case <-timer.C:
		return control{}, errors.New("browser agent timed out")
	case response := <-reply:
		if response.Type == "error" {
			return control{}, errors.New("browser agent rejected request")
		}
		return response, nil
	}
}

type browserSigner struct {
	terminal *terminal
	key      ssh.PublicKey
}

func (s *browserSigner) PublicKey() ssh.PublicKey { return s.key }
func (s *browserSigner) Sign(random io.Reader, data []byte) (*ssh.Signature, error) {
	return s.SignWithAlgorithm(random, data, s.key.Type())
}
func (s *browserSigner) Algorithms() []string {
	if s.key.Type() == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	}
	return []string{s.key.Type()}
}
func (s *browserSigner) SignWithAlgorithm(_ io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	var hash interface{}
	switch algorithm {
	case ssh.KeyAlgoRSASHA256:
		hash = "sha256"
	case ssh.KeyAlgoRSASHA512:
		hash = "sha512"
	case ssh.KeyAlgoED25519:
	default:
		return nil, errors.New("unsupported SSH signature algorithm")
	}
	response, err := s.terminal.ask("agent-sign", map[string]interface{}{"publicKey": base64.StdEncoding.EncodeToString(s.key.Marshal()), "data": base64.StdEncoding.EncodeToString(data), "hash": hash})
	if err != nil {
		return nil, err
	}
	if response.Type != "sign" || len(response.Signature) > 2048 {
		return nil, errors.New("invalid signature response")
	}
	signature, err := base64.StdEncoding.DecodeString(response.Signature)
	if err != nil {
		return nil, err
	}
	result := &ssh.Signature{Format: algorithm, Blob: signature}
	if err := s.key.Verify(data, result); err != nil {
		return nil, err
	}
	return result, nil
}
func (h *Handler) terminal(w http.ResponseWriter, r *http.Request, node *database.Node) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"rw"}})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	t := &terminal{ctx: ctx, ws: ws}
	t.last.Store(time.Now().UnixNano())
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.session != nil {
			t.session.Close()
		}
	}()
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(time.Unix(0, t.last.Load())) > 30*time.Minute {
					cancel()
					ws.CloseNow()
					return
				}
				pingCtx, done := context.WithTimeout(ctx, 10*time.Second)
				err := ws.Ping(pingCtx)
				done()
				if err != nil {
					cancel()
					ws.CloseNow()
					return
				}
			}
		}
	}()
	for {
		kind, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		t.last.Store(time.Now().UnixNano())
		if kind == websocket.MessageBinary {
			t.mu.Lock()
			input := t.stdin
			t.mu.Unlock()
			if input != nil {
				if _, err := input.Write(data); err != nil {
					return
				}
			}
			continue
		}
		var message control
		if json.Unmarshal(data, &message) != nil {
			t.send(map[string]interface{}{"t": "error", "message": "Malformed control message"})
			return
		}
		switch message.Type {
		case "open":
			if t.opened || message.Port < 1 || message.Port > 65535 || len(message.Username) < 1 || len(message.Username) > 64 || !validSize(message) {
				t.send(map[string]interface{}{"t": "error", "message": "Invalid SSH open request"})
				return
			}
			allowed := message.Host == node.Address
			var ips []struct {
				IP string `json:"ip"`
			}
			json.Unmarshal([]byte(node.IPs), &ips)
			for _, ip := range ips {
				allowed = allowed || message.Host == ip.IP
			}
			if !allowed {
				t.send(map[string]interface{}{"t": "error", "message": "Host does not belong to this node"})
				return
			}
			t.opened = true
			go func() {
				if err := t.connect(message); err != nil {
					t.send(map[string]interface{}{"t": "error", "message": "SSH connection failed"})
				}
				cancel()
				ws.CloseNow()
			}()
		case "resize":
			if !validSize(message) {
				return
			}
			t.mu.Lock()
			session := t.session
			t.mu.Unlock()
			if session != nil {
				session.WindowChange(message.Rows, message.Cols)
			}
		case "identities", "sign", "hostkey", "error":
			if pending, ok := t.pending.Load(message.ID); ok {
				select {
				case pending.(chan control) <- message:
				default:
				}
			}
		default:
			t.send(map[string]interface{}{"t": "error", "message": "Unknown control message"})
			return
		}
	}
}
func validSize(message control) bool {
	return message.Cols >= 1 && message.Cols <= 1000 && message.Rows >= 1 && message.Rows <= 1000
}
func (t *terminal) connect(open control) error {
	config := &ssh.ClientConfig{User: open.Username, Timeout: 30 * time.Second, Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		response, err := t.ask("agent-identities", map[string]interface{}{})
		if err != nil {
			return nil, err
		}
		if response.Type != "identities" || len(response.Keys) > 16 {
			return nil, errors.New("invalid identities")
		}
		var signers []ssh.Signer
		for _, line := range response.Keys {
			if len(line) > 4096 {
				return nil, errors.New("identity too large")
			}
			key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
			if err != nil {
				return nil, err
			}
			if key.Type() != ssh.KeyAlgoRSA && key.Type() != ssh.KeyAlgoED25519 {
				continue
			}
			signers = append(signers, &browserSigner{t, key})
		}
		return signers, nil
	})}, HostKeyCallback: func(host string, remote net.Addr, key ssh.PublicKey) error {
		response, err := t.ask("hostkey", map[string]interface{}{"algo": key.Type(), "fingerprint": ssh.FingerprintSHA256(key), "key": base64.StdEncoding.EncodeToString(key.Marshal())})
		if err != nil {
			return err
		}
		if response.Type != "hostkey" || !response.Accept {
			return errors.New("host key rejected")
		}
		return nil
	}}
	address := net.JoinHostPort(open.Host, strconv.Itoa(open.Port))
	conn, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(t.ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(t.ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	clientConn, channels, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Time{})
	client := ssh.NewClient(clientConn, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	input, err := session.StdinPipe()
	if err != nil {
		return err
	}
	output := terminalWriter{t}
	session.Stdout = output
	session.Stderr = output
	if err := session.RequestPty("xterm-256color", open.Rows, open.Cols, ssh.TerminalModes{}); err != nil {
		return err
	}
	if err := session.Shell(); err != nil {
		return err
	}
	t.mu.Lock()
	t.session = session
	t.stdin = input
	t.mu.Unlock()
	if err := t.send(map[string]interface{}{"t": "ready"}); err != nil {
		return err
	}
	err = session.Wait()
	code := 0
	var signal interface{}
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitStatus()
		if exit.Signal() != "" {
			signal = exit.Signal()
		}
	}
	t.send(map[string]interface{}{"t": "exit", "code": code, "signal": signal})
	return nil
}

type terminalWriter struct{ terminal *terminal }

func (w terminalWriter) Write(data []byte) (int, error) {
	ctx, cancel := context.WithTimeout(w.terminal.ctx, 15*time.Second)
	defer cancel()
	if len(data) > 1<<20 {
		return 0, fmt.Errorf("output exceeds buffer limit")
	}
	if err := w.terminal.ws.Write(ctx, websocket.MessageBinary, data); err != nil {
		return 0, err
	}
	w.terminal.last.Store(time.Now().UnixNano())
	return len(data), nil
}
