package nbt

import "errors"
import "testing"

import "go-mc-server/internal/protocol"

// tagWriter 拼客户端发上来的网络格式 NBT，名字长度和整数都用变长编码。
type tagWriter struct {
	stream *protocol.BinaryStream
}

func newTagWriter() *tagWriter {
	return &tagWriter{stream: protocol.NewWriter()}
}

func (writer *tagWriter) data() []byte {
	return writer.stream.Bytes()
}

func (writer *tagWriter) root() {
	writer.byte(TagCompound)
	writer.text("")
}

func (writer *tagWriter) end() {
	writer.byte(TagEnd)
}

// WriteByte 的 error 恒为 nil，用 WriteBytes 免得写一段没人看的错误处理。
func (writer *tagWriter) byte(value byte) {
	writer.stream.WriteBytes([]byte{value})
}

func (writer *tagWriter) text(value string) {
	writer.stream.WriteString(value)
}

// 数组和列表的长度和整数一样是 zigzag 变长整数，不是无符号的。
func (writer *tagWriter) count(value int32) {
	writer.stream.WriteVarInt(value)
}

func (writer *tagWriter) textTag(name string, value string) {
	writer.byte(TagString)
	writer.text(name)
	writer.text(value)
}

func (writer *tagWriter) intTag(name string, value int32) {
	writer.byte(TagInt)
	writer.text(name)
	writer.stream.WriteVarInt(value)
}

// emptyTag 写一个名字叫 name、内容为空的指定类型标签，用来验证不认识的类型能被干净地跳过去。
func (writer *tagWriter) emptyTag(kind byte, name string) {
	writer.byte(kind)
	writer.text(name)
	switch kind {
	case TagByte:
		writer.byte(0)
	case TagShort:
		writer.stream.WriteInt16LE(0)
	case TagFloat:
		writer.stream.WriteLFloat(0)
	case TagDouble:
		writer.stream.WriteInt64LE(0)
	case TagLong:
		writer.stream.WriteVarLong(0)
	}
}

// 能读出字符串和整数，其他类型（短整型、浮点、数组、列表、嵌套复合标签）读过去但不保留。
func TestReadNetworkCompoundKeepsStringAndInt(test *testing.T) {
	var writer *tagWriter = newTagWriter()
	writer.root()
	writer.emptyTag(TagByte, "byte")
	writer.emptyTag(TagShort, "short")
	writer.emptyTag(TagFloat, "float")
	writer.emptyTag(TagDouble, "double")
	writer.emptyTag(TagLong, "long")
	writer.byte(TagByteArray)
	writer.text("bytes")
	writer.count(3)
	writer.stream.WriteBytes([]byte{1, 2, 3})
	writer.byte(TagIntArray)
	writer.text("ints")
	writer.count(2)
	writer.stream.WriteBytes(make([]byte, 8))
	writer.byte(TagList)
	writer.text("list")
	writer.byte(TagString)
	writer.count(2)
	writer.text("甲")
	writer.text("乙")
	writer.byte(TagCompound)
	writer.text("nested")
	writer.textTag("id", "不该被保留")
	writer.end()
	writer.textTag("id", IDSign)
	writer.intTag("x", -7)
	writer.end()
	var compound, err = ReadNetworkCompound(writer.data())
	if err != nil {
		test.Fatalf("应能读出来，实际报错 %v", err)
	}
	if compound.Strings["id"] != IDSign {
		test.Fatalf("期望 id 是 %q，实际 %q", IDSign, compound.Strings["id"])
	}
	if compound.Ints["x"] != -7 {
		test.Fatalf("期望 x 是 -7，实际 %d", compound.Ints["x"])
	}
	if len(compound.Strings) != 1 || len(compound.Ints) != 1 {
		test.Fatalf("只该留下顶层两个字段，实际 字符串 %v 整数 %v", compound.Strings, compound.Ints)
	}
}

// 同名字段后出现的覆盖先出现的。
func TestReadNetworkCompoundLaterWins(test *testing.T) {
	var writer *tagWriter = newTagWriter()
	writer.root()
	writer.textTag("id", "旧")
	writer.textTag("id", "新")
	writer.intTag("x", 1)
	writer.intTag("x", 2)
	writer.end()
	var compound, err = ReadNetworkCompound(writer.data())
	if err != nil {
		test.Fatalf("应能读出来，实际报错 %v", err)
	}
	if compound.Strings["id"] != "新" || compound.Ints["x"] != 2 {
		test.Fatalf("期望 新/2，实际 %q/%d", compound.Strings["id"], compound.Ints["x"])
	}
}

// 根标签后面多出来的字节不管，客户端有时会多塞东西。
func TestReadNetworkCompoundIgnoresTrailingBytes(test *testing.T) {
	var writer *tagWriter = newTagWriter()
	writer.root()
	writer.textTag("id", IDSign)
	writer.end()
	var compound, err = ReadNetworkCompound(append(writer.data(), 0xde, 0xad))
	if err != nil {
		test.Fatalf("多余字节不该报错，实际 %v", err)
	}
	if compound.Strings["id"] != IDSign {
		test.Fatalf("期望 id 是 %q，实际 %q", IDSign, compound.Strings["id"])
	}
}

type readErrorCase struct {
	Name  string
	Build func() []byte
	Wants error
}

// 畸形数据要么报具体错误，要么至少报错，不能悄悄读出一个空结果。
func TestReadNetworkCompoundRejectsBadData(test *testing.T) {
	var cases []readErrorCase = []readErrorCase{
		{
			Name: "一个字节都没有",
			Build: func() []byte {
				return nil
			},
		},
		{
			Name: "根标签不是复合标签",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.byte(TagString)
				writer.text("")
				return writer.data()
			},
		},
		{
			Name: "只有根标签类型没有名字",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.byte(TagCompound)
				return writer.data()
			},
		},
		{
			Name: "字段内容被截断",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				writer.byte(TagString)
				writer.text("id")
				writer.count(8)
				writer.stream.WriteBytes([]byte("abc"))
				return writer.data()
			},
		},
		{
			Name: "标签数量超上限",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				writer.byte(TagList)
				writer.text("list")
				writer.byte(TagByte)
				writer.count(int32(maxReadTags + 1))
				var index int = 0
				for index = 0; index < maxReadTags+1; index++ {
					writer.byte(0)
				}
				writer.end()
				return writer.data()
			},
			Wants: errTooManyTags,
		},
		{
			Name: "嵌套层数超上限",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				var index int = 0
				for index = 0; index < maxReadDepth+1; index++ {
					writer.byte(TagCompound)
					writer.text("nested")
				}
				writer.end()
				writer.end()
				return writer.data()
			},
			Wants: errTooDeep,
		},
		{
			// 一层套一层的列表：每次只往下走一层，复合标签那条深度检查根本碰不到，
			// 只有 skipList 自己的检查能拦住。
			Name: "纯列表嵌套层数超上限",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				// 列表元素不带名字和类型字节，只有最外层的列表有名字。
				writer.byte(TagList)
				writer.text("nested")
				var index int = 0
				for index = 0; index < maxReadDepth+1; index++ {
					writer.byte(TagList)
					writer.count(1)
				}
				writer.byte(TagByte)
				writer.count(0)
				writer.end()
				return writer.data()
			},
			Wants: errTooDeep,
		},
		{
			Name: "字节数组长度超过剩下的字节",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				writer.byte(TagByteArray)
				writer.text("bytes")
				writer.count(10)
				writer.stream.WriteBytes([]byte{1, 2})
				writer.end()
				return writer.data()
			},
			Wants: errBadLength,
		},
		{
			Name: "整数数组长度是负数",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				writer.byte(TagIntArray)
				writer.text("ints")
				writer.stream.WriteVarInt(-1)
				writer.end()
				return writer.data()
			},
			Wants: errBadLength,
		},
		{
			Name: "列表长度是负数",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				writer.byte(TagList)
				writer.text("list")
				writer.byte(TagByte)
				writer.stream.WriteVarInt(-1)
				writer.end()
				return writer.data()
			},
			Wants: errBadLength,
		},
		{
			Name: "不认识的标签类型",
			Build: func() []byte {
				var writer *tagWriter = newTagWriter()
				writer.root()
				writer.byte(12)
				writer.text("unknown")
				writer.end()
				return writer.data()
			},
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item readErrorCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var _, err = ReadNetworkCompound(item.Build())
			if err == nil {
				sub.Fatal("期望报错，实际读成功了")
			}
			if item.Wants != nil && !errors.Is(err, item.Wants) {
				sub.Fatalf("期望错误 %v，实际 %v", item.Wants, err)
			}
		})
	}
}
