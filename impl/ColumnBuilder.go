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
	"bufio"
	"bytes"
	"fmt"
	"github.com/apache/arrow/go/v7/arrow"
	"github.com/apache/arrow/go/v7/arrow/array"
	"github.com/apache/arrow/go/v7/arrow/ipc"
	"github.com/apache/arrow/go/v7/arrow/memory"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
	"golang.org/x/xerrors"
	"io"
	"os"
	"strconv"
	"sync"
	"time"
)

type FixedField struct {
	Len   int
	Field arrow.Field
}

type FixedRow struct {
	FixedField []FixedField
}

type FixedSizeTableChunk struct {
	fixedSizeTable *FixedSizeTable
	columnBuilders []ColumnBuilder
	recordBuilder  *array.RecordBuilder
	record         array.Record
	bytes          []byte
}

type FixedSizeTable struct {
	// pointer to bytebuffer
	bytes       []byte
	TableChunks []FixedSizeTableChunk
	row         *FixedRow
	mem         *memory.GoAllocator
	schema      *arrow.Schema
	wg          *sync.WaitGroup
	records     []array.Record
}

func (f FixedRow) CalRowLength() int {
	sum := 0

	for _, num := range f.FixedField {
		sum += num.Len
	}
	return sum + 2
}

func (f *FixedSizeTableChunk) createColumBuilders() bool {
	f.columnBuilders = make([]ColumnBuilder, len(f.fixedSizeTable.row.FixedField))

	f.recordBuilder = array.NewRecordBuilder(f.fixedSizeTable.mem, f.fixedSizeTable.schema)
	//	defer b.Release()

	for i, ff := range f.fixedSizeTable.row.FixedField {
		f.columnBuilders[i] = *CreateColumBuilder(&ff, f.recordBuilder, ff.Len)
	}
	return true
}

func SaveFeather(w *os.File, fst *FixedSizeTable) error {
	mem := memory.NewGoAllocator()

	tbl := array.NewTableFromRecords(fst.schema, fst.records)
	rr := array.NewTableReader(tbl, 1010000)

	ww, err := ipc.NewFileWriter(w, ipc.WithAllocator(mem), ipc.WithSchema(rr.Schema()))
	if err != nil {
		return xerrors.Errorf("could not create ARROW file writer: %w", err)
	}

	defer ww.Close()

	return nil
}

// Read chunks of file and process them in go route after each chunk read. Slow disk is non non zero disk like sans etc
func CreateFixedSizeTableFromSlowDisk2(row *FixedRow, fileName string, cores int) (*FixedSizeTable, error) {
	var fst FixedSizeTable
	fst.row = row
	fst.mem = memory.NewGoAllocator()
	fst.schema = createSchemaFromFixedRow(row)

	fst.wg = &sync.WaitGroup{}
	ParalizeChunks(&fst, fileName, cores)

	//	defer tbl.Release()

	return &fst, nil
}

func createSchemaFromFixedRow(row *FixedRow) *arrow.Schema {
	var fields []arrow.Field
	fields = make([]arrow.Field, len(row.FixedField))

	for index, element := range row.FixedField {
		fields[index] = element.Field
	}
	return arrow.NewSchema(fields, nil)
}

//  unsigned char glyph=(unsigned char)195;
func findLastNL(bytes []byte) int {
	p2 := len(bytes)
	if 0 == p2 {
		return -1
	}

	for p2 > 0 {
		if bytes[p2-1] == 0x0d && bytes[p2] == 0x0a {
			return p2 + 1
		}
		p2--
	}

	return 0
}

type ColumnBuilder interface {
	ParseValue(name string) bool
	FinishColumn() bool
}

func CreateColumBuilder(fixedField *FixedField, builder *array.RecordBuilder, columnsize int) *ColumnBuilder {
	var result ColumnBuilder
	columnsize = 0
	columnsizeCap := 3000000

	switch fixedField.Field.Type.ID() {
	//	case types.String.ID():
	case arrow.BinaryTypes.String.ID():
		result = &ColumnBuilderString{fixedField: fixedField, recordBuilder: builder, values: make([]string, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Date32.ID():
		result = &ColumnBuilderDate32{fixedField: fixedField, recordBuilder: builder, values: make([]arrow.Date32, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Date64.ID():
		result = &ColumnBuilderDate64{fixedField: fixedField, recordBuilder: builder, values: make([]arrow.Date64, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Int8.ID():
		result = &ColumnBuilderInt8{fixedField: fixedField, recordBuilder: builder, values: make([]int8, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Int16.ID():
		result = &ColumnBuilderInt16{fixedField: fixedField, recordBuilder: builder, values: make([]int16, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Int32.ID():
		result = &ColumnBuilderInt32{fixedField: fixedField, recordBuilder: builder, values: make([]int32, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Int64.ID():
		result = &ColumnBuilderInt64{fixedField: fixedField, recordBuilder: builder, values: make([]int64, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Uint8.ID():
		result = &ColumnBuilderUint8{fixedField: fixedField, recordBuilder: builder, values: make([]uint8, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Uint16.ID():
		result = &ColumnBuilderUint16{fixedField: fixedField, recordBuilder: builder, values: make([]uint16, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Uint32.ID():
		result = &ColumnBuilderUint32{fixedField: fixedField, recordBuilder: builder, values: make([]uint32, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Uint64.ID():
		result = &ColumnBuilderUint64{fixedField: fixedField, recordBuilder: builder, values: make([]uint64, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Float32.ID():
		result = &ColumnBuilderFloat32{fixedField: fixedField, recordBuilder: builder, values: make([]float32, columnsize, columnsizeCap)}

	case arrow.PrimitiveTypes.Float64.ID():
		result = &ColumnBuilderFloat64{fixedField: fixedField, recordBuilder: builder, values: make([]float64, columnsize, columnsizeCap)}

	case arrow.FixedWidthTypes.Boolean.ID():
		result = &ColumnBuilderBoolean{fixedField: fixedField, recordBuilder: builder, values: make([]bool, columnsize, columnsizeCap)}

	}

	return &result
}

func ParalizeChunks(fst *FixedSizeTable, filename string, core int) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	fi, _ := file.Stat()

	fst.bytes = make([]byte, fi.Size())
	fst.TableChunks = make([]FixedSizeTableChunk, core)

	chunkSize := fi.Size() / int64(core)
	rowlength := int64(fst.row.CalRowLength())

	if chunkSize < int64(rowlength) {
		chunkSize = int64(rowlength)
	}

	goon := true
	chunkNr := 0
	p1 := 0
	p2 := 0

	for goon {

		fst.TableChunks[chunkNr] = FixedSizeTableChunk{fixedSizeTable: fst}
		fst.TableChunks[chunkNr].createColumBuilders()

		i1 := int(chunkSize) * chunkNr
		i2 := int(chunkSize) * (chunkNr + 1)
		if chunkNr == (core - 1) {
			i2 = len(fst.bytes)
		}
		buf := fst.bytes[i1:i2]
		nread, _ := io.ReadFull(file, buf)
		buf = buf[:nread]
		goon = i2 < len(fst.bytes)
		p2 = i1 + findLastNL(buf)

		fst.TableChunks[chunkNr].bytes = fst.bytes[p1:p2]
		p1 = p2
		fst.wg.Add(1)
		go fst.TableChunks[chunkNr].process()
		fst.TableChunks[chunkNr].record = fst.TableChunks[chunkNr].recordBuilder.NewRecord()

		chunkNr++
	}
	fst.wg.Wait()

	//	var r []array.Record=make([]array.Record, len(fst.TableChunks))
	fst.records = make([]array.Record, len(fst.TableChunks))

	for i, num := range fst.TableChunks {
		fst.records[i] = num.record
	}

	return nil
}

//func processChunk(gobuf []byte,wg *sync.WaitGroup) {
func (fstc FixedSizeTableChunk) process() int {

	defer fstc.fixedSizeTable.wg.Done()
	re := bytes.NewReader(fstc.bytes)
	decodingReader := transform.NewReader(re, charmap.ISO8859_1.NewDecoder()) //   lines := []string{}
	//	lines := make([]string, 0, 8000000)

	scanner := bufio.NewScanner(decodingReader)
	lineCnt := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line[:12] == "************" {
			fmt.Println("skipping footer")
			break
		}
		lineCnt++

		//		lines = append(lines, line)
		var columnPos int
		for ci, cc := range fstc.fixedSizeTable.row.FixedField {
			columString := line[columnPos : columnPos+cc.Len]
			fstc.columnBuilders[ci].ParseValue(columString)
			columnPos += cc.Len
		}

	}
	return lineCnt

}

var lo = &time.Location{}

// 2020-07-09-09.59.59.99375
func DateStringT1ToUnix(dateString string) (error, int64) {

	var year64, month64, day64, hour64, minute64, second64 int64
	var err error

	year64, err = strconv.ParseInt(dateString[:4], 10, 32)

	if nil != err {
		return err, 0
	}

	month64, err = strconv.ParseInt(dateString[5:7], 10, 8)

	if nil != err {
		return err, 0
	}

	day64, err = strconv.ParseInt(dateString[8:10], 10, 8)
	if nil != err {
		return err, 0
	}

	hour64, err = strconv.ParseInt(dateString[11:13], 10, 8)
	if nil != err {
		return err, 0
	}

	minute64, err = strconv.ParseInt(dateString[14:16], 10, 8)
	if nil != err {
		return err, 0
	}

	second64, err = strconv.ParseInt(dateString[17:19], 10, 8)
	if nil != err {
		return err, 0
	}

	var ti time.Time

	ti = time.Date(int(year64), time.Month(month64), int(day64), int(hour64), int(minute64), int(second64), 0, lo)

	return nil, ti.Unix()

}

func IsError(err error) bool {
	if err != nil {
		fmt.Println(err.Error())
	}
	return (err != nil)
}
