/*
Copyright 2012, Google Inc.
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

    * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
    * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,           
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY           
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package tabletserver

import (
	"bytes"
	"code.google.com/p/vitess/go/bson"
)

type Query struct {
	Sql           string
	BindVariables map[string]interface{}
	TransactionId int64
	ConnectionId  int64
	SessionId     int64
}

func (self *Query) MarshalBson(buf *bytes.Buffer) {
	lenWriter := bson.NewLenWriter(buf)

	bson.EncodePrefix(buf, bson.Binary, "Sql")
	bson.EncodeString(buf, self.Sql)

	bson.EncodePrefix(buf, bson.Object, "BindVariables")
	self.encodeBindVariablesBson(buf)

	bson.EncodePrefix(buf, bson.Long, "TransactionId")
	bson.EncodeUint64(buf, uint64(self.TransactionId))

	bson.EncodePrefix(buf, bson.Long, "ConnectionId")
	bson.EncodeUint64(buf, uint64(self.ConnectionId))

	bson.EncodePrefix(buf, bson.Long, "SessionId")
	bson.EncodeUint64(buf, uint64(self.SessionId))

	buf.WriteByte(0)
	lenWriter.RecordLen()
}

func (self *Query) encodeBindVariablesBson(buf *bytes.Buffer) {
	lenWriter := bson.NewLenWriter(buf)
	for k, v := range self.BindVariables {
		bson.EncodeField(buf, k, v)
	}
	buf.WriteByte(0)
	lenWriter.RecordLen()
}

func (self *Query) UnmarshalBson(buf *bytes.Buffer) {
	bson.Next(buf, 4)

	kind := bson.NextByte(buf)
	for kind != bson.EOO {
		key := bson.ReadCString(buf)
		switch key {
		case "Sql":
			self.Sql = bson.DecodeString(buf, kind)
		case "BindVariables":
			self.decodeBindVariablesBson(buf, kind)
		case "TransactionId":
			self.TransactionId = bson.DecodeInt64(buf, kind)
		case "ConnectionId":
			self.ConnectionId = bson.DecodeInt64(buf, kind)
		case "SessionId":
			self.SessionId = bson.DecodeInt64(buf, kind)
		default:
			panic(bson.NewBsonError("Unrecognized tag %s", key))
		}
		kind = bson.NextByte(buf)
	}
}

func (self *Query) decodeBindVariablesBson(buf *bytes.Buffer, kind byte) {
	switch kind {
	case bson.Object:
		if err := bson.UnmarshalFromStream(buf, &self.BindVariables); err != nil {
			panic(err)
		}
	case bson.Null:
		// no op
	default:
		panic(bson.NewBsonError("Unexpected data type %v for Query.BindVariables", kind))
	}
}
