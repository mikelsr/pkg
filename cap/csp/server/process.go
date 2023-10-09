package csp_server

import (
	"context"
	"fmt"
	"sync"

	"capnproto.org/go/capnp/v3"
	"github.com/tetratelabs/wazero/sys"
	api "github.com/wetware/pkg/api/process"
	"github.com/wetware/pkg/cap/csp"
)

type killFunc func(uint32)

// process is the main implementation of the Process capability.
type process struct {
	csp.Args
	time     int64
	done     <-chan execResult
	killFunc // killFunc must call cancel()
	cancel   context.CancelFunc
	result   execResult

	linked  *sync.Map
	getProc func(uint32) (*process, bool)
}

func (p *process) Kill(ctx context.Context, call api.Process_kill) error {
	return p.kill()
}

func (p *process) kill() error {
	defer p.killLinked()
	p.killFunc(p.Pid)
	return nil
}

func (p *process) killLinked() error {
	p.linked.Range(func(key, value any) bool {
		if value != nil {
			value.(*process).kill()
		}
		return true
	})
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

func (p *process) Link(ctx context.Context, call api.Process_link) error {
	return nil
}

func (p *process) link(pid uint32) error {
	other, ok := p.getProc(pid)
	if !ok {
		return fmt.Errorf("process %d not found", pid)
	}
	p.linked.Store(other.Pid, other)
	return nil
}

func (p *process) Unlink(ctx context.Context, call api.Process_unlink) error {
	return nil
}

func (p *process) unlink(pid uint32) {
	p.linked.Delete(pid)
}

// Create an info struct from the process meta.
func (p *process) info() (api.Info, error) {
	_, seg := capnp.NewSingleSegmentMessage(nil)
	info, err := api.NewInfo(seg)
	if err != nil {
		return api.Info{}, err
	}
	info.SetPid(p.Pid)
	info.SetPpid(p.Ppid)
	info.SetTime(p.time)
	if err = info.SetCid(p.Cid.Bytes()); err != nil {
		return api.Info{}, err
	}
	_, seg = capnp.NewSingleSegmentMessage(nil)
	argv, err := capnp.NewTextList(seg, int32(len(p.Cmd)))
	if err != nil {
		return api.Info{}, err
	}
	for i, v := range p.Cmd {
		if err = argv.Set(i, v); err != nil {
			return api.Info{}, err
		}
	}
	return info, info.SetArgv(argv)
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
