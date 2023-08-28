// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"strings"

	"golang.org/x/exp/trace/internal/event"
	v2 "golang.org/x/exp/trace/internal/v2"
)

// ordering emulates Go scheduler state for both validation and
// for putting events in the right order.
type ordering struct {
	gStates      map[GoID]*gState
	pStates      map[ProcID]*pState // TODO: The keys are dense, so this can be a slice.
	mStates      map[ThreadID]*mState
	activeTasks  map[TaskID]struct{}
	gcSeq        uint64
	gcInProgress bool
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
func (o *ordering) advance(ev *baseEvent, evt *evTable, m ThreadID) (schedCtx, bool, error) {
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
	case v2.EvProcStatus:
		pid := ProcID(ev.args[0])
		status := v2.ProcStatus(ev.args[1])
		if s, ok := o.pStates[pid]; ok {
			if s.status == v2.ProcBad {
				s.status = status
			} else if s.status != status {
				return schedCtx{}, false, fmt.Errorf("inconsistent proc status: old %v vs. new %v", s.status, status)
			}
			s.seq = 0 // Reset seq.
		} else {
			o.pStates[pid] = &pState{id: pid, status: status}
		}
		if status == v2.ProcRunning {
			newCtx.P = pid
		}
		return curCtx, true, nil
	case v2.EvProcStart:
		pid := ProcID(ev.args[0])
		seq := ev.args[1]

		// Check if we're currently in a syscalling G.
		g, haveG := o.gStates[curCtx.G]
		inSyscallWithP := curCtx.P != NoProc && haveG && g.status == v2.GoSyscall

		// Try to advance. We might fail here due to sequencing, because the P hasn't
		// had a status emitted, or because we already have a P and we're in a syscall,
		// and we haven't observed that it was stolen from us yet.
		state, ok := o.pStates[pid]
		if !ok || state.status != v2.ProcIdle || state.seq+1 != seq || inSyscallWithP {
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
			println(curCtx.M, curCtx.G, curCtx.P)
			return curCtx, false, err
		}
		state.status = v2.ProcRunning
		state.seq = seq
		newCtx.P = pid
		return curCtx, true, nil
	case v2.EvProcStop:
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
		if state.status != v2.ProcRunning {
			return curCtx, false, fmt.Errorf("%v event for proc that's not %v", typ, ProcRunning)
		}
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MayHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			return curCtx, false, err
		}
		state.status = v2.ProcIdle
		newCtx.P = NoProc
		return curCtx, true, nil
	case v2.EvProcSteal:
		pid := ProcID(ev.args[0])
		seq := ev.args[1]

		// Determine if we're ready to steal.
		stealReady := false
		mid := ThreadID(ev.args[2]) // The M we're stealing from.
		m, mExists := o.mStates[mid]
		if mExists && m.g != NoGoroutine {
			g, gExists := o.gStates[m.g]
			stealReady = gExists && g.status == v2.GoSyscall
		}
		state, ok := o.pStates[pid]
		if !ok || state.status != v2.ProcRunning || state.seq+1 != seq || !stealReady {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a ProcStart on a different M before the proc's state was emitted, or before we
			// got to the right point in the trace.
			//
			// Note that
			return curCtx, false, nil
		}
		// We can advance this P. Check some invariants.
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MayHave, Goroutine: event.MayHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			return curCtx, false, err
		}
		state.status = v2.ProcIdle
		state.seq = seq

		// Tell the M it has no P so it can proceed.
		m.p = NoProc
		return curCtx, true, nil

	// Handle goroutines.
	case v2.EvGoStatus:
		gid := GoID(ev.args[0])
		status := v2.GoStatus(ev.args[1])
		if s, ok := o.gStates[gid]; ok {
			if s.status == v2.GoBad {
				s.status = status
			} else if s.status != status {
				return schedCtx{}, false, fmt.Errorf("inconsistent go status: old %v vs. new %v", s.status, status)
			}
			s.seq = 0 // Reset seq.
		} else {
			o.gStates[gid] = &gState{id: gid, status: status}
		}
		if status == v2.GoRunning || status == v2.GoSyscall {
			newCtx.G = gid
		}
		return curCtx, true, nil
	case v2.EvGoCreate, v2.EvGoDestroy, v2.EvGoStop, v2.EvGoBlock, v2.EvGoSyscallBegin:
		// These are goroutine events that all require an active running
		// goroutine on some thread. They must *always* be advance-able.
		//
		// The reason why they must always be advance-able is a bit subtle.
		// These must happen on a running goroutine. A goroutine can only be
		// running if in a particular M's event stream
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		state, ok := o.gStates[curCtx.G]
		if !ok {
			return curCtx, false, fmt.Errorf("event %d for goroutine (%v) that doesn't exist", typ, curCtx.G)
		}
		if state.status != v2.GoRunning {
			return curCtx, false, fmt.Errorf("%v event for goroutine that's not %v", typ, GoRunning)
		}
		// Handle each case slightly differently; we just group them together
		// because they have shared preconditions.
		switch typ {
		case v2.EvGoCreate:
			// This goroutine created another. Add a state for it.
			newgid := GoID(ev.args[0])
			if _, ok := o.gStates[newgid]; ok {
				return curCtx, false, fmt.Errorf("tried to create goroutine (%v) that already exists", newgid)
			}
			o.gStates[newgid] = &gState{id: newgid, status: v2.GoRunnable}
		case v2.EvGoDestroy:
			// This goroutine is exiting itself.
			delete(o.gStates, curCtx.G)
			newCtx.G = NoGoroutine
		case v2.EvGoStop:
			// Goroutine stopped (yielded). It's runnable but not running on this M.
			state.status = v2.GoRunnable
			newCtx.G = NoGoroutine
		case v2.EvGoBlock:
			// Goroutine blocked. It's waiting now and not running on this M.
			state.status = v2.GoWaiting
			newCtx.G = NoGoroutine
		case v2.EvGoSyscallBegin:
			// Goroutine entered a syscall. It's still running on this P and M.
			state.status = v2.GoSyscall
		}
		return curCtx, true, nil
	case v2.EvGoStart:
		gid := GoID(ev.args[0])
		seq := ev.args[1]
		state, ok := o.gStates[gid]
		if !ok || state.status != v2.GoRunnable || state.seq+1 != seq {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a GoStart on a different M before the goroutine was created, before it had its
			// state emitted, or before we got to the right point in the trace yet.
			return curCtx, false, nil
		}
		// We can advance this goroutine. Check some invariants.
		reqs := event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MustNotHave}
		if err := validateCtx(curCtx, reqs); err != nil {
			println(curCtx.M, curCtx.P, curCtx.G)
			return curCtx, false, err
		}
		state.status = v2.GoRunning
		state.seq = seq
		newCtx.G = gid
		return curCtx, true, nil
	case v2.EvGoUnblock:
		// N.B. These both reference the goroutine to unblock, not the current goroutine.
		gid := GoID(ev.args[0])
		seq := ev.args[1]
		state, ok := o.gStates[gid]
		if !ok || state.status != v2.GoWaiting || state.seq+1 != seq {
			// We can't make an inference as to whether this is bad. We could just be seeing
			// a GoUnblock on a different M before the goroutine was created and blocked itself,
			// before it had its state emitted, or before we got to the right point in the trace yet.
			return curCtx, false, nil
		}
		state.status = v2.GoRunnable
		state.seq = seq
		// N.B. No context to validate. Basically anything can unblock
		// a goroutine (e.g. sysmon).
		return curCtx, true, nil
	case v2.EvGoSyscallEnd:
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
		if state.status != v2.GoSyscall {
			return curCtx, false, fmt.Errorf("%v event for goroutine that's not %v", typ, GoRunning)
		}
		state.status = v2.GoRunning
		return curCtx, true, nil
	case v2.EvGoSyscallEndBlocked:
		// This event is always advance-able because it happens on the same
		// thread that EvGoSyscallStart happened, and the goroutine can't leave
		// that thread until its done.
		//
		// We *may* still have a P if we encounter this event before a ProcSteal.
		if err := validateCtx(curCtx, event.SchedReqs{Thread: event.MustHave, Proc: event.MayHave, Goroutine: event.MustHave}); err != nil {
			return curCtx, false, err
		}
		state, ok := o.gStates[curCtx.G]
		if !ok {
			return curCtx, false, fmt.Errorf("event %d for goroutine (%v) that doesn't exist", typ, curCtx.G)
		}
		if state.status != v2.GoSyscall {
			return curCtx, false, fmt.Errorf("%v event for goroutine that's not %v", typ, GoRunning)
		}
		newCtx.G = NoGoroutine
		state.status = v2.GoRunnable
		return curCtx, true, nil

	// Handle tasks. Tasks are interesting because:
	// - There's no Begin event required to reference a task.
	// - End for a particular task ID can appear multiple times.
	// As a result, there's very little to validate. The only
	// thing we have to be sure of is that a task didn't begin
	// after it had already begun. Task IDs are allowed to be
	// reused, so we don't care about a Begin after an End.
	case v2.EvUserTaskBegin:
		id := TaskID(ev.args[0])
		if _, ok := o.activeTasks[id]; ok {
			return curCtx, false, fmt.Errorf("task ID conflict: %d", id)
		}
		o.activeTasks[id] = struct{}{}
		return curCtx, true, validateCtx(curCtx, event.UserGoReqs)
	case v2.EvUserTaskEnd:
		id := TaskID(ev.args[0])
		if _, ok := o.activeTasks[id]; ok {
			delete(o.activeTasks, id)
		}
		return curCtx, true, validateCtx(curCtx, event.UserGoReqs)

	// Handle user regions.
	case v2.EvUserRegionBegin:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		tid := TaskID(ev.args[0])
		nameID := stringID(ev.args[1])
		name, ok := evt.strings[nameID]
		if !ok {
			return curCtx, false, fmt.Errorf("invalid string ID %v for %v event", nameID, typ)
		}
		if err := o.gStates[curCtx.G].beginRegion(userRegion{tid, name}); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case v2.EvUserRegionEnd:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		tid := TaskID(ev.args[0])
		nameID := stringID(ev.args[1])
		name, ok := evt.strings[nameID]
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
	case v2.EvGCBegin:
		seq := ev.args[0]
		if seq != o.gcSeq+1 {
			// This is not the right GC cycle.
			return curCtx, false, nil
		}
		if o.gcInProgress {
			return curCtx, false, fmt.Errorf("encountered GCBegin while GC was already in progress")
		}
		o.gcSeq = seq
		o.gcInProgress = true
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case v2.EvGCEnd:
		seq := ev.args[0]
		if seq != o.gcSeq+1 {
			// This is not the right GC cycle.
			return curCtx, false, nil
		}
		if !o.gcInProgress {
			return curCtx, false, fmt.Errorf("encountered GCEnd when GC was not in progress")
		}
		o.gcSeq = seq
		o.gcInProgress = false
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle allocation states, which don't require a G.
	case v2.EvHeapAlloc, v2.EvHeapGoal:
		if err := validateCtx(curCtx, event.SchedReqs{Thread: event.MustHave, Proc: event.MustHave, Goroutine: event.MayHave}); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle simple instantaneous events that require a G
	case v2.EvGoLabel, v2.EvProcsChange, v2.EvUserLog:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil

	// Handle special goroutine-bound event ranges.
	case v2.EvSTWBegin, v2.EvGCSweepBegin, v2.EvGCMarkAssistBegin:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		if err := o.gStates[curCtx.G].beginRange(typ); err != nil {
			return curCtx, false, err
		}
		return curCtx, true, nil
	case v2.EvSTWEnd, v2.EvGCSweepEnd, v2.EvGCMarkAssistEnd:
		if err := validateCtx(curCtx, event.UserGoReqs); err != nil {
			return curCtx, false, err
		}
		if err := o.gStates[curCtx.G].endRange(typ); err != nil {
			return curCtx, false, err
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

// gState is the state of a goroutine at some point in the
//
// It's mostly just data, managed by ordering.
type gState struct {
	id     GoID // Goroutine ID.
	status v2.GoStatus
	seq    uint64 // Sequence counter for ordering.

	// regions are the active user regions for this goroutine.
	regions []userRegion

	// inFlight is the event start type of a special range in time
	// that the goroutine is in.
	inFlight event.Type
}

// beginRange begins a special range in time on the goroutine.
func (s *gState) beginRange(typ event.Type) error {
	if s.inFlight != v2.EvNone {
		return fmt.Errorf("discovered event %v already in-flight for goroutine %v when starting event %v", s.inFlight, s.id, typ)
	}
	s.inFlight = typ
	return nil
}

// endsRange ends a special range in time on the goroutine.
//
// This must line up with the start event type  of the range the goroutine is currently in.
func (s *gState) endRange(typ event.Type) error {
	st := v2.Specs()[typ].StartEv
	if s.inFlight != st {
		return fmt.Errorf("tried to end event %v, but not in-flight for goroutine %v", st, s.id)
	}
	s.inFlight = v2.EvNone
	return nil
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
	status v2.ProcStatus
	seq    uint64
}

// pState is the state of a thread at some point in the
//
// It's just data, managed by ordering.
type mState struct {
	g GoID
	p ProcID
}

func dumpOrdering(order *ordering) string {
	var sb strings.Builder
	for id, state := range order.gStates {
		fmt.Fprintf(&sb, "G %d [status=%d seq=%d]\n", id, state.status, state.seq)
	}
	fmt.Fprintln(&sb)
	for id, state := range order.pStates {
		fmt.Fprintf(&sb, "P %d [status=%d seq=%d]\n", id, state.status, state.seq)
	}
	fmt.Fprintln(&sb)
	for id, state := range order.mStates {
		fmt.Fprintf(&sb, "M %d [g=%d p=%d]\n", id, state.g, state.p)
	}
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "GC %d\n", order.gcSeq)
	return sb.String()
}
