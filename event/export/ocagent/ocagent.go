// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ocagent adds the ability to export all telemetry to an ocagent.
// This keeps the compile time dependencies to zero and allows the agent to
// have the exporters needed for telemetry aggregation and viewing systems.
package ocagent

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/export/metric"
	"golang.org/x/exp/event/export/ocagent/wire"
	"golang.org/x/exp/event/keys"
)

type Config struct {
	Start       time.Time
	Host        string
	Process     uint32
	Client      *http.Client
	Service     string
	Address     string
	Rate        time.Duration
	IDGenerator IDGenerator
}

type IDGenerator interface {
	TraceID(event.Event) []byte
	SpanID(event.Event) []byte
}

var (
	connectMu sync.Mutex
	exporters = make(map[Config]*Exporter)
)

// Discover finds the local agent to export to, it will return nil if there
// is not one running.
// TODO: Actually implement a discovery protocol rather than a hard coded address
func Discover() *Config {
	return &Config{
		Address: "http://localhost:55678",
	}
}

type Exporter struct {
	mu        sync.Mutex
	config    Config
	inFlight  map[uint64]*wire.Span
	completed []*wire.Span
	//TODO:metrics []metric.Data
}

// Connect creates a process specific exporter with the specified
// serviceName and the address of the ocagent to which it will upload
// its telemetry.
func Connect(config *Config) *Exporter {
	if config == nil || config.Address == "off" {
		return nil
	}
	resolved := *config
	if resolved.Host == "" {
		hostname, _ := os.Hostname()
		resolved.Host = hostname
	}
	if resolved.Process == 0 {
		resolved.Process = uint32(os.Getpid())
	}
	if resolved.Client == nil {
		resolved.Client = http.DefaultClient
	}
	if resolved.Service == "" {
		resolved.Service = filepath.Base(os.Args[0])
	}
	if resolved.Rate == 0 {
		resolved.Rate = 2 * time.Second
	}
	if resolved.IDGenerator == nil {
		resolved.IDGenerator = newIDGenerator()
	}

	connectMu.Lock()
	defer connectMu.Unlock()
	if exporter, found := exporters[resolved]; found {
		return exporter
	}
	exporter := &Exporter{
		config:   resolved,
		inFlight: map[uint64]*wire.Span{},
	}
	exporters[resolved] = exporter
	if exporter.config.Start.IsZero() {
		exporter.config.Start = time.Now()
	}
	go func() {
		for range time.Tick(exporter.config.Rate) {
			exporter.Flush()
		}
	}()
	return exporter
}

func (e *Exporter) Export(ev event.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	parent := e.inFlight[ev.Parent]
	switch ev.Kind {
	case event.StartKind:
		span := &wire.Span{
			SpanID:                  e.config.IDGenerator.SpanID(ev),
			TraceState:              nil, //TODO?
			Kind:                    wire.UnspecifiedSpanKind,
			StartTime:               convertTimestamp(ev.At),
			Attributes:              convertAttributes(ev),
			SameProcessAsParentSpan: true,
			Name:                    toTruncatableString(ev.Message),
		}

		if parent == nil {
			span.TraceID = e.config.IDGenerator.TraceID(ev)
		} else {
			span.TraceID = parent.TraceID
			span.ParentSpanID = parent.SpanID
		}
		//TODO: TimeEvents:              convertEvents(span.Events()),
		//TODO: ParentSpanID:            span.ParentID[:],
		//TODO: StackTrace?
		//TODO: Links?
		//TODO: Status?
		//TODO: Resource?

		//TODO:
		e.inFlight[ev.ID] = span
	case event.EndKind:
		delete(e.inFlight, ev.Parent)
		parent.EndTime = convertTimestamp(ev.At)
		e.completed = append(e.completed, parent)
	case event.MetricKind:
		//TODO: data := metric.Entries.Get(ev).([]metric.Data)
		//TODO: e.metrics = append(e.metrics, data...)
	case event.LogKind, nil:
		if parent.TimeEvents == nil {
			parent.TimeEvents = &wire.TimeEvents{}
		}
		parent.TimeEvents.TimeEvent = append(parent.TimeEvents.TimeEvent, convertTimeEvent(ev))
	default:
		fmt.Fprintf(os.Stderr, "other %v %d %d\n", ev.Kind, ev.ID, ev.Parent)
	}
}

func (e *Exporter) Flush() {
	e.mu.Lock()
	defer e.mu.Unlock()
	/*TODO:
	metrics := make([]*wire.Metric, len(e.metrics))
	for i, m := range e.metrics {
		metrics[i] = convertMetric(m, e.config.Start)
	}
	e.metrics = nil
	*/
	if len(e.completed) > 0 {
		e.send("/v1/trace", &wire.ExportTraceServiceRequest{
			Node:  e.config.buildNode(),
			Spans: e.completed,
			//TODO: Resource?
		})
		// clear it down but leave the space reserved
		e.completed = e.completed[:0]
	}
	/* TODO:
	if len(metrics) > 0 {
		e.send("/v1/metrics", &wire.ExportMetricsServiceRequest{
			Node:    e.config.buildNode(),
			Metrics: metrics,
			//TODO: Resource?
		})
	}
	*/
}

func (cfg *Config) buildNode() *wire.Node {
	return &wire.Node{
		Identifier: &wire.ProcessIdentifier{
			HostName:       cfg.Host,
			Pid:            cfg.Process,
			StartTimestamp: convertTimestamp(cfg.Start),
		},
		LibraryInfo: &wire.LibraryInfo{
			Language:           wire.LanguageGo,
			ExporterVersion:    "0.0.1",
			CoreLibraryVersion: "x/tools",
		},
		ServiceInfo: &wire.ServiceInfo{
			Name: cfg.Service,
		},
	}
}

func (e *Exporter) send(endpoint string, message interface{}) {
	blob, err := json.Marshal(message)
	if err != nil {
		errorInExport("ocagent failed to marshal message for %v: %v", endpoint, err)
		return
	}
	uri := e.config.Address + endpoint
	req, err := http.NewRequest("POST", uri, bytes.NewReader(blob))
	if err != nil {
		errorInExport("ocagent failed to build request for %v: %v", uri, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := e.config.Client.Do(req)
	if err != nil {
		errorInExport("ocagent failed to send message: %v \n", err)
		return
	}
	if res.Body != nil {
		res.Body.Close()
	}
}

type defaultIDGenerator struct {
	rand *rand.Rand
}

func newIDGenerator() *defaultIDGenerator {
	var seed int64
	binary.Read(crand.Reader, binary.LittleEndian, &seed)
	return &defaultIDGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

func (g *defaultIDGenerator) TraceID(ev event.Event) []byte {
	tid := make([]byte, 16)
	g.rand.Read(tid)
	return tid
}

func (g *defaultIDGenerator) SpanID(ev event.Event) []byte {
	sid := make([]byte, 8)
	binary.LittleEndian.PutUint64(sid, ev.ID)
	return sid
}

func errorInExport(message string, args ...interface{}) {
	// This function is useful when debugging the exporter, but in general we
	// want to just drop any export
}

func convertTimestamp(t time.Time) wire.Timestamp {
	return t.Format(time.RFC3339Nano)
}

func toTruncatableString(s string) *wire.TruncatableString {
	if s == "" {
		return nil
	}
	return &wire.TruncatableString{Value: s}
}

func convertMetric(data metric.Data, start time.Time) *wire.Metric {
	descriptor := dataToMetricDescriptor(data)
	timeseries := dataToTimeseries(data, start)

	if descriptor == nil && timeseries == nil {
		return nil
	}

	// TODO: handle Histogram metrics
	return &wire.Metric{
		MetricDescriptor: descriptor,
		Timeseries:       timeseries,
		// TODO: attach Resource?
	}
}

func convertAttributes(ev event.Event) *wire.Attributes {
	attributes := make(map[string]wire.Attribute)
	for _, l := range ev.Static {
		addAttribute(attributes, l)
	}
	for _, l := range ev.Dynamic {
		addAttribute(attributes, l)
	}
	if len(attributes) == 0 {
		return nil
	}
	return &wire.Attributes{AttributeMap: attributes}
}

func addAttribute(attributes map[string]wire.Attribute, l event.Label) {
	if !l.Valid() {
		return
	}
	attr := convertAttribute(l)
	if attr != nil {
		attributes[l.Key().Name()] = attr
	}
}

func convertAttribute(l event.Label) wire.Attribute {
	switch key := l.Key().(type) {
	case *keys.Int:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.Int8:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.Int16:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.Int32:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.Int64:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.UInt:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.UInt8:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.UInt16:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.UInt32:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.UInt64:
		return wire.IntAttribute{IntValue: int64(key.From(l))}
	case *keys.Float32:
		return wire.DoubleAttribute{DoubleValue: float64(key.From(l))}
	case *keys.Float64:
		return wire.DoubleAttribute{DoubleValue: key.From(l)}
	case *keys.Boolean:
		return wire.BoolAttribute{BoolValue: key.From(l)}
	case *keys.String:
		return wire.StringAttribute{StringValue: toTruncatableString(key.From(l))}
	case *keys.Value:
		return wire.StringAttribute{StringValue: toTruncatableString(fmt.Sprint(key.From(l)))}
	case event.ErrKey:
		return wire.StringAttribute{StringValue: toTruncatableString(l.UnpackValue().(error).Error())}
	default:
		return wire.StringAttribute{StringValue: toTruncatableString(fmt.Sprintf("%T", key))}
	}
}

func convertTimeEvent(ev event.Event) wire.TimeEvent {
	return wire.TimeEvent{
		Time:       convertTimestamp(ev.At),
		Annotation: convertAnnotation(ev),
	}
}

func getAnnotationDescription(ev event.Event) string {
	if ev.Kind != event.LogKind {
		return ""
	}
	if ev.Message != "" {
		return ev.Message
	}
	if ev.Static[0].Key() != (event.ErrKey{}) {
		return ""
	}
	err := ev.Static[0].UnpackValue().(error)
	if err == nil {
		return ""
	}
	return err.Error()
}

func convertAnnotation(ev event.Event) *wire.Annotation {
	annotation := &wire.Annotation{}
	attributeMap := make(map[string]wire.Attribute)
	remains := ev.Static[:]
	if ev.Kind == event.LogKind {
		annotation.Description = toTruncatableString(ev.Message)
		if annotation.Description == nil && remains[0].Key() == (event.ErrKey{}) {
			err := remains[0].UnpackValue().(error)
			remains = remains[1:]
			if err != nil {
				annotation.Description = toTruncatableString(err.Error())
			}
		}
	}
	for _, l := range remains {
		addAttribute(attributeMap, l)
	}
	for _, l := range ev.Dynamic {
		addAttribute(attributeMap, l)
	}
	if len(attributeMap) > 0 {
		annotation.Attributes = &wire.Attributes{AttributeMap: attributeMap}
	}
	if annotation.Description == nil && annotation.Attributes == nil {
		return nil
	}
	return annotation
}
