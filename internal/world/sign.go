package world

import "strings"
import "unicode"

// SignLineMax 告示牌一行最多放多少个字。这是防滥用的上限，不是从客户端行为里量出来的。
const SignLineMax int = 32

const BlockSignPost byte = 63
const BlockWallSign byte = 68

// NewSignTile 一块空告示牌。editor 是放下它的玩家的 UUID，只有这个人能给它写字。
func NewSignTile(posX int32, posY int32, posZ int32, editor string) Tile {
	return Tile{ID: TileSign, X: posX, Y: posY, Z: posZ, Editor: editor}
}

// 格式码 § 后面跟这些字符才是格式码，和参考端 TextFormat::clean 一致，只认小写。
func isFormatCode(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'k' && char <= 'o') || char == 'r'
}

// CleanSignLine 清洗客户端写在告示牌一行上的字：去掉格式码和孤立的 §、控制字符（换行、制表符、
// 转义符都算）、私用区字符、非法 UTF-8，最后截到 SignLineMax 个字。
// 控制字符不留，免得一行字里夹着换行，把控制台日志和别的玩家的聊天栏搞乱。
func CleanSignLine(text string) string {
	var runes []rune = []rune(strings.ToValidUTF8(text, ""))
	var builder strings.Builder = strings.Builder{}
	var kept int = 0
	var index int = 0
	for index = 0; index < len(runes) && kept < SignLineMax; index++ {
		var char rune = runes[index]
		if char == '§' {
			if index+1 < len(runes) && isFormatCode(runes[index+1]) {
				index++
			}
			continue
		}
		if unicode.IsControl(char) || (char >= 0xE000 && char <= 0xF8FF) {
			continue
		}
		builder.WriteRune(char)
		kept++
	}
	return builder.String()
}
