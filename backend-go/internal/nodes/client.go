package nodes

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gowave/internal/database"

	"github.com/golang-jwt/jwt/v5"
)

type Client struct {
	httpClient *http.Client
	rsaPrivKey *rsa.PrivateKey
}

func NewNodeClient(caCertPEM, clientCertPEM, clientKeyPEM, jwtPrivKeyPEM, jwtPubKeyPEM []byte) (*Client, error) {
	var certPool *x509.CertPool
	if len(caCertPEM) > 0 {
		certPool = x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCertPEM)
	}

	var serverName string
	if len(caCertPEM) > 0 && len(jwtPubKeyPEM) > 0 {
		serverName = DeriveSNI(string(caCertPEM), string(jwtPubKeyPEM))
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // skip default IP hostname validation
		ServerName:         serverName,
	}

	if certPool != nil {
		tlsConfig.RootCAs = certPool
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("no certificates presented by node")
			}
			certs := make([]*x509.Certificate, len(rawCerts))
			for i, asn1Data := range rawCerts {
				cert, err := x509.ParseCertificate(asn1Data)
				if err != nil {
					return fmt.Errorf("failed to parse node certificate: %w", err)
				}
				certs[i] = cert
			}

			opts := x509.VerifyOptions{
				Roots:       certPool,
				CurrentTime: time.Now(),
			}
			for _, cert := range certs[1:] {
				if opts.Intermediates == nil {
					opts.Intermediates = x509.NewCertPool()
				}
				opts.Intermediates.AddCert(cert)
			}

			_, err := certs[0].Verify(opts)
			var unknownCA x509.UnknownAuthorityError
			if errors.As(err, &unknownCA) {
				return fmt.Errorf("%w: the service on this port is not a node of this panel; another program may hold NODE_PORT, or the node uses a SECRET_KEY from another panel", err)
			}
			return err
		}
	}

	if len(clientCertPEM) > 0 && len(clientKeyPEM) > 0 {
		cert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	transport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	var rsaKey *rsa.PrivateKey
	if len(jwtPrivKeyPEM) > 0 {
		block, _ := pem.Decode(jwtPrivKeyPEM)
		if block != nil {
			if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				if rk, ok := k.(*rsa.PrivateKey); ok {
					rsaKey = rk
				}
			} else if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				rsaKey = k
			}
		}
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		rsaPrivKey: rsaKey,
	}, nil
}

func (c *Client) generateToken() (string, error) {
	if c.rsaPrivKey == nil {
		return "", nil
	}
	claims := jwt.MapClaims{
		"uuid":     nil,
		"username": nil,
		"role":     "API",
		"sub":      "remnawave",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(c.rsaPrivKey)
}

func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if token, err := c.generateToken(); err == nil && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return c.httpClient.Do(req)
}

func getNodePort(node *database.Node) int {
	if node.Port != nil && *node.Port > 0 {
		return *node.Port
	}
	return 443
}

type NodeHealthCheckResult struct {
	IsAlive      bool
	StatusMsg    string
	XrayVersion  string
	NodeVersion  string
	NodeType     string
	IsCustomCore bool
}

func (c *Client) CheckHealth(node *database.Node) (NodeHealthCheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Remnanode/TS endpoint is /node/xray/healthcheck
	url := fmt.Sprintf("https://%s:%d/node/xray/healthcheck", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return NodeHealthCheckResult{IsAlive: false, StatusMsg: err.Error()}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NodeHealthCheckResult{IsAlive: false, StatusMsg: fmt.Sprintf("Node returned status %d", resp.StatusCode)}, nil
	}

	var res struct {
		Response struct {
			IsAlive                  bool    `json:"isAlive"`
			XrayInternalStatusCached bool    `json:"xrayInternalStatusCached"`
			XrayVersion              *string `json:"xrayVersion"`
			NodeVersion              string  `json:"nodeVersion"`
			NodeType                 string  `json:"nodeType"`
			IsCustomCore             bool    `json:"isCustomCore"`
		} `json:"response"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	alive := res.Response.IsAlive && res.Response.XrayInternalStatusCached
	msg := "OK"
	if !res.Response.XrayInternalStatusCached {
		alive = false
		msg = "Xray core is stopped"
	}

	xrayVer := ""
	if res.Response.XrayVersion != nil {
		xrayVer = *res.Response.XrayVersion
	}

	nodeType := strings.ToLower(res.Response.NodeType)
	if nodeType == "" {
		nodeType = "ts"
	}

	isCustom := res.Response.IsCustomCore
	if strings.Contains(strings.ToLower(xrayVer), "custom") || strings.Contains(strings.ToLower(xrayVer), "mod") {
		isCustom = true
	}

	return NodeHealthCheckResult{
		IsAlive:      alive,
		StatusMsg:    msg,
		XrayVersion:  xrayVer,
		NodeVersion:  res.Response.NodeVersion,
		NodeType:     nodeType,
		IsCustomCore: isCustom,
	}, nil
}

// CollectTorrentReports drains the agent's torrent-blocker reports
// (POST /node/plugin/torrent-blocker/collect). The agent forgets them once
// returned.
func (c *Client) CollectTorrentReports(node *database.Node) ([]json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/plugin/torrent-blocker/collect", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}
	var res struct {
		Response struct {
			Reports []json.RawMessage `json:"reports"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Response.Reports, nil
}

func (c *Client) SyncPlugin(node *database.Node, payload map[string]interface{}) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/plugin/sync", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, payload)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) StartXray(node *database.Node, payload map[string]interface{}) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/xray/start", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, payload)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return false, fmt.Errorf("node returned status %d: %s", resp.StatusCode, string(body))
		}
		return false, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response struct {
			IsStarted bool    `json:"isStarted"`
			Error     *string `json:"error"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return true, nil
	}

	if !res.Response.IsStarted {
		msg := "failed to start xray"
		if res.Response.Error != nil && *res.Response.Error != "" {
			msg = *res.Response.Error
		}
		return false, errors.New(msg)
	}

	return true, nil
}

func (c *Client) StopXray(node *database.Node) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/xray/stop", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

type NodeUserIPInfo struct {
	IP       string      `json:"ip"`
	LastSeen interface{} `json:"lastSeen"`
}

type NodeUserSession struct {
	UserID int              `json:"userId"`
	IPs    []NodeUserIPInfo `json:"ips"`
}

func (c *Client) GetUsersIpList(node *database.Node) ([]NodeUserSession, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/stats/get-users-ip-list", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response struct {
			Users []struct {
				UserID interface{}      `json:"userId"`
				IPs    []NodeUserIPInfo `json:"ips"`
			} `json:"users"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var users []NodeUserSession
	for _, u := range res.Response.Users {
		var uid int
		switch v := u.UserID.(type) {
		case float64:
			uid = int(v)
		case string:
			uid, _ = strconv.Atoi(v)
		}
		if uid > 0 {
			users = append(users, NodeUserSession{
				UserID: uid,
				IPs:    u.IPs,
			})
		}
	}
	return users, nil
}

func (c *Client) GetUserIpList(node *database.Node, userID string) ([]NodeUserIPInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/stats/get-user-ip-list", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, map[string]string{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response struct {
			IPs []interface{} `json:"ips"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	var list []NodeUserIPInfo
	for _, item := range res.Response.IPs {
		switch v := item.(type) {
		case string:
			list = append(list, NodeUserIPInfo{IP: v, LastSeen: time.Now()})
		case map[string]interface{}:
			ip, _ := v["ip"].(string)
			lastSeen := v["lastSeen"]
			list = append(list, NodeUserIPInfo{IP: ip, LastSeen: lastSeen})
		}
	}
	return list, nil
}

func (c *Client) GetLogs(node *database.Node, logType string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 500
	}
	if logType == "" {
		logType = "xray"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/logs?type=%s&lines=%d", node.Address, getNodePort(node), logType, lines)
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response struct {
			Logs []string `json:"logs"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Response.Logs, nil
}

func (c *Client) CheckUpdates(node *database.Node, repo string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/updates/check", node.Address, getNodePort(node))
	if repo != "" {
		url += "?repo=" + repo
	}
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response map[string]interface{}
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Response, nil
}

func (c *Client) ApplyUpdate(node *database.Node, payload map[string]interface{}) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/updates/apply", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response map[string]interface{}
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Response, nil
}

type NodeUserTraffic struct {
	Username string `json:"username"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
}

type NodeSystemStatsResponse struct {
	Plugins *struct {
		TorrentBlocker *struct {
			ReportsCount int `json:"reportsCount"`
		} `json:"torrentBlocker"`
	} `json:"plugins,omitempty"`
	XrayInfo *struct {
		Uptime       int64 `json:"uptime"`
		NumGoroutine int   `json:"numGoroutine"`
		Alloc        int64 `json:"alloc"`
	} `json:"xrayInfo"`
	System *struct {
		Info *struct {
			Arch              string   `json:"arch"`
			CPUs              int      `json:"cpus"`
			CPUModel          string   `json:"cpuModel"`
			MemoryTotal       uint64   `json:"memoryTotal"`
			Hostname          string   `json:"hostname"`
			Platform          string   `json:"platform"`
			Release           string   `json:"release"`
			Type              string   `json:"type"`
			Version           string   `json:"version"`
			NetworkInterfaces []string `json:"networkInterfaces"`
		} `json:"info"`
		Stats *struct {
			MemoryFree uint64    `json:"memoryFree"`
			MemoryUsed uint64    `json:"memoryUsed"`
			Uptime     float64   `json:"uptime"`
			LoadAvg    []float64 `json:"loadAvg"`
			Interface  *struct {
				Interface     string `json:"interface"`
				RxBytesPerSec uint64 `json:"rxBytesPerSec"`
				TxBytesPerSec uint64 `json:"txBytesPerSec"`
				RxTotal       uint64 `json:"rxTotal"`
				TxTotal       uint64 `json:"txTotal"`
			} `json:"interface"`
		} `json:"stats"`
	} `json:"system"`
}

func (c *Client) GetSystemStats(node *database.Node) (*NodeSystemStatsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/stats/get-system-stats", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response NodeSystemStatsResponse `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res.Response, nil
}

func (c *Client) GetUsersStats(node *database.Node, reset bool) ([]NodeUserTraffic, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/stats/get-users-stats", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, map[string]bool{"reset": reset})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response struct {
			Users []NodeUserTraffic `json:"users"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Response.Users, nil
}

type NodeTrafficEntry struct {
	Inbound  string `json:"inbound,omitempty"`
	Outbound string `json:"outbound,omitempty"`
	Downlink int64  `json:"downlink"`
	Uplink   int64  `json:"uplink"`
}

type NodeCombinedStatsResponse struct {
	Inbounds  []NodeTrafficEntry `json:"inbounds"`
	Outbounds []NodeTrafficEntry `json:"outbounds"`
}

func (c *Client) GetCombinedStats(node *database.Node, reset bool) (*NodeCombinedStatsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/stats/get-combined-stats", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, map[string]bool{"reset": reset})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	var res struct {
		Response NodeCombinedStatsResponse `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res.Response, nil
}

type NodeUserInboundData struct {
	Type       string  `json:"type"`
	Tag        string  `json:"tag"`
	Username   string  `json:"username"`
	UUID       string  `json:"uuid,omitempty"`
	Password   string  `json:"password,omitempty"`
	Flow       string  `json:"flow,omitempty"`
	CipherType *string `json:"cipherType,omitempty"`
	IVCheck    bool    `json:"ivCheck,omitempty"`
}

type NodeUserHashData struct {
	VlessUUID     string  `json:"vlessUuid"`
	PrevVlessUUID *string `json:"prevVlessUuid,omitempty"`
}

type AddUserRequestPayload struct {
	HashData NodeUserHashData         `json:"hashData"`
	Data     []map[string]interface{} `json:"data"`
}

type RemoveUserRequestPayload struct {
	Username string           `json:"username"`
	HashData NodeUserHashData `json:"hashData"`
}

func (c *Client) AddUser(node *database.Node, payload AddUserRequestPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/handler/add-user", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("node returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Response *struct {
			Success *bool   `json:"success"`
			Error   *string `json:"error"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Response != nil && result.Response.Success != nil && !*result.Response.Success {
		if result.Response.Error != nil && *result.Response.Error != "" {
			return fmt.Errorf("node rejected user sync: %s", *result.Response.Error)
		}
		return fmt.Errorf("node rejected user sync")
	}
	return nil
}

func (c *Client) RemoveUser(node *database.Node, payload RemoveUserRequestPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://%s:%d/node/handler/remove-user", node.Address, getNodePort(node))
	resp, err := c.doRequest(ctx, "POST", url, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("node returned status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
