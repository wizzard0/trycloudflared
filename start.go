package trycloudflared

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"runtime"
	"time"

	"github.com/cloudflare/cloudflared/client"
	"github.com/cloudflare/cloudflared/config"
	"github.com/cloudflare/cloudflared/connection"
	"github.com/cloudflare/cloudflared/edgediscovery"
	"github.com/cloudflare/cloudflared/edgediscovery/allregions"
	"github.com/cloudflare/cloudflared/features"
	"github.com/cloudflare/cloudflared/ingress"
	"github.com/cloudflare/cloudflared/logger"
	"github.com/cloudflare/cloudflared/orchestration"
	"github.com/cloudflare/cloudflared/signal"
	"github.com/cloudflare/cloudflared/supervisor"
	"github.com/cloudflare/cloudflared/tlsconfig"
	"github.com/cloudflare/cloudflared/tunnelrpc/pogs"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

func CreateCloudflareTunnel(ctx context.Context, port int) (string, error) {
	// todo make this configurable
	logTransport := logger.Create(logger.CreateConfig(
		"",
		false,
		false,
		"",
		"",
	))

	observer := connection.NewObserver(logTransport, logTransport)

	featureSelector, err := features.NewFeatureSelector(ctx, "", nil, false, logTransport)
	if err != nil {
		return "", errors.Wrap(err, "can't create feature selector")
	}

	clientConfig, err := client.NewConfig(Version, runtime.GOOS+"_"+runtime.GOARCH, featureSelector)
	if err != nil {
		return "", errors.Wrap(err, "can't create client config")
	}

	ing, err := ingress.ParseIngress(&config.Configuration{
		Ingress: []config.UnvalidatedIngressRule{
			{
				Service: fmt.Sprintf("http://localhost:%d", port),
			},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "can't parse ingress")
	}

	orchestrator, err := orchestration.NewOrchestrator(
		ctx,
		&orchestration.Config{
			Ingress:            &ing,
			WarpRouting:        ingress.NewWarpRoutingConfig(&config.WarpRoutingConfig{}),
			ConfigurationFlags: map[string]string{},
		},
		[]pogs.Tag{},
		[]ingress.Rule{},
		logTransport,
	)
	if err != nil {
		return "", errors.Wrap(err, "can't create orchestrator")
	}

	connectedSignal := signal.New(make(chan struct{}))
	reconnectCh := make(chan supervisor.ReconnectSignal, 4) // 4 is default

	protocolSelector, err := connection.NewProtocolSelector(
		connection.HTTP2.String(),
		"random value", // credentials account tag
		false,
		false,
		edgediscovery.ProtocolPercentage,
		connection.ResolveTTL,
		logTransport,
	)
	if err != nil {
		return "", errors.Wrap(err, "unable to create protocol selector")
	}

	edgeTLSConfigs := make(map[connection.Protocol]*tls.Config, len(connection.ProtocolList))
	for _, p := range connection.ProtocolList {
		tlsSettings := p.TLSSettings()
		if tlsSettings == nil {
			return "", fmt.Errorf("%s has unknown TLS settings", p)
		}
		edgeTLSConfig, err := tlsconfig.CreateTunnelConfig(cli.NewContext(cli.NewApp(), &flag.FlagSet{}, nil), tlsSettings.ServerName)
		if err != nil {
			return "", errors.Wrap(err, "unable to create TLS config to connect with edge")
		}
		if len(tlsSettings.NextProtos) > 0 {
			edgeTLSConfig.NextProtos = tlsSettings.NextProtos
		}
		edgeTLSConfigs[p] = edgeTLSConfig
	}
	tunnel, err := createTunnel()
	if err != nil {
		return "", err
	}

	tunnelConfig := &supervisor.TunnelConfig{
		ClientConfig:                        clientConfig,
		GracePeriod:                         30,    // grace-period, default is 30
		EdgeAddrs:                           []string{},
		Region:                              "",
		EdgeIPVersion:                       allregions.Auto, // Default is ipv4
		EdgeBindAddr:                        nil,             // default is to let cf handle it
		HAConnections:                       2,               // 4 is default
		IsAutoupdated:                       false,
		LBPool:                              "",
		Tags:                                []pogs.Tag{},
		Log:                                 logTransport,
		LogTransport:                        logTransport,
		Observer:                            observer,
		ReportedVersion:                     "embedded-go-test",
		Retries:                             5,    // retries, default is 5
		RunFromTerminal:                     true, // todo false
		NamedTunnel:                         tunnel,
		ProtocolSelector:                    protocolSelector,
		EdgeTLSConfigs:                      edgeTLSConfigs,
		MaxEdgeAddrRetries:                  8,               // max-edge-addr-retries, default is 8
		RPCTimeout:                          5 * time.Second, // rpc-timeout, default is 5s
		WriteStreamTimeout:                  time.Second * 0,
		DisableQUICPathMTUDiscovery:         false,
		QUICConnectionLevelFlowControlLimit: 30 * (1 << 20), // default is 30MB
		QUICStreamLevelFlowControlLimit:     6 * (1 << 20),  // default is 6MB
		ICMPRouterServer:                    nil,
	}

	shutdown := make(chan struct{}) // eat this

	go func() {
		// todo might do errgroup here
		startErr := supervisor.StartTunnelDaemon(ctx, tunnelConfig, orchestrator, connectedSignal, reconnectCh, shutdown)
		if startErr != nil {
			// todo expose more graceful error reporter
			panic(errors.Wrap(startErr, "failed to start tunnel daemon"))
		}
	}()
	return "https://" + tunnel.QuickTunnelUrl, nil
}

// logAdapter wraps zerolog.Logger to satisfy the *zerolog.Logger interface requirement
func zerologPtr(l zerolog.Logger) *zerolog.Logger {
	return &l
}
