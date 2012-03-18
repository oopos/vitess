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
	"code.google.com/p/vitess/go/vt/sqlparser"
	"encoding/base64"
	"fmt"
	"strconv"
)

func buildValueList(pkValues []interface{}, bindVars map[string]interface{}) [][]interface{} {
	length := -1
	for _, pkValue := range pkValues {
		if list, ok := pkValue.([]interface{}); ok {
			if length == -1 {
				if length = len(list); length == 0 {
					panic(NewTabletError(FAIL, "empty list for values %v", pkValues))
				}
			} else if length != len(list) {
				panic(NewTabletError(FAIL, "mismatched lengths for values %v", pkValues))
			}
		}
	}
	if length == -1 {
		length = 1
	}
	valueList := make([][]interface{}, length)
	for i := 0; i < length; i++ {
		valueList[i] = make([]interface{}, len(pkValues))
		for j, pkValue := range pkValues {
			if list, ok := pkValue.([]interface{}); ok {
				valueList[i][j] = resolveValue(list[i], bindVars)
			} else {
				valueList[i][j] = resolveValue(pkValue, bindVars)
			}
		}
	}
	return valueList
}

func buildSecondaryList(pkList [][]interface{}, secondaryList []interface{}, bindVars map[string]interface{}) [][]interface{} {
	if secondaryList == nil {
		return nil
	}
	valueList := make([][]interface{}, len(pkList))
	for i, row := range pkList {
		valueList[i] = make([]interface{}, len(row))
		for j, cell := range row {
			if secondaryList[j] == nil {
				valueList[i][j] = cell
			} else {
				valueList[i][j] = resolveValue(secondaryList[j], bindVars)
			}
		}
	}
	return valueList
}

func resolveValue(value interface{}, bindVars map[string]interface{}) interface{} {
	if v, ok := value.(string); ok {
		if v[0] == ':' {
			lookup, ok := bindVars[v[1:]]
			if !ok {
				panic(NewTabletError(FAIL, "No bind var found for %s", v))
			}
			return lookup
		}
	}
	return value
}

func normalizePKRows(tableInfo *TableInfo, pkRows [][]interface{}) {
	normalizeRows(tableInfo, tableInfo.PKColumns, pkRows)
}

func normalizeRows(tableInfo *TableInfo, columnNumbers []int, rows [][]interface{}) {
	for _, row := range rows {
		if len(row) != len(columnNumbers) {
			panic(NewTabletError(FAIL, "data inconsistency %d vs %d", len(row), len(columnNumbers)))
		}
		for j, cell := range row {
			if tableInfo.ColumnIsNumber[columnNumbers[j]] {
				switch val := cell.(type) {
				case string:
					row[j] = tonumber(val)
				case []byte:
					row[j] = tonumber(string(val))
				}
			}
		}
	}
}

func buildKey(tableInfo *TableInfo, row []interface{}) (key string) {
	buf := bytes.NewBuffer(make([]byte, 0, 32))
	for i, pkValue := range row {
		encodePKValue(buf, pkValue, tableInfo.ColumnIsNumber[tableInfo.PKColumns[i]])
		buf.WriteByte(',')
	}
	return buf.String()
}

func buildStreamComment(tableInfo *TableInfo, pkValueList [][]interface{}, secondaryList [][]interface{}) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	fmt.Fprintf(buf, " /* _stream %s (", tableInfo.Name)
	// We assume the first index exists, and is the pk
	for i, pkName := range tableInfo.Indexes[0].Columns {
		// Skip column if its value is nil
		if pkValueList[0][i] == nil {
			continue
		}
		buf.WriteString(pkName)
		buf.WriteString(" ")
	}
	buf.WriteString(")")
	buildPKValueList(buf, tableInfo, pkValueList)
	buildPKValueList(buf, tableInfo, secondaryList)
	buf.WriteString("; */")
	return buf.Bytes()
}

func buildPKValueList(buf *bytes.Buffer, tableInfo *TableInfo, pkValueList [][]interface{}) {
	for _, pkValues := range pkValueList {
		buf.WriteString(" (")
		for j, pkValue := range pkValues {
			if pkValue == nil {
				continue
			}
			encodePKValue(buf, pkValue, tableInfo.ColumnIsNumber[tableInfo.PKColumns[j]])
			buf.WriteString(" ")
		}
		buf.WriteString(")")
	}
}

func encodePKValue(buf *bytes.Buffer, pkValue interface{}, isNumber bool) {
	if isNumber {
		switch val := pkValue.(type) {
		case int, int32, int64, uint, uint32, uint64:
			sqlparser.EncodeValue(buf, val)
		case string:
			sqlparser.EncodeValue(buf, tonumber(val))
		case []byte:
			sqlparser.EncodeValue(buf, tonumber(string(val)))
		default:
			panic(NewTabletError(FAIL, "Type %T disallowed for pk columns", val))
		}
	} else {
		buf.WriteString("'")
		switch val := pkValue.(type) {
		case int, int32, int64, uint, uint32, uint64:
			sqlparser.EncodeValue(buf, val)
		case string:
			for i := 0; i < len(val); i++ {
				escapeWrite(buf, val[i])
			}
		case []byte:
			for i := 0; i < len(val); i++ {
				escapeWrite(buf, val[i])
			}
		default:
			panic(NewTabletError(FAIL, "Type %T disallowed for pk columns", val))
		}
		buf.WriteString("'")
	}
}

func tonumber(val string) (number interface{}) {
	var err error
	if val[0] == '-' {
		number, err = strconv.ParseInt(val, 0, 64)
	} else {
		number, err = strconv.ParseUint(val, 0, 64)
	}
	if err != nil {
		panic(NewTabletError(FAIL, "%s", err))
	}
	return number
}

func escapeWrite(buf *bytes.Buffer, b byte) {
	switch b {
	case '\'':
		buf.WriteString("\\'")
	case '\\':
		buf.WriteString("\\\\")
	case '/':
		buf.WriteString("\\/")
	case '*':
		buf.WriteString("\\*")
	default:
		buf.WriteByte(b)
	}
}

func base64Decode(b []byte) string {
	decodedKey := make([]byte, base64.StdEncoding.DecodedLen(len(b)))
	if _, err := base64.StdEncoding.Decode(decodedKey, b); err != nil {
		panic(NewTabletError(FAIL, "%s", err))
	}
	return string(decodedKey)
}
