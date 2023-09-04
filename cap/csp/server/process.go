package csp_server

import (
	"context"

	"capnproto.org/go/capnp/v3"
	"github.com/tetratelabs/wazero/sys"
	api "github.com/wetware/pkg/api/process"
)

// process is the main implementation of the Process capability.
type process struct {
	*info
	done     <-chan execResult
	killFunc func(uint32) // killFunc must call cancel()
	cancel   context.CancelFunc
	result   execResult
}

type info struct {
	pid      uint32
	ppid     uint32
	cid      []byte
	args     []string
	creation uint64
}

func (i info) argsMsg() capnp.TextList {
	_, seg := capnp.NewSingleSegmentMessage(nil)
	tl, _ := capnp.NewTextList(seg, int32(len(i.args)))
	for i, arg := range i.args {
		tl.Set(i, arg)
	}
	return tl
}

func (i info) msg() api.ProcessInfo {
	_, seg := capnp.NewSingleSegmentMessage(nil)
	pi, _ := api.NewProcessInfo(seg)
	pi.SetArgs(i.argsMsg())
	pi.SetCreation(i.creation)
	pi.SetCid(i.cid)
	pi.SetPid(i.pid)
	pi.SetPpid(i.ppid)
	return pi
}

func (p *process) Kill(ctx context.Context, call api.Process_kill) error {
	p.killFunc(p.pid)
	return nil
}

func (p *process) Wait(ctx context.Context, call api.Process_wait) error {
	call.Go()
	select {
	case res, ok := <-p.done:
		if ok {
			p.result = res
		}

	case <-ctx.Done():
		return ctx.Err()
	}

	res, err := call.AllocResults()
	if err == nil {
		err = p.result.Bind(res)
	}

	return err
}

func (p *process) Info(ctx context.Context, call api.Process_info) error {
	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	return res.SetInfo(p.msg())
}

type execResult struct {
	Values []uint64
	Err    error
}

func (r execResult) Bind(res api.Process_wait_Results) error {
	if r.Err != nil {
		res.SetExitCode(r.Err.(*sys.ExitError).ExitCode())
	}

	return nil
}
