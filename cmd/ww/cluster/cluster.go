package cluster

import (
	"log/slog"

	"github.com/libp2p/go-libp2p"
	local "github.com/libp2p/go-libp2p/core/host"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/urfave/cli/v2"

	"github.com/wetware/pkg/cap/host"
	"github.com/wetware/pkg/client"
	"github.com/wetware/pkg/system"
)

var (
	h        host.Host
	releases *[]func()
	closes   *[]func() error
)

func Command() *cli.Command {
	return &cli.Command{
		Name:    "cluster",
		Usage:   "cli client for wetware clusters",
		Aliases: []string{"client"}, // TODO(soon):  deprecate
		Subcommands: []*cli.Command{
			run(),
		},
	}
}

func setup() cli.BeforeFunc {
	return func(c *cli.Context) (err error) {
		rs := make([]func(), 0)
		releases = &rs
		cs := make([]func() error, 0)
		closes = &cs

		ch, err := clientHost(c)
		if err != nil {
			return err
		}
		*closes = append(*closes, ch.Close)

		h, err = system.Bootstrap[host.Host](c.Context, ch, client.Dialer{
			Logger:   slog.Default(),
			NS:       c.String("ns"),
			Peers:    c.StringSlice("peer"),
			Discover: c.String("discover"),
		})
		if err != nil {
			return err
		}
		*releases = append(*releases, h.Release)
		return nil
	}
}

func teardown() cli.AfterFunc {
	return func(c *cli.Context) (err error) {
		for _, close := range *closes {
			defer close()
		}
		for _, release := range *releases {
			defer release()
		}
		return nil
	}
}

func clientHost(c *cli.Context) (local.Host, error) {
	return libp2p.New(
		libp2p.NoTransports,
		libp2p.NoListenAddrs,
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(quic.NewTransport))
}
