package storage

import "bytes"
import "compress/gzip"
import "compress/zlib"
import "encoding/binary"
import "fmt"
import "io"

// 地图文件是大端 NBT；不能用游戏包里的 VarInt 或物品栈的小端 NBT 读取。
type pmTag struct {
	kind    byte
	raw     []byte
	fields  map[string]pmTag
	names   []string
	element byte
	list    []pmTag
}

type pmReader struct {
	data   []byte
	budget int
}

func (reader *pmReader) take(size int) ([]byte, error) {
	if size < 0 || size > len(reader.data) {
		return nil, io.ErrUnexpectedEOF
	}
	var value = reader.data[:size]
	reader.data = reader.data[size:]
	return value, nil
}

func (reader *pmReader) text() ([]byte, error) {
	var length, err = reader.take(2)
	if err != nil {
		return nil, err
	}
	return reader.take(int(binary.BigEndian.Uint16(length)))
}

func (reader *pmReader) tag(kind byte, depth int) (pmTag, error) {
	var result = pmTag{kind: kind}
	reader.budget--
	if depth > 64 || reader.budget < 0 {
		return result, fmt.Errorf("地图 NBT 层数或条目数超过限制")
	}
	var err error = nil
	switch kind {
	case 1, 2, 3, 4, 5, 6:
		var widths = [...]int{0, 1, 2, 4, 8, 4, 8}
		result.raw, err = reader.take(widths[kind])
	case 8:
		result.raw, err = reader.text()
	case 7, 11, 12:
		var length, readErr = reader.take(4)
		if readErr != nil {
			return result, readErr
		}
		var width = 1
		if kind == 11 {
			width = 4
		} else if kind == 12 {
			width = 8
		}
		var size = int64(int32(binary.BigEndian.Uint32(length))) * int64(width)
		if size < 0 || size > int64(len(reader.data)) {
			return result, fmt.Errorf("NBT 数组长度无效: %d", size)
		}
		result.raw, err = reader.take(int(size))
	case 9:
		var header, readErr = reader.take(5)
		if readErr != nil {
			return result, readErr
		}
		var count = int(int32(binary.BigEndian.Uint32(header[1:])))
		if count < 0 || count > reader.budget || count > len(reader.data) {
			return result, fmt.Errorf("NBT 列表长度无效: %d", count)
		}
		result.element = header[0]
		result.list = make([]pmTag, count)
		var index = 0
		for index = 0; index < count; index++ {
			result.list[index], err = reader.tag(header[0], depth+1)
			if err != nil {
				return result, err
			}
		}
	case 10:
		result.fields = make(map[string]pmTag)
		for {
			var child, readErr = reader.take(1)
			if readErr != nil {
				return result, readErr
			}
			if child[0] == 0 {
				break
			}
			var name, nameErr = reader.text()
			if nameErr != nil {
				return result, nameErr
			}
			var key = string(name)
			if result.fields[key].kind != 0 {
				return result, fmt.Errorf("地图 NBT 字段重复: %s", key)
			}
			result.names = append(result.names, key)
			result.fields[key], err = reader.tag(child[0], depth+1)
			if err != nil {
				return result, err
			}
		}
	default:
		return result, fmt.Errorf("未知地图 NBT 类型: %d", kind)
	}
	return result, err
}

func (tag pmTag) integer(name string, kind byte) (int64, error) {
	var value = tag.fields[name]
	if value.kind != kind {
		return 0, fmt.Errorf("字段 %s 需要 NBT 类型 %d，实际 %d", name, kind, value.kind)
	}
	switch kind {
	case 1:
		return int64(int8(value.raw[0])), nil
	case 2:
		return int64(int16(binary.BigEndian.Uint16(value.raw))), nil
	case 3:
		return int64(int32(binary.BigEndian.Uint32(value.raw))), nil
	case 4:
		return int64(binary.BigEndian.Uint64(value.raw)), nil
	}
	return 0, fmt.Errorf("不是整数类型: %d", kind)
}

func readPMRoot(input io.Reader, compression byte, field string) (pmTag, error) {
	var stream io.ReadCloser = nil
	var err error = nil
	switch compression {
	case 1:
		stream, err = gzip.NewReader(input)
	case 2:
		stream, err = zlib.NewReader(input)
	default:
		return pmTag{}, fmt.Errorf("未知地图压缩类型: %d", compression)
	}
	if err != nil {
		return pmTag{}, err
	}
	defer stream.Close()
	const maxNBT = 16 << 20
	var raw, readErr = io.ReadAll(io.LimitReader(stream, maxNBT+1))
	if readErr != nil {
		return pmTag{}, readErr
	}
	if len(raw) > maxNBT || !bytes.HasPrefix(raw, []byte{10}) {
		return pmTag{}, fmt.Errorf("NBT 根标签无效或超过 16 MiB")
	}
	var reader = pmReader{data: raw[1:], budget: 1 << 18}
	if _, err = reader.text(); err != nil {
		return pmTag{}, err
	}
	var root, rootErr = reader.tag(10, 0)
	if rootErr != nil {
		return pmTag{}, rootErr
	}
	var value = root.fields[field]
	if value.kind != 10 || len(reader.data) != 0 {
		return pmTag{}, fmt.Errorf("缺少根复合字段 %s 或结尾有多余数据", field)
	}
	return value, nil
}
