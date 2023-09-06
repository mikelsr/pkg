package csp

import (
	"context"
	"errors"

	capnp "capnproto.org/go/capnp/v3"
	"github.com/tetratelabs/wazero/sys"

	api "github.com/wetware/pkg/api/process"
)

var (
	ErrRunning    = errors.New("running")
	ErrNotStarted = errors.New("not started")
)

type Proc api.Process

type Pinfo struct {
	Pid      uint32
	Ppid     uint32
	Cid      []byte
	Args     []string
	Creation uint64
}

func (i Pinfo) argsMsg() capnp.TextList {
	_, seg := capnp.NewSingleSegmentMessage(nil)
	tl, _ := capnp.NewTextList(seg, int32(len(i.Args)))
	for i, arg := range i.Args {
		tl.Set(i, arg)
	}
	return tl
}

func (i Pinfo) Msg() api.ProcessInfo {
	_, seg := capnp.NewSingleSegmentMessage(nil)
	pi, _ := api.NewProcessInfo(seg)
	pi.SetArgs(i.argsMsg())
	pi.SetCreation(i.Creation)
	pi.SetCid(i.Cid)
	pi.SetPid(i.Pid)
	pi.SetPpid(i.Ppid)
	return pi
}

func (i *Pinfo) FromMsg(inf api.ProcessInfo) error {

	tl, err := inf.Args()
	if err != nil {
		return err
	}
	args := make([]string, tl.Len())
	for i := 0; i < tl.Len(); i++ {
		arg, err := tl.At(i)
		if err != nil {
			return err
		}
		args[i] = arg
	}

	i.Cid, err = inf.Cid()
	if err != nil {
		return err
	}

	i.Creation = inf.Creation()
	i.Pid = inf.Pid()
	i.Ppid = inf.Ppid()

	return nil
}

func (p Proc) AddRef() Proc {
	return Proc(api.Process(p).AddRef())
}

func (p Proc) Release() {
	capnp.Client(p).Release()
}

// Kill a process and any sub processes it might have spawned.
func (p Proc) Kill(ctx context.Context) error {
	f, release := api.Process(p).Kill(ctx, nil)
	defer release()

	select {
	case <-f.Done():
	case <-ctx.Done():
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	_, err := f.Struct()
	if err != nil {
		return err
	}
	return nil
}

func (p Proc) Wait(ctx context.Context) error {
	f, release := api.Process(p).Wait(ctx, nil)
	defer release()

	select {
	case <-f.Done():
	case <-ctx.Done():
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	res, err := f.Struct()
	if err != nil {
		return err
	}

	if code := res.ExitCode(); code != 0 {
		err = sys.NewExitError(code)
	}

	return err
}
