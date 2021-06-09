// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"encoding/json"

	"golang.org/x/exp/event/keys"
)

var (
	Method       = keys.String("method")
	RPCDirection = keys.String("direction")
	RPCID        = keys.String("rpcID")
	Error        = keys.Value("error")
)

type Handler func(ctx context.Context, reply Replier, req Request) error

type Replier func(ctx context.Context, result interface{}, err error) error

type Message interface {
	isJSONRPC2Message()
}

type Request interface {
	Message
	// Method is a string containing the method name to invoke.
	Method() string
	// Params is either a struct or an array with the parameters of the method.
	Params() json.RawMessage
	// isJSONRPC2Request is used to make the set of request implementations closed.
	isJSONRPC2Request()
}

type ID struct {
	name   string
	number int64
}

type Response struct {
	result json.RawMessage
	err    error
	id     ID
}

func NewResponse(id ID, result interface{}, err error) (*Response, error) {
	return nil, nil
}

// func (msg *Response) ID() ID                  { return msg.id }
// func (msg *Response) Result() json.RawMessage { return msg.result }
// func (msg *Response) Err() error              { return msg.err }
func (msg *Response) isJSONRPC2Message() {}

type Notification struct {
	method string
	params json.RawMessage
}

func (Notification) isJSONRPC2Message() {}

type Stream interface {
	Read(context.Context) (Message, int64, error)
	Write(context.Context, Message) (int64, error)
	Close() error
}

func NewNotification(method string, params interface{}) (*Notification, error) { return nil, nil }

type Call struct {
	method string
	params json.RawMessage
	id     ID
}

func NewCall(id ID, method string, params interface{}) (*Call, error) {
	return nil, nil
}

func (msg *Call) Method() string { return msg.method }

func (msg *Call) Params() json.RawMessage { return msg.params }

func (msg *Call) ID() ID             { return msg.id }
func (msg *Call) isJSONRPC2Message() {}
func (msg *Call) isJSONRPC2Request() {}
