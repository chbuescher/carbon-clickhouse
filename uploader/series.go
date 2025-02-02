package uploader

import (
	"bytes"
	"io"
	"strconv"
	"time"

	"github.com/lomik/carbon-clickhouse/helper/RowBinary"
)

type Series struct {
	*cached
	isReverse bool
}

var _ Uploader = &Series{}
var _ UploaderWithReset = &Series{}

func NewSeries(base *Base, reverse bool) *Series {
	u := &Series{}
	u.cached = newCached(base)
	u.cached.parser = u.parseFile
	u.isReverse = reverse
	return u
}

func (u *Series) parseFile(filename string, out io.Writer) (uint64, map[string]bool, error) {
	var reader *RowBinary.Reader
	var err error
	var n uint64

	reader, err = RowBinary.NewReader(filename, u.isReverse)
	if err != nil {
		return n, nil, err
	}
	defer reader.Close()

	version := uint32(time.Now().Unix())
	version = version - (version % 60)
	newSeries := make(map[string]bool)
	wb := RowBinary.GetWriteBuffer()

	var level int

LineLoop:
	for {
		name, err := reader.ReadRecord()
		if err != nil { // io.EOF or corrupted file
			break
		}

		// skip tagged
		if bytes.IndexByte(name, '?') >= 0 {
			continue
		}

		key := strconv.Itoa(int(reader.Days())) + ":" + unsafeString(name)

		if u.existsCache.Exists(key) {
			continue LineLoop
		}

		if newSeries[key] {
			continue LineLoop
		}
		n++

		level = pathLevel(name)

		wb.Reset()

		newSeries[key] = true
		wb.WriteUint16(reader.Days())
		wb.WriteUint32(uint32(level))
		wb.WriteBytes(name)
		wb.WriteUint32(version)

		_, err = out.Write(wb.Bytes())
		if err != nil {
			return n, nil, err
		}
	}

	wb.Release()

	return n, newSeries, nil
}
