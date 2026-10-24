package nbt

import "errors"
import "fmt"

import "go-mc-server/internal/protocol"

const TagLong byte = 4
const TagFloat byte = 5
const TagDouble byte = 6
const TagByteArray byte = 7
const TagList byte = 9
const TagIntArray byte = 11

// 客户端发来的 NBT 不可信，读的时候设两道上限：嵌套最多几层、标签一共最多几个。
// 否则一个精心构造的小包就能让服务器递归爆栈，或者在一个"一百万个空标签"的列表上空转。
const maxReadDepth int = 8
const maxReadTags int = 2048

var errTooDeep error = errors.New("NBT 嵌套太深")
var errTooManyTags error = errors.New("NBT 标签太多")
var errBadLength error = errors.New("NBT 长度不合法")

// Compound 是从客户端 NBT 里读出来的顶层字符串和整数字段。
// 别的类型（短整型、列表、嵌套复合标签…）会被读过去但不保留，同名字段后出现的覆盖先出现的。
type Compound struct {
	Strings map[string]string
	Ints    map[string]int32
}

type reader struct {
	stream *protocol.BinaryStream
	budget int
}

// ReadNetworkCompound 读网络格式（长度和整数用变长编码）的根复合标签，格式照参考端 NBT::read(network=true)。
// 根标签后面多出来的字节忽略。
func ReadNetworkCompound(data []byte) (Compound, error) {
	var result Compound = Compound{Strings: map[string]string{}, Ints: map[string]int32{}}
	var source *reader = &reader{stream: protocol.NewStream(data), budget: maxReadTags}
	var kind, kindErr = source.stream.ReadByte()
	if kindErr != nil {
		return Compound{}, fmt.Errorf("读根标签类型: %w", kindErr)
	}
	if kind != TagCompound {
		return Compound{}, fmt.Errorf("根标签应是复合标签，实际类型 %d", kind)
	}
	var _, nameErr = source.stream.ReadString()
	if nameErr != nil {
		return Compound{}, fmt.Errorf("读根标签名: %w", nameErr)
	}
	var bodyErr error = source.readBody(1, &result)
	if bodyErr != nil {
		return Compound{}, fmt.Errorf("读复合标签: %w", bodyErr)
	}
	return result, nil
}

func (source *reader) spend() error {
	source.budget--
	if source.budget < 0 {
		return fmt.Errorf("数标签: %w", errTooManyTags)
	}
	return nil
}

// readBody 读复合标签的内容直到结束标记。result 为空表示这是嵌套的，里面的字段一个都不保留。
func (source *reader) readBody(depth int, result *Compound) error {
	if depth > maxReadDepth {
		return fmt.Errorf("读复合标签: %w", errTooDeep)
	}
	var count int = 0
	for count = 0; count <= maxReadTags; count++ {
		var kind, kindErr = source.stream.ReadByte()
		if kindErr != nil {
			return fmt.Errorf("读标签类型: %w", kindErr)
		}
		if kind == TagEnd {
			return nil
		}
		var spendErr error = source.spend()
		if spendErr != nil {
			return spendErr
		}
		var name, nameErr = source.stream.ReadString()
		if nameErr != nil {
			return fmt.Errorf("读标签名: %w", nameErr)
		}
		var entryErr error = source.readEntry(kind, name, depth, result)
		if entryErr != nil {
			return fmt.Errorf("读字段 %q: %w", name, entryErr)
		}
	}
	return errTooManyTags
}

func (source *reader) readEntry(kind byte, name string, depth int, result *Compound) error {
	if kind == TagString {
		var text, textErr = source.stream.ReadString()
		if textErr != nil {
			return fmt.Errorf("读字符串 %q: %w", name, textErr)
		}
		if result != nil {
			result.Strings[name] = text
		}
		return nil
	}
	if kind == TagInt {
		var number, numberErr = source.stream.ReadVarInt()
		if numberErr != nil {
			return fmt.Errorf("读整数 %q: %w", name, numberErr)
		}
		if result != nil {
			result.Ints[name] = number
		}
		return nil
	}
	var skipErr error = source.skip(kind, depth)
	if skipErr != nil {
		return fmt.Errorf("跳过标签 %q: %w", name, skipErr)
	}
	return nil
}

func (source *reader) skipBytes(count int) error {
	var _, err = source.stream.ReadBytes(count)
	if err != nil {
		return fmt.Errorf("跳过 %d 字节: %w", count, err)
	}
	return nil
}

// skipArray 数组长度是变长整数，后面跟 length×width 字节。长度不能是负数，也不能超过剩下的字节数。
func (source *reader) skipArray(width int) error {
	var length, err = source.stream.ReadVarInt()
	if err != nil {
		return fmt.Errorf("读数组长度: %w", err)
	}
	if length < 0 || int(length) > source.stream.Remaining()/width {
		return fmt.Errorf("读数组长度 %d: %w", length, errBadLength)
	}
	return source.skipBytes(int(length) * width)
}

func (source *reader) skipList(depth int) error {
	if depth+1 > maxReadDepth {
		return fmt.Errorf("读列表: %w", errTooDeep)
	}
	var kind, kindErr = source.stream.ReadByte()
	if kindErr != nil {
		return fmt.Errorf("读列表元素类型: %w", kindErr)
	}
	var size, sizeErr = source.stream.ReadVarInt()
	if sizeErr != nil {
		return fmt.Errorf("读列表长度: %w", sizeErr)
	}
	if size < 0 {
		return fmt.Errorf("读列表长度 %d: %w", size, errBadLength)
	}
	var index int32 = 0
	for index = 0; index < size; index++ {
		var spendErr error = source.spend()
		if spendErr != nil {
			return fmt.Errorf("读列表元素: %w", spendErr)
		}
		if kind == TagEnd {
			continue
		}
		var elementErr error = source.skip(kind, depth+1)
		if elementErr != nil {
			return fmt.Errorf("读列表第 %d 个元素: %w", index, elementErr)
		}
	}
	return nil
}

// skip 跳过一个标签的内容（类型和名字已经读过了）。
func (source *reader) skip(kind byte, depth int) error {
	switch kind {
	case TagByte:
		return source.skipBytes(1)
	case TagShort:
		return source.skipBytes(2)
	case TagFloat:
		return source.skipBytes(4)
	case TagDouble:
		return source.skipBytes(8)
	case TagInt:
		var _, numberErr = source.stream.ReadVarInt()
		if numberErr != nil {
			return fmt.Errorf("跳过整数: %w", numberErr)
		}
		return nil
	case TagLong:
		var _, numberErr = source.stream.ReadVarLong()
		if numberErr != nil {
			return fmt.Errorf("跳过长整数: %w", numberErr)
		}
		return nil
	case TagString:
		var _, textErr = source.stream.ReadString()
		if textErr != nil {
			return fmt.Errorf("跳过字符串: %w", textErr)
		}
		return nil
	case TagByteArray:
		return source.skipArray(1)
	case TagIntArray:
		return source.skipArray(4)
	case TagList:
		return source.skipList(depth)
	case TagCompound:
		return source.readBody(depth+1, nil)
	}
	return fmt.Errorf("不认识的 NBT 标签类型 %d", kind)
}
