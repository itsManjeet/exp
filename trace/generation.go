// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/trace/internal/event"
	v2 "golang.org/x/exp/trace/internal/v2"
)

// generation contains all the trace data for a single
// trace generation. It is purely data: it does not
// track any parse state nor does it contain a cursor
// into the generation.
type generation struct {
	gen        uint64
	batches    map[ThreadID][]batch
	cpuSamples []cpuSample
	*evTable
}

// spilledBatch represents a batch that was read out for the next generation,
// while reading the previous one. It's passed on when parsing the next
// generation.
type spilledBatch struct {
	gen uint64
	*batch
}

// readGeneration buffers and decodes the structural elements of a trace generation
// out of r. spill is the first batch of the new generation (already buffered and
// parsed from reading the last generation). Returns the generation and the first
// batch read of the next generation, if any.
func readGeneration(r *bufio.Reader, spill *spilledBatch) (*generation, *spilledBatch, error) {
	g := &generation{
		gen:     1,
		evTable: new(evTable),
		batches: make(map[ThreadID][]batch),
	}
	// Process the spilled batch.
	if spill != nil {
		g.gen = spill.gen
		if err := processBatch(g, *spill.batch); err != nil {
			return nil, nil, err
		}
		spill = nil
	}
	// Read batches one at a time until we either hit EOF or
	// the next generation.
	for {
		b, gen, err := readBatch(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if gen == g.gen+1 {
			spill = &spilledBatch{gen: gen, batch: &b}
			break
		}
		if gen != g.gen {
			// N.B. Fail as fast as possible if we see this. At first it
			// may seem prudent to be fault-tolerant and assume we have a
			// complete generation, parsing and returning that first. However,
			// if the batches are mixed across generations then it's likely
			// we won't be able to parse this generation correctly at all.
			// Rather than return a cryptic error in that case, indicate the
			// problem as soon as we see it.
			return nil, nil, fmt.Errorf("generations out of order")
		}
		if err := processBatch(g, b); err != nil {
			return nil, nil, err
		}
	}

	// Check some invariants.
	if g.freq == 0 {
		return nil, nil, fmt.Errorf("no frequency event found")
	}
	for _, batches := range g.batches {
		sorted := slices.IsSortedFunc(batches, func(a, b batch) int {
			return cmp.Compare(a.time, b.time)
		})
		if !sorted {
			// TODO(mknyszek): Consider just sorting here.
			return nil, nil, fmt.Errorf("per-M streams are out-of-order")
		}
	}

	// Compactify stacks and strings for better lookup performance later.
	g.stacks.compactify()
	g.strings.compactify()

	// Fix up the CPU sample timestamps, now that we have freq.
	for i := range g.cpuSamples {
		s := &g.cpuSamples[i]
		s.time = g.freq.mul(timestamp(s.time))
	}
	// Sort the CPU samples.
	slices.SortFunc(g.cpuSamples, func(a, b cpuSample) int {
		return cmp.Compare(a.time, b.time)
	})
	return g, spill, nil
}

// processBatch adds the batch to the generation.
func processBatch(g *generation, b batch) error {
	switch {
	case b.isStringsBatch():
		if err := addStrings(&g.strings, b); err != nil {
			return err
		}
	case b.isStacksBatch():
		if err := addStacks(&g.stacks, b); err != nil {
			return err
		}
	case b.isCPUSamplesBatch():
		samples, err := addCPUSamples(g.cpuSamples, b)
		if err != nil {
			return err
		}
		g.cpuSamples = samples
	case b.isFreqBatch():
		freq, err := parseFreq(b)
		if err != nil {
			return err
		}
		if g.freq != 0 {
			return fmt.Errorf("found multiple frequency events")
		}
		g.freq = freq
	default:
		g.batches[b.m] = append(g.batches[b.m], b)
	}
	return nil
}

const maxStringSize = 1 << 10

// addStrings takes a batch whose first byte is a EvStrings event
// (indicating that the batch contains only strings) and adds each
// string contained therein to the provided strings map.
func addStrings(stringTable *dataTable[stringID, string], b batch) error {
	if !b.isStringsBatch() {
		return fmt.Errorf("internal error: addStrings called on non-string batch")
	}
	r := bytes.NewReader(b.data)
	r.ReadByte() // Consume the EvStrings byte.

	var sb strings.Builder
	for r.Len() != 0 {
		// Read the header.
		ev, err := r.ReadByte()
		if err != nil {
			return err
		}
		if event.Type(ev) != v2.EvString {
			return fmt.Errorf("expected string event, got %d", ev)
		}

		// Read the string's ID.
		id, err := binary.ReadUvarint(r)
		if err != nil {
			return err
		}

		// Read the string's length.
		len, err := binary.ReadUvarint(r)
		if err != nil {
			return err
		}
		if len > maxStringSize {
			return fmt.Errorf("invalid string size %d, maximum is %d", len, maxStringSize)
		}

		// Copy out the string.
		n, err := io.CopyN(&sb, r, int64(len))
		if n != int64(len) {
			return fmt.Errorf("failed to read full string: read %d but wanted %d", n, len)
		}
		if err != nil {
			return fmt.Errorf("copying string data: %w", err)
		}

		// Add the string to the map.
		s := sb.String()
		sb.Reset()
		if err := stringTable.insert(stringID(id), s); err != nil {
			return err
		}
	}
	return nil
}

const maxStackSize = 128

// addStacks takes a batch whose first byte is a EvStacks event
// (indicating that the batch contains only stacks) and adds each
// string contained therein to the provided stacks map.
func addStacks(stackTable *dataTable[stackID, stack], b batch) error {
	if !b.isStacksBatch() {
		return fmt.Errorf("internal error: addStacks called on non-stacks batch")
	}
	r := bytes.NewReader(b.data)
	r.ReadByte() // Consume the EvStacks byte.

	for r.Len() != 0 {
		// Read the header.
		ev, err := r.ReadByte()
		if err != nil {
			return err
		}
		if event.Type(ev) != v2.EvStack {
			return fmt.Errorf("expected stack event, got %d", ev)
		}

		// Read the stack's ID.
		id, err := binary.ReadUvarint(r)
		if err != nil {
			return err
		}

		// Read how many frames are in each stack.
		nFrames, err := binary.ReadUvarint(r)
		if err != nil {
			return err
		}
		if nFrames > maxStackSize {
			return fmt.Errorf("invalid stack size %d, maximum is %d", nFrames, maxStackSize)
		}

		// Each frame consists of 4 fields: pc, funcID (string), fileID (string), line.
		frames := make([]frame, 0, nFrames)
		for i := uint64(0); i < nFrames; i++ {
			// Read the frame data.
			pc, err := binary.ReadUvarint(r)
			if err != nil {
				return fmt.Errorf("reading frame %d's PC for stack %d: %w", i+1, id, err)
			}
			funcID, err := binary.ReadUvarint(r)
			if err != nil {
				return fmt.Errorf("reading frame %d's funcID for stack %d: %w", i+1, id, err)
			}
			fileID, err := binary.ReadUvarint(r)
			if err != nil {
				return fmt.Errorf("reading frame %d's fileID for stack %d: %w", i+1, id, err)
			}
			line, err := binary.ReadUvarint(r)
			if err != nil {
				return fmt.Errorf("reading frame %d's line for stack %d: %w", i+1, id, err)
			}
			frames = append(frames, frame{
				pc:     pc,
				funcID: stringID(funcID),
				fileID: stringID(fileID),
				line:   line,
			})
		}

		// Add the stack to the map.
		if err := stackTable.insert(stackID(id), stack{frames: frames}); err != nil {
			return err
		}
	}
	return nil
}

// addCPUSamples takes a batch whose first byte is a EvCPUSamples event
// (indicating that the batch contains only CPU samples) and adds each
// sample contained therein to the provided samples list.
func addCPUSamples(samples []cpuSample, b batch) ([]cpuSample, error) {
	if !b.isCPUSamplesBatch() {
		return nil, fmt.Errorf("internal error: addStrings called on non-string batch")
	}
	r := bytes.NewReader(b.data)
	r.ReadByte() // Consume the EvCPUSamples byte.
	for r.Len() != 0 {
		// Read the header.
		ev, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if event.Type(ev) != v2.EvCPUSample {
			return nil, fmt.Errorf("expected CPU sample event, got %d", ev)
		}

		// Read the sample's timestamp.
		ts, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}

		// Read the sample's M.
		m, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}

		// Read the sample's P.
		p, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}

		// Read the sample's G.
		g, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}

		// Read the sample's stack.
		s, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}

		// Add the sample to the slice.
		samples = append(samples, cpuSample{
			schedCtx: schedCtx{
				M: ThreadID(m),
				P: ProcID(p),
				G: GoID(g),
			},
			time:  Time(ts), // N.B. this is really a "timestamp," not a Time.
			stack: stackID(s),
		})
	}
	return samples, nil
}

// parseFreq parses out a lone EvFrequency from a batch.
func parseFreq(b batch) (frequency, error) {
	if !b.isFreqBatch() {
		return 0, fmt.Errorf("internal error: parseFreq called on non-frequency batch")
	}
	r := bytes.NewReader(b.data)
	r.ReadByte() // Consume the EvFrequency byte.

	// Read the frequency. It'll come out as timestamp units per second.
	f, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	// Convert to nanoseconds per timestamp unit.
	return frequency(1.0 / (float64(f) / 1e9)), nil
}
