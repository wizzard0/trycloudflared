package trycloudflared

import (
	"encoding/json"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"runtime"
	"time"
)

func createTunnel(clientID uuid.UUID) (*connection.TunnelProperties, error) {
	// can be slow
	timeout := 30 * time.Second
	client := http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
		},
		Timeout: timeout,
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.trycloudflare.com/tunnel", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build tunnel request")
	}

	req.Header.Add("Content-Type", "application/json")
	// be nice and tell them where to find us
	req.Header.Add("User-Agent", "cloudflared/embedded-wizzard0-trycloudflared")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request Tunnel")
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read creation response")
	}

	var parsedResponse CreateTunnelResponse
	if err := json.Unmarshal(response, &parsedResponse); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal credentials")
	}

	tunnelID, err := uuid.Parse(parsedResponse.Result.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse Tunnel ID")
	}

	return &connection.TunnelProperties{Credentials: connection.Credentials{
		AccountTag:   parsedResponse.Result.AccountTag,
		TunnelSecret: parsedResponse.Result.Secret,
		TunnelID:     tunnelID,
	}, QuickTunnelUrl: parsedResponse.Result.Hostname, Client: pogs.ClientInfo{
		ClientID: clientID[:],
		Features: []string{},
		Version:  Version,
		Arch:     runtime.GOOS + "_" + runtime.GOARCH,
	}}, nil
}

type CreateTunnelResponse struct {
	Success bool                `json:"success"`
	Result  TunnelCredentials   `json:"result"`
	Errors  []CreateTunnelError `json:"errors"`
}

type CreateTunnelError struct {
	Code    int
	Message string
}

type TunnelCredentials struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	AccountTag string `json:"account_tag"`
	Secret     []byte `json:"secret"`
}
