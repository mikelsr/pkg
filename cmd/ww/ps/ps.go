package ps

import (
	"fmt"
	"time"

	"log/slog"

	"capnproto.org/go/capnp/v3"
	"capnproto.org/go/capnp/v3/rpc"
	"github.com/ipfs/go-cid"
	local "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multibase"
	"github.com/urfave/cli/v2"

	"github.com/wetware/pkg/cap/csp"
	"github.com/wetware/pkg/cap/host"
	"github.com/wetware/pkg/cap/view"
	"github.com/wetware/pkg/client"
	"github.com/wetware/pkg/cluster/routing"
	"github.com/wetware/pkg/system"
	"github.com/wetware/pkg/util/proto"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "ps",
		Usage: "list processes running in the cluster",
		Action: func(c *cli.Context) error {
			h, err := client.NewHost()
			if err != nil {
				return err
			}
			defer h.Close()

			host, err := system.Bootstrap[host.Host](c.Context, h, client.Dialer{
				Logger:   slog.Default(),
				NS:       c.String("ns"),
				Peers:    c.StringSlice("peer"),
				Discover: c.String("discover"),
			})
			if err != nil {
				return err
			}
			defer host.Release()

			view, release := host.View(c.Context)
			defer release()

			it, release := view.Iter(c.Context, query(c))
			defer release()

			for r := it.Next(); r != nil; r = it.Next() {
				peer := r.Peer()
				if peer == h.ID() {
					continue
				}
				e, release, err := bsExecutor(c, h, peer)
				defer release()
				render(c, r, e, err)
			}

			return it.Err()
		},
	}
}

func query(c *cli.Context) view.Query {
	return view.NewQuery(view.All())
}

func bsExecutor(c *cli.Context, h local.Host, peer peer.ID) (csp.Executor, capnp.ReleaseFunc, error) {
	hCap, err := bootStrap(c, h, peer)
	if err != nil {
		return csp.Executor{}, hCap.Release, err
	}

	exec, execRelease := hCap.Executor(c.Context)
	release := func() {
		execRelease()
		hCap.Release()
	}

	return exec, release, nil
}

func render(c *cli.Context, r routing.Record, e csp.Executor, err error) {
	fmt.Fprintf(c.App.Writer, "Executor %s:%v\n", r.Server(), e)

	if err != nil {
		fmt.Printf("\t%s", err.Error())
		return
	}

	procs, release, err := e.Ps(c.Context)
	defer release()
	if err != nil {
		fmt.Printf("\t%s", err.Error())
		return
	}

	fmt.Fprintf(c.App.Writer, "%s\t%s\t%s\t%s\t%s\n", "PID", "PPID", "Creation", "CID", "Args")
	for _, proc := range procs {
		renderPinfo(c, proc)
	}
}

func renderPinfo(c *cli.Context, i csp.Pinfo) {
	_, cid, _ := cid.CidFromBytes(i.Cid)

	fmt.Fprintf(c.App.Writer, "%d\t%d\t%s\t%s\t%s\n",
		i.Pid,
		i.Ppid,
		time.UnixMicro(int64(i.Creation)).Format(time.UnixDate),
		cid.Encode(multibase.MustNewEncoder(multibase.Base58BTC)),
		i.Args,
	)
}

func bootStrap(c *cli.Context, h local.Host, peer peer.ID) (host.Host, error) {
	err := h.Connect(c.Context, h.Peerstore().PeerInfo(peer))
	if err != nil {
		return host.Host{}, err
	}

	protos := proto.Namespace(c.String("ns"))
	s, err := h.NewStream(c.Context, peer, protos...)
	if err != nil {
		return host.Host{}, err
	}

	conn := rpc.NewConn(client.Transport(s), &rpc.Options{
		ErrorReporter: system.ErrorReporter{
			Logger: slog.Default(),
		},
	})
	cap := conn.Bootstrap(c.Context)
	return host.Host(cap), cap.Resolve(c.Context)
}
