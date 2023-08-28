// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"strings"

	"golang.org/x/exp/trace/internal/event"
	"golang.org/x/exp/trace/internal/event/go122"
)

// ordering emulates Go scheduler state for both validation and
// for putting events in the right order.
type ordering struct {
	gStates     map[GoID]*gState
	pStates     map[ProcID]*pState // TODO: The keys are dense, so this can be a slice.
	mStates     map[ThreadID]*mState
	activeTasks map[TaskID]struct{}
	gcSeq       uint64
	gcState     gcState
}

// gcState is a trinary variable for the current state of the GC.
//
// The third state besides "enabled" and "disabled" is "undetermined."
type gcState uint8

const (
	gcUndetermined gcState = iota
	gcNotRunning
	gcRunning
)

// String returns a human-readable string for the GC state.
func (s gcState) String() string {
	switch s {
	case gcUndetermined:
		return "Undetermined"
	case gcNotRunning:
		return "NotRunning"
	case gcRunning:
		return "Running"
	}
	return "Bad"
}

// advance checks if it's valid to proceed with ev which came from thread m.
//
// Returns the schedCtx at the point of the event, whether it's OK to advance
// with this event, and any error encountered in validation.
//
// If any error is returned, then the trace is broken and trace parsing must cease.
// If it's not valid to advance with ev, but no error was encountered, the caller
// should attempt to advance with other candidate events from other threads. If the
// caller runs out of candidates, the trace is invalid.
func (o *ordering) advance(ev *baseEvent, evt *evTable, m ThreadID, isInitialGen bool) (schedCtx, bool, error) {
	var curCtx, newCtx schedCtx
	curCtx.M = m
	newCtx.M = m

	if m == NoThread {
		curCtx.P = NoProc
		curCtx.G = NoGoroutine
		newCtx = curCtx
	} else {
		// Pull out or create the mState for this event.
		ms, ok := o.mStates[m]
		if !ok {
			ms = &mState{
				g: NoGoroutine,
				p: NoProc,
			}
			o.mStates[m] = ms
		}
		curCtx.P = ms.p
		curCtx.G = ms.g
		newCtx = curCtx
		defer func() {
			// Update the mState for this event.
			ms.p = newCtx.P
			ms.g = newCtx.G
		}()
	}

	switch typ := ev.typ; typ {
	// Handle procs.
	case go122.EvProcStatus:
		pid := ProcID(ev.args[0])
		status := go122.ProcStatus(ev.args[1])
		oldState := go122ProcStatus2ProcState[status]
		if s, ok := o.pStates[pid]; ok {
			if s.status == go122.ProcBad {
				oldState = ProcNotExist
				s.status = status
			} else if status == go122.ProcSyscallAbandoned && s.status == go122.ProcSyscall {
				// ProcSyscallAbandonded is a special case of ProcSyscall. It indicates a
				// potential loss of information, but if we're already in ProcSyscall,
				// we haven't lost the relevant information. Promote the status and advance.
				oldState = ProcRunning
				ev.args[1] = uint64(go122.ProcSyscall)
			} else if s.status != status {
				return schedCtx{}, false, fmt.Errorf("inconsistent status for proc %d: old %v vs. new %v", pid, s.status, status)
			}
			s.seq = 0 // Reset seq.
		} else if isInitialGen {
			o.pStates[pid] = &pState{id: pid, status: status}
			oldState = ProcUndetermined
		} else {
			return curCtx, false, fmt.Errorf("found proc status for new proc after the first generation: id=%v status=%v", pid, status)
		}
		ev.args[2] = uint64(oldState) // Smuggle in the old state for StateTransition.

		// Bind the proc to the new context, if it's running.
		if status == go122.ProcRunning || status == go122.ProcSyscall {
			newCtx.P = pid
		}
		// Set the current context to the state of the M current running this G. Otherwise
		// we'll emit a Running -> Running event that doesn't correspond to the right M.
		if status == go122.ProcSyscallAbandoned && oldState != ProcUndetermined {
			// N.B. This is slow but it should be fairly rare.
			found := false
			for mid, ms := range o.mStates {
				if ms.p == pid {
					curCtx.M = mid
					curCtx.P = pid
					curCtx.G = ms.g
					found = true
				}
			}
			if !found {
				return curCtx, false, fmt.Errorf("failed to find sched context for proc %d that's about to be stolen", pid)
			}
		}
		return curCtx, true, nil
	case go122.EvProcStart:
		pid := ProcID(ev.args[0])
		seq := ev.args[1]

		// Try to advance. We might fail here due to sequencing, because the P hasn't
		// had a status emitted, or because we already have a P and we're in a syscall,
		// and we haven't observed that it was stolen from us yet.
		state, ok := o.pStates[pid]
		if !ok || state.status != go122.ProcIdle || state.seq+1 != seq || curCtx.P != NoProc {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a ProcStart on a different M before the proc's state was emitted, or before we
			// got to the right point in the trace.
			//
			// Note that we also don't advance here if we have a P and we're in a syscall.
			return curCtx, false, nil
		}
		// We can advance this P. Check some invariants.
		//
		// We might have a goroutine if a goroutine is exiting a syscall.
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MustNotHave, Goroutine: event.MayHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			return curCtx, false, err
		}
		state.status = go122.ProcRunning
		state.seq = seq
		newCtx.P = pid
		return curCtx, true, nil
	case go122.EvProcStop:
		// We must be able to advance this P.
		//
		// There are 2 ways a P can stop: ProcStop and ProcSteal. ProcStop is used when the P
		// is stopped by the same M that started it, while ProcSteal is used when another M
		// steals the P by stopping it from a distance.
		//
		// Since a P is bound to an M, and we're stopping on the same M we started, it must
		// always be possible to advance the current M's P from a ProcStop. This is also why
		// ProcStop doesn't need a sequence number.
		state, ok := o.pStates[curCtx.P]
		if !ok {
			return curCtx, false, fmt.Errorf("event %d for proc (%v) that doesn't exist", typ, curCtx.P)
		}
		if state.status != go122.ProcRunning && state.status != go122.ProcSyscall {
			return curCtx, false, fmt.Errorf("%v event for proc that's not %v or %v", typ, go122.ProcRunning, go122.ProcSyscall)
		}
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MayHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			return curCtx, false, err
		}
		state.status = go122.ProcIdle
		newCtx.P = NoProc
		return curCtx, true, nil
	case go122.EvProcSteal:
		pid := ProcID(ev.args[0])
		seq := ev.args[1]
		state, ok := o.pStates[pid]
		if !ok || (state.status != go122.ProcSyscall && state.status != go122.ProcSyscallAbandoned) || state.seq+1 != seq {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a ProcStart on a different M before the proc's state was emitted, or before we
			// got to the right point in the trace.
			return curCtx, false, nil
		}
		// We can advance this P. Check some invariants.
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MayHave, Goroutine: event.MayHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			return curCtx, false, err
		}
		// Smuggle in the P state that let us advance so we can surface information to the event.
		// Specifically, we need to make sure that the event is interpreted not as a transition of
		// ProcRunning -> ProcIdle but ProcIdle -> ProcIdle instead.
		//
		// ProcRunning is binding, but we may be running with a P on the current M and we can't
		// bind another P. This P is about to go ProcIdle anyway.
		oldStatus := state.status
		ev.args[3] = uint64(oldStatus)

		// Update the P's status and sequence number.
		state.status = go122.ProcIdle
		state.seq = seq

		// If we've lost information then don't try to do anything with the M.
		// It may have moved on and we can't be sure.
		if oldStatus == go122.ProcSyscallAbandoned {
			return curCtx, true, nil
		}

		// Validate that the M we're stealing from is what we expect.
		mid := ThreadID(ev.args[2]) // The M we're stealing from.
		mState, ok := o.mStates[mid]
		if !ok {
			return curCtx, false, fmt.Errorf("stole proc from non-existent thread %d", mid)
		}

		// Make sure we're actually stealing the right P.
		if mState.p != pid {
			return curCtx, false, fmt.Errorf("tried to steal proc %d from thread %d, but got proc %d instead", pid, mid, mState.p)
		}

		// Tell the M it has no P so it can proceed.
		//
		// This is safe because we know the P was in a syscall and
		// the other M must be trying to get out of the syscall.
		// GoSyscallEndBlocked cannot advance until the corresponding
		// M loses its P.
		mState.p = NoProc
		return curCtx, true, nil

	// Handle goroutines.
	case go122.EvGoStatus:
		gid := GoID(ev.args[0])
		status := go122.GoStatus(ev.args[1])
		oldState := go122GoStatus2GoState[status]
		if s, ok := o.gStates[gid]; ok {
			if s.status == go122.GoBad {
				oldState = GoNotExist
				s.status = status
			} else if s.status != status {
				return curCtx, false, fmt.Errorf("inconsistent status for goroutine %d: old %v vs. new %v", gid, s.status, status)
			}
			s.seq = 0 // Reset seq.
		} else if isInitialGen {
			// Set the state.
			o.gStates[gid] = &gState{id: gid, status: status}
			oldState = GoUndetermined
		} else {
			return curCtx, false, fmt.Errorf("found goroutine status for new goroutine after the first generation: id=%v status=%v", gid, status)
		}
		ev.args[2] = uint64(oldState) // Smuggle in the old state for StateTransition.

		// Bind the goroutine to the new context, if it's running.
		if status == go122.GoRunning || status == go122.GoSyscall {
			newCtx.G = gid
		}
		return curCtx, true, nil
	case go122.EvGoCreate, go122.EvGoDestroy, go122.EvGoStop, go122.EvGoBlock, go122.EvGoSyscallBegin:
		// These are goroutine events that all require an active running
		// goroutine on some thread. They must *always* be advance-able,
		// since running goroutines are bound to their M.
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		state, ok := o.gStates[curCtx.G]
		if !ok {
			return curCtx, false, fmt.Errorf("event %d for goroutine (%v) that doesn't exist", typ, curCtx.G)
		}
		if state.status != go122.GoRunning {
			return curCtx, false, fmt.Errorf("%v event for goroutine that's not %v", typ, GoRunning)
		}
		// Handle each case slightly differently; we just group them together
		// because they have shared preconditions.
		switch typ {
		case go122.EvGoCreate:
			// This goroutine created another. Add a state for it.
			newgid := GoID(ev.args[0])
			if _, ok := o.gStates[newgid]; ok {
				return curCtx, false, fmt.Errorf("tried to create goroutine (%v) that already exists", newgid)
			}
			o.gStates[newgid] = &gState{id: newgid, status: go122.GoRunnable}
		case go122.EvGoDestroy:
			// This goroutine is exiting itself.
			delete(o.gStates, curCtx.G)
			newCtx.G = NoGoroutine
		case go122.EvGoStop:
			// Goroutine stopped (yielded). It's runnable but not running on this M.
			state.status = go122.GoRunnable
			newCtx.G = NoGoroutine
		case go122.EvGoBlock:
			// Goroutine blocked. It's waiting now and not running on this M.
			state.status = go122.GoWaiting
			newCtx.G = NoGoroutine
		case go122.EvGoSyscallBegin:
			// Goroutine entered a syscall. It's still running on this P and M.
			state.status = go122.GoSyscall
			pState, ok := o.pStates[curCtx.P]
			if !ok {
				return curCtx, false, fmt.Errorf("uninitialized proc %d found during %v", curCtx.P, typ)
			}
			pState.status = go122.ProcSyscall
		}
		return curCtx, true, nil
	case go122.EvGoStart:
		gid := GoID(ev.args[0])
		seq := ev.args[1]
		state, ok := o.gStates[gid]
		if !ok || state.status != go122.GoRunnable || state.seq+1 != seq {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a GoStart on a different M before the goroutine was created, before it had its
			// state emitted, or before we got to the right point in the trace yet.
			return curCtx, false, nil
		}
		// We can advance this goroutine. Check some invariants.
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MustNotHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			return curCtx, false, err
		}
		state.status = go122.GoRunning
		state.seq = seq
		newCtx.G = gid
		return curCtx, true, nil
	case go122.EvGoUnblock:
		// N.B. These both reference the goroutine to unblock, not the current goroutine.
		gid := GoID(ev.args[0])
		seq := ev.args[1]
		state, ok := o.gStates[gid]
		if !ok || state.status != go122.GoWaiting || state.seq+1 != seq {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a GoUnblock on a different M before the goroutine was created and blocked itself,
			// before it had its state emitted, or before we got to the right point in the trace yet.
			return curCtx, false, nil
		}
		state.status = go122.GoRunnable
		state.seq = seq
		// N.B. No context to validate. Basically anything can unblock
		// a goroutine (e.g. sysmon).
		return curCtx, true, nil
	case go122.EvGoSyscallEnd:
		// This event is always advance-able because it happens on the same
		// thread that EvGoSyscallStart happened, and the goroutine can't leave
		// that thread until its done.
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		state, ok := o.gStates[curCtx.G]
		if !ok {
			return curCtx, false, fmt.Errorf("event %d for goroutine (%v) that doesn't exist", typ, curCtx.G)
		}
		if state.status != go122.GoSyscall {
			return curCtx, false, fmt.Errorf("%v event for goroutine that's not %v", typ, GoRunning)
		}
		state.status = go122.GoRunning

		// Transfer the P back to running from syscall.
		pState, ok := o.pStates[curCtx.P]
		if !ok {
			return curCtx, false, fmt.Errorf("uninitialized proc %d found during %v", curCtx.P, typ)
		}
		if pState.status != go122.ProcSyscall {
			return curCtx, false, fmt.Errorf("expected proc %d in state %v, but got %v instead", curCtx.P, go122.ProcSyscall, pState.status)
		}
		pState.status = go122.ProcRunning
		return curCtx, true, nil
	case go122.EvGoSyscallEndBlocked:
		// This event becomes advanceable when its P is not in a syscall state
		// (lack of a P altogether is also acceptable for advancing).
		// The transfer out of ProcSyscall can happen either voluntarily via
		// ProcStop or involuntarily via ProcSteal. We may also acquire a new P
		// before we get here (after the transfer out) but that's OK: that new
		// P won't be in the ProcSyscall state anymore.
		//
		// Basically: while we have a preemptible P, don't advance, because we
		// *know* from the event that we're going to lose it at some point during
		// the syscall. We shouldn't advance until that happens.
		if curCtx.P != NoProc {
			pState, ok := o.pStates[curCtx.P]
			if !ok {
				return curCtx, false, fmt.Errorf("uninitialized proc %d found during %v", curCtx.P, typ)
			}
			if pState.status == go122.ProcSyscall {
				return curCtx, false, nil
			}
		}
		// As mentioned above, we may have a P here if we ProcStart
		// before this event.
		if err := validateCtx(curCtx, event.SchedReqs{Thread: event.MustHave, Proc: event.MayHave, Goroutine: event.MustHave}); err != nil {
			return curCtx, false, err
		}
		state, ok := o.gStates[curCtx.G]
		if !ok {
			return curCtx, false, fmt.Errorf("event %d for goroutine (%v) that doesn't exist", typ, curCtx.G)
		}
		if state.status != go122.GoSyscall {
			return curCtx, false, fmt.Errorf("%v event for goroutine that's not %v", typ, GoRunning)
		}
		newCtx.G = NoGoroutine
		state.status = go122.GoRunnable
		return curCtx, true, nil

	// Handle tasks. Tasks are interesting because:
	// - There's no Begin event required to reference a task.
	// - End for a particular task ID can appear multiple times.
	// As a result, there's very little to validate. The only
	// thing we have to be sure of is that a task didn't begin
	// after it had already begun. Task IDs are allowed to be
	// reused, so we don't care about a Begin after an End.
	case go122.EvUserTaskBegin:
		id := TaskID(ev.args[0])
		if _, ok := o.activeTasks[id]; ok {
			return curCtx, false, fmt.Errorf("task ID conflict: %d", id)
		}
		o.activeTasks[id] = struct{}{}
		return curCtx, true, validateCtx(curCtx, event.UserGoReqs)
	case go122.EvUserTaskEnd:
		id := TaskID(ev.args[0])
		if _, ok := o.activeTasks[id]; ok {
			delete(o.activeTasks, id)
		}
		return curCtx, true, validateCtx(curCtx, event.UserGoReqs)

	// Handle user regions.
	case go122.EvUserRegionBegin:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		tid := TaskID(ev.args[0])
		nameID := stringID(ev.args[1])
		name, ok := evt.strings.get(nameID)
		if !ok {
			return curCtx, false, fmt.Errorf("invalid string ID %v for %v event", nameID, typ)
		}
		if err := o.gStates[curCtx.G].beginRegion(userRegion{tid, name}); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvUserRegionEnd:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		tid := TaskID(ev.args[0])
		nameID := stringID(ev.args[1])
		name, ok := evt.strings.get(nameID)
		if !ok {
			return curCtx, false, fmt.Errorf("invalid string ID %v for %v event", nameID, typ)
		}
		if err := o.gStates[curCtx.G].endRegion(userRegion{tid, name}); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle the GC mark phase.
	//
	// We have sequence numbers for both start and end because they
	// can happen on completely different threads. We want an explicit
	// partial order edge between start and end here, otherwise we're
	// relying entirely on timestamps to make sure we don't advance a
	// GCEnd for a _different_ GC cycle if timestamps are wildly broken.
	case go122.EvGCActive:
		seq := ev.args[0]
		if isInitialGen {
			if o.gcState != gcUndetermined {
				return curCtx, false, fmt.Errorf("GCActive in the first generation isn't first GC event")
			}
			o.gcSeq = seq
			o.gcState = gcRunning
			return curCtx, true, nil
		}
		if seq != o.gcSeq+1 {
			// This is not the right GC cycle.
			return curCtx, false, nil
		}
		if o.gcState != gcRunning {
			return curCtx, false, fmt.Errorf("encountered GCActive while GC was not in progress")
		}
		o.gcSeq = seq
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvGCBegin:
		seq := ev.args[0]
		if o.gcState == gcUndetermined {
			o.gcSeq = seq
			o.gcState = gcRunning
			return curCtx, true, nil
		}
		if seq != o.gcSeq+1 {
			// This is not the right GC cycle.
			return curCtx, false, nil
		}
		if o.gcState == gcRunning {
			return curCtx, false, fmt.Errorf("encountered GCBegin while GC was already in progress")
		}
		o.gcSeq = seq
		o.gcState = gcRunning
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvGCEnd:
		seq := ev.args[0]
		if seq != o.gcSeq+1 {
			// This is not the right GC cycle.
			return curCtx, false, nil
		}
		if o.gcState == gcNotRunning {
			return curCtx, false, fmt.Errorf("encountered GCEnd when GC was not in progress")
		}
		o.gcSeq = seq
		o.gcState = gcNotRunning
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle simple instantaneous events that require a G.
	case go122.EvGoLabel, go122.EvProcsChange, go122.EvUserLog:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle allocation states, which don't require a G.
	case go122.EvHeapAlloc, go122.EvHeapGoal:
		if err := validateCtx(curCtx, event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MayHave}); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle sweep, which is bound to a P and doesn't require a G.
	case go122.EvGCSweepBegin:
		if err := validateCtx(curCtx, event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MayHave}); err != nil {
			return curCtx, false, err
		}
		if err := o.pStates[curCtx.P].beginRange(makeRangeType(typ, 0)); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvGCSweepActive:
		pid := ProcID(ev.args[0])
		// N.B. In practice Ps can't block while they're sweeping, so this can only
		// ever reference curCtx.P. However, be lenient about this like we are with
		// GCMarkAssistActive; there's no reason the runtime couldn't change to block
		// in the middle of a sweep.
		if err := o.pStates[pid].activeRange(makeRangeType(typ, 0), isInitialGen); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvGCSweepEnd:
		if err := validateCtx(curCtx, event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MayHave}); err != nil {
			return curCtx, false, err
		}
		_, err := o.pStates[curCtx.P].endRange(typ)
		if err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle special goroutine-bound event ranges.
	case go122.EvSTWBegin, go122.EvGCMarkAssistBegin:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		desc := stringID(0)
		if typ == go122.EvSTWBegin {
			desc = stringID(ev.args[0])
		}
		if err := o.gStates[curCtx.G].beginRange(makeRangeType(typ, desc)); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvGCMarkAssistActive:
		gid := GoID(ev.args[0])
		// N.B. Like GoStatus, this can happen at any time, because it can
		// reference a non-running goroutine. Don't check anything about the
		// current scheduler context.
		if err := o.gStates[gid].activeRange(makeRangeType(typ, 0), isInitialGen); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case go122.EvSTWEnd, go122.EvGCMarkAssistEnd:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		desc, err := o.gStates[curCtx.G].endRange(typ)
		if err != nil {
			return curCtx, false, err
		}
		if typ == go122.EvSTWEnd {
			// Smuggle the kind into the event.
			ev.args[0] = uint64(desc)
		}
		return curCtx, true, nil
	}
	return schedCtx{}, false, fmt.Errorf("bad event type found while ordering: %v", ev.typ)
}

// schedCtx represents the scheduling resources associated with an event.
type schedCtx struct {
	G GoID
	P ProcID
	M ThreadID
}

// validateCtx ensures that ctx conforms to some reqs, returning an error if
// it doesn't.
func validateCtx(ctx schedCtx, reqs event.SchedReqs) error {
	// Check thread requirements.
	if reqs.Thread == event.MustHave && ctx.M == NoThread {
		return fmt.Errorf("expected a thread but didn't have one")
	} else if reqs.Thread == event.MustNotHave && ctx.M != NoThread {
		return fmt.Errorf("expected no thread but had one")
	}

	// Check proc requirements.
	if reqs.Proc == event.MustHave && ctx.P == NoProc {
		return fmt.Errorf("expected a proc but didn't have one")
	} else if reqs.Proc == event.MustNotHave && ctx.P != NoProc {
		return fmt.Errorf("expected no proc but had one")
	}

	// Check goroutine requirements.
	if reqs.Goroutine == event.MustHave && ctx.G == NoGoroutine {
		return fmt.Errorf("expected a goroutine but didn't have one")
	} else if reqs.Goroutine == event.MustNotHave && ctx.G != NoGoroutine {
		return fmt.Errorf("expected no goroutine but had one")
	}
	return nil
}

// userRegion represents a unique user region when attached to some gState.
type userRegion struct {
	// name must be a resolved string because the string ID for the same
	// string may change across generations, but we care about checking
	// the value itself.
	taskID TaskID
	name   string
}

// rangeType is a way to classify special ranges of time.
//
// These typically correspond 1:1 with "Begin" events, but
// they may have an optional subtype that describes the range
// in more detail.
type rangeType struct {
	typ  event.Type // "Begin" event.
	desc stringID   // Optional subtype.
}

// makeRangeType constructs a new rangeType.
func makeRangeType(typ event.Type, desc stringID) rangeType {
	if styp := go122.Specs()[typ].StartEv; styp != go122.EvNone {
		typ = styp
	}
	return rangeType{typ, desc}
}

// gState is the state of a goroutine at some point in the
//
// It's mostly just data, managed by ordering.
type gState struct {
	id     GoID
	status go122.GoStatus
	seq    uint64 // Sequence counter for ordering.

	// regions are the active user regions for this goroutine.
	regions []userRegion

	// rangeState is the state of special time ranges bound to this goroutine.
	rangeState
}

// beginRegion starts a user region on the goroutine.
func (s *gState) beginRegion(r userRegion) error {
	s.regions = append(s.regions, r)
	return nil
}

// endRegion ends a user region on the goroutine.
func (s *gState) endRegion(r userRegion) error {
	if next := s.regions[len(s.regions)-1]; next != r {
		return fmt.Errorf("misuse of region in goroutine %v: region end %v when the inner-most active region start event is %v", s.id, r, next)
	}
	s.regions = s.regions[:len(s.regions)-1]
	return nil
}

// pState is the state of a proc at some point in the
//
// It's just data, managed by ordering.
type pState struct {
	id     ProcID
	status go122.ProcStatus
	seq    uint64 // Sequence counter for ordering.

	// rangeState is the state of special time ranges bound to this proc.
	rangeState
}

// pState is the state of a thread at some point in the
//
// It's just data, managed by ordering.
type mState struct {
	g GoID
	p ProcID
}

// rangeState represents the state of special time ranges.
type rangeState struct {
	// inFlight contains the rangeTypes of any ranges bound to a resource.
	inFlight []rangeType
}

// beginRange begins a special range in time on the goroutine.
//
// Returns an error if the range is already in progress.
func (s *rangeState) beginRange(typ rangeType) error {
	if s.hasRange(typ) {
		return fmt.Errorf("discovered event already in-flight for when starting event %v", go122.Specs()[typ.typ].Name)
	}
	s.inFlight = append(s.inFlight, typ)
	return nil
}

// activeRange marks special range in time on the goroutine as active in the
// initial generation, or confirms that it is indeed active in later generations.
func (s *rangeState) activeRange(typ rangeType, isInitialGen bool) error {
	if isInitialGen {
		if s.hasRange(typ) {
			return fmt.Errorf("found named active range already in first gen: %v", typ)
		}
		s.inFlight = append(s.inFlight, typ)
	} else if !s.hasRange(typ) {
		return fmt.Errorf("resource is missing active range: %v %v", go122.Specs()[typ.typ].Name, s.inFlight)
	}
	return nil
}

// hasRange returns true if a special time range on the goroutine as in progress.
func (s *rangeState) hasRange(typ rangeType) bool {
	for _, ftyp := range s.inFlight {
		if ftyp == typ {
			return true
		}
	}
	return false
}

// endsRange ends a special range in time on the goroutine.
//
// This must line up with the start event type  of the range the goroutine is currently in.
func (s *rangeState) endRange(typ event.Type) (stringID, error) {
	st := go122.Specs()[typ].StartEv
	idx := -1
	for i, r := range s.inFlight {
		if r.typ == st {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, fmt.Errorf("tried to end event %v, but not in-flight", go122.Specs()[st].Name)
	}
	// Swap remove.
	desc := s.inFlight[idx].desc
	s.inFlight[idx], s.inFlight[len(s.inFlight)-1] = s.inFlight[len(s.inFlight)-1], s.inFlight[idx]
	s.inFlight = s.inFlight[:len(s.inFlight)-1]
	return desc, nil
}

func dumpOrdering(order *ordering) string {
	var sb strings.Builder
	for id, state := range order.gStates {
		fmt.Fprintf(&sb, "G %d [status=%s seq=%d]\n", id, state.status, state.seq)
	}
	fmt.Fprintln(&sb)
	for id, state := range order.pStates {
		fmt.Fprintf(&sb, "P %d [status=%s seq=%d]\n", id, state.status, state.seq)
	}
	fmt.Fprintln(&sb)
	for id, state := range order.mStates {
		fmt.Fprintf(&sb, "M %d [g=%d p=%d]\n", id, state.g, state.p)
	}
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "GC %d %s\n", order.gcSeq, order.gcState)
	return sb.String()
}
