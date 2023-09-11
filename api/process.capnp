using Go = import "/go.capnp";

@0x9a51e53177277763;

$Go.package("process");
$Go.import("github.com/wetware/pkg/api/process");

using Cid = Data;

interface BytecodeCache {
    # BytecodeCache is used to store WASM byte code. May be implemented with
    # anchors or any other means.
    put @0 (bytecode :Data) -> (cid :Data);
    # Put stores the bytecode and returns the cid of the submitted bytecode.
    get @1 (cid :Cid) -> (bytecode :Data);
    # Get returns the bytecode matching a cid if there's a match, null otherwise.
    has @2 (cid :Cid) -> (has :Bool);
    # Has returns true if a bytecode identified by the cid has been previously stored.
}

interface Process {
    # Process is a points to a running WASM process.
    wait   @0 () -> (exitCode :UInt32);
    # Wait until a process finishes running.
    kill   @1 () -> ();
    # Kill the process.
    pause  @2 () -> ();
    # Pause a process.
    resume @3 () -> ();
    # Resume a paused process.
}

struct Info {
    pid  @0 :UInt32;
    ppid @1 :UInt32;
    cid  @2 :Cid;
    argv @3 :List(Text);
    time @4 :Int64;
}

interface Events {
    # Events are sent to the WASM process. It's the process' responsiblity to
    # handle events.
    pause  @0 () -> ();
    resume @1 () -> ();
    stop   @2 () -> ();
}
