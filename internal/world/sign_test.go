package world_test

import "strings"
import "testing"

import "go-mc-server/internal/world"

type cleanCase struct {
	Name  string
	Input string
	Want  string
}

// 客户端写在告示牌上的字要洗干净：格式码、孤立 §、控制字符、私用区字符、非法 UTF-8 全部去掉，
// 只留下能正常显示的字符，长度也算洗过之后的。
func TestCleanSignLine(test *testing.T) {
	var cases []cleanCase = []cleanCase{
		{
			Name:  "普通文字原样保留",
			Input: "你好 world",
			Want:  "你好 world",
		},
		{
			Name:  "格式码连同后面的字母一起去掉",
			Input: "§a绿色§l§r",
			Want:  "绿色",
		},
		{
			Name:  "末尾孤立的 § 去掉",
			Input: "abc§",
			Want:  "abc",
		},
		{
			Name:  "§ 后面是大写字母不算格式码，只去掉 §",
			Input: "a§Ab",
			Want:  "aAb",
		},
		{
			Name:  "§ 后面是中文不算格式码，只去掉 §",
			Input: "a§你",
			Want:  "a你",
		},
		{
			Name:  "控制字符去掉，换行制表符零字符都算",
			Input: "a\nb\tc\r\000d",
			Want:  "abcd",
		},
		{
			Name:  "私用区字符去掉",
			Input: "abc",
			Want:  "abc",
		},
		{
			Name:  "非法 UTF-8 去掉",
			Input: "a\xffb",
			Want:  "ab",
		},
		{
			Name:  "超长截断",
			Input: strings.Repeat("字", world.SignLineMax+5),
			Want:  strings.Repeat("字", world.SignLineMax),
		},
		{
			Name:  "被删掉的字符不占长度",
			Input: strings.Repeat("a", world.SignLineMax-1) + "\n\n\nbc",
			Want:  strings.Repeat("a", world.SignLineMax-1) + "b",
		},
		{
			Name:  "格式码和控制字符去掉后只剩正文",
			Input: "§a§b\ne",
			Want:  "e",
		},
	}
	var index int = 0
	for index = 0; index < len(cases); index++ {
		var item cleanCase = cases[index] // 闭包里用循环变量前先拷贝
		test.Run(item.Name, func(sub *testing.T) {
			var got string = world.CleanSignLine(item.Input)
			if got != item.Want {
				sub.Fatalf("期望 %q，实际 %q", item.Want, got)
			}
		})
	}
}
