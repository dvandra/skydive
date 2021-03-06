/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package openflow

import (
	"github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of14"
)

// OpenFlow14 implements the 1.4 OpenFlow protocol basic methods
var OpenFlow14 OpenFlow14Protocol

// OpenFlow14Protocol implements the basic methods for OpenFlow 1.4
type OpenFlow14Protocol struct {
}

// String returns the OpenFlow protocol version as a string
func (p OpenFlow14Protocol) String() string {
	return "OpenFlow 1.4"
}

// GetVersion returns the OpenFlow protocol wire version
func (p OpenFlow14Protocol) GetVersion() uint8 {
	return goloxi.VERSION_1_4
}

// NewHello returns a new hello message
func (p OpenFlow14Protocol) NewHello(versionBitmap uint32) goloxi.Message {
	msg := of14.NewHello()
	elem := of14.NewHelloElemVersionbitmap()
	elem.Length = 8
	bitmap := of14.NewUint32()
	bitmap.Value = versionBitmap
	elem.Bitmaps = append(elem.Bitmaps, bitmap)
	msg.Elements = append(msg.Elements, elem)
	return msg
}

// NewEchoRequest returns a new echo request message
func (p OpenFlow14Protocol) NewEchoRequest() goloxi.Message {
	return of14.NewEchoRequest()
}

// NewEchoReply returns a new echo reply message
func (p OpenFlow14Protocol) NewEchoReply() goloxi.Message {
	return of14.NewEchoReply()
}

// NewBarrierRequest returns a new barrier request message
func (p OpenFlow14Protocol) NewBarrierRequest() goloxi.Message {
	return of14.NewBarrierRequest()
}

// DecodeMessage parses an OpenFlow message
func (p OpenFlow14Protocol) DecodeMessage(data []byte) (goloxi.Message, error) {
	return of14.DecodeMessage(data)
}
