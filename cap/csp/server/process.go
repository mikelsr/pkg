package csp_server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"capnproto.org/go/capnp/v3"
	"github.com/tetratelabs/wazero/sys"
	api "github.com/wetware/pkg/api/process"
	"github.com/wetware/pkg/cap/csp"
)

var nilEvents = api.Events{}

type killFunc func(uint32)
type procFetch func(uint32) (*process, bool)

// process is the main implementation of the Process capability.
type process struct {
	csp.Args
	time int64

	done     <-chan execResult
	killFunc // killFunc must call cancel()
	cancel   context.CancelFunc
	result   execResult

	links      *sync.Map
	localLinks *sync.Map
	monitors   chan api.Process_monitor
	procFetch
	events api.Events
}

func (p *process) Kill(ctx context.Context, call api.Process_kill) error {
	return p.kill(ctx)
}

func (p *process) kill(ctx context.Context) error {
	defer p.killLocalLinks(ctx)
	defer p.killLinks(ctx)
	p.killFunc(p.Pid)
	return nil
}

func (p *process) killLinks(ctx context.Context) error {
	p.localLinks.Range(func(key, value any) bool {
		if value != nil {
			value.(api.Process).Kill(ctx, nil)
		}
		return true
	})
	return nil
}

func (p *process) killLocalLinks(ctx context.Context) error {
	p.localLinks.Range(func(key, value any) bool {
		if value != nil {
			value.(*process).kill(ctx)
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

func (p *process) LinkLocal(ctx context.Context, call api.Process_linkLocal) error {
	return nil
}

func (p *process) linkLocal(pid uint32) error {
	other, ok := p.procFetch(pid)
	if !ok {
		return fmt.Errorf("process %d not found", pid)
	}
	p.localLinks.Store(other.Pid, other)
	other.localLinks.Store(p.Pid, p)
	return nil
}

func (p *process) Unlink(ctx context.Context, call api.Process_unlink) error {
	return nil
}

func (p *process) UnlinkLocal(ctx context.Context, call api.Process_unlinkLocal) error {
	return nil
}

func (p *process) unlink(ctx context.Context, op api.Process) error {
	return nil
}

func (p *process) unlinkLocal(pid uint32) error {
	other, ok := p.procFetch(pid)
	if !ok {
		return fmt.Errorf("process %d not found", pid)
	}
	other.localLinks.Delete(p.Pid)
	p.localLinks.Delete(pid)
	return nil
}

func (p *process) Monitor(ctx context.Context, call api.Process_monitor) error {
	call.Go()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.monitors <- call:
		res, err := call.AllocResults()
		if err != nil {
			return err
		}
		return res.SetEvent("process ended")
	}
}

func (p *process) releaseMonitors(ctx context.Context) {
	for len(p.monitors) > 0 {
		select {
		case <-ctx.Done():
			return
		case <-p.monitors:
		}
	}
}

func (p *process) Pause(ctx context.Context, call api.Process_pause) error {
	if p.events == nilEvents {
		return errors.New("event handler not initialized")
	}

	p.events.Pause(ctx, nil)

	return nil
}

func (p *process) Resume(ctx context.Context, call api.Process_resume) error {
	if p.events == nilEvents {
		return errors.New("event handler not initialized")
	}

	p.events.Resume(ctx, nil)

	return nil
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
