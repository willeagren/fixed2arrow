/*
 * MIT No Attribution
 *
 * Copyright 2021 Rickard Lundin (rickard@ignalina.dk)
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this
 * software and associated documentation files (the "Software"), to deal in the Software
 * without restriction, including without limitation the rights to use, copy, modify,
 * merge, publish, distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
 * INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A
 * PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
 * HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
 * OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package impl

import (
	"github.com/apache/arrow/go/v9/arrow/array"
	"strconv"
)

type ColumnBuilderInt8 struct {
	fixedField    *FixedField
	recordBuilder *array.RecordBuilder
	fieldnr       int
	values        []int8
	valid         []bool
}

func (c *ColumnBuilderInt8) ParseValue(name string) bool {
	u, err := strconv.ParseInt(name, 10, 8)

	if nil != err {
		c.Nullify()
		return false
	}

	c.values = append(c.values, int8(u))
	c.valid = append(c.valid, true)

	return true
}

func (c *ColumnBuilderInt8) FinishColumn() bool {
	c.recordBuilder.Field(c.fieldnr).(*array.Int8Builder).AppendValues(c.values, c.valid)

	return true
}

func (c *ColumnBuilderInt8) Nullify() {
	c.values = append(c.values, 0)
	c.valid = append(c.valid, false)
}
