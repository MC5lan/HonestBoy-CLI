package tui

import "strings"

// 侧脸像素小男孩：10 像素宽、12 像素高 → 6 个字符行。
// 后脑头发偏多、脸朝右、只露一只眼；整体比球门矮。
const (
	pixHair  = 'H'
	pixSkin  = 'F'
	pixEye   = 'E'
	pixBlush = 'B'
	pixMouth = 'M'
	pixShirt = 'G'
)

var mascotPixel = []string{
	"..HHHHHH..", // 0 头发
	".HHHHHHFF.", // 1 头顶 + 右侧额头
	".HHHFFFFF.", // 2 脸
	".HHHFFFEF.", // 3 一只眼靠右（与上一行组成半块对比）
	".HHHFFFFF.", // 4 脸
	".HHHFFMMF.", // 5 嘴
	"...HFF....", // 6 下巴/脖子
	"..GGGGG...", // 7 肩
	".GGGGGGG..", // 8 身体
	".GGGGGGG..", // 9
	"..GG.GG...", // 10 双腿
	"..FF.FF...", // 11 双脚
}

var pixelPalette = map[rune]int{
	pixHair:  237,
	pixSkin:  223,
	pixEye:   16, // 纯黑，在肤色背景上清晰
	pixBlush: 210,
	pixMouth: 160,
	pixShirt: 30,
}

const (
	pixReset = "\x1b[0m"
	pixW     = 10
	pixH     = 12
)

func renderPixelMascot(color bool) []string {
	rows := make([]string, pixH/2)
	for y := 0; y < pixH; y += 2 {
		var b strings.Builder
		for x := 0; x < pixW; x++ {
			top := pixelAt(x, y)
			bottom := pixelAt(x, y+1)
			switch {
			case top == 0 && bottom == 0:
				b.WriteByte(' ')
			case top != 0 && bottom == 0:
				b.WriteString(pixelBlock(top, 0, color))
			case top == 0 && bottom != 0:
				b.WriteString(pixelBlock(0, bottom, color))
			default:
				b.WriteString(pixelBlock(top, bottom, color))
			}
		}
		rows[y/2] = b.String()
	}
	return rows
}

func pixelBlock(top, bottom rune, color bool) string {
	glyph := "▀"
	if top == 0 {
		glyph = "▄"
	}
	if !color {
		return glyph
	}
	if top == 0 {
		return "\x1b[38;5;" + itoa(pixelPalette[bottom]) + "m" + glyph + pixReset
	}
	if bottom == 0 {
		return "\x1b[38;5;" + itoa(pixelPalette[top]) + "m" + glyph + pixReset
	}
	return "\x1b[38;5;" + itoa(pixelPalette[top]) +
		"m\x1b[48;5;" + itoa(pixelPalette[bottom]) + "m" + glyph + pixReset
}

const mascotPixelWidth = pixW

func pixelAt(x, y int) rune {
	if y < 0 || y >= len(mascotPixel) {
		return 0
	}
	row := mascotPixel[y]
	if x < 0 || x >= len(row) {
		return 0
	}
	if row[x] == '.' {
		return 0
	}
	return rune(row[x])
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
