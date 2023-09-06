package csp_server

import (
	"context"

	"github.com/tetratelabs/wazero/sys"
	api "github.com/wetware/pkg/api/process"
	"github.com/wetware/pkg/cap/csp"
)

// process is the main implementation of the Process capability.
type process struct {
	*csp.Pinfo
	done     <-chan execResult
	killFunc func(uint32) // killFunc must call cancel()
	cancel   context.CancelFunc
	result   execResult
}

func (p *process) Kill(ctx context.Context, call api.Process_kill) error {
	p.killFunc(p.Pid)
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
	return res.SetInfo(p.Msg())
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
