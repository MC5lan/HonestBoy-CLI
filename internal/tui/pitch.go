package tui

import "strings"

// 2D 侧视足球场：很小的像素小男孩在左侧颠球，右侧球门。
// 高度 10 行：0-1 球区，2-8 小人，9 地面。
type pitchPhase uint8

const (
	pitchIdle pitchPhase = iota
	pitchWorking
	pitchKick
	pitchGoal
)

const (
	pitchRows = 10
	groundRow = pitchRows - 1
	mascotY   = 3

	ballGlyph = "⚽"
	ballW     = 2
)

// renderPitch 渲染侧视球场（仅球场，不含 GOAL 艺术字）。
// color=false 时不输出任何 ANSI，供 NO_COLOR / theme=none 使用。
func renderPitch(phase pitchPhase, frame, width int, color bool) string {
	if width < 30 {
		width = 30
	}
	const goalW = 5
	mascotX := 2
	goalX := width - goalW - 1
	ballHomeX := mascotX + mascotPixelWidth + 2

	goalStyle := ""
	dimStyle := ""
	reset := ""
	if color {
		goalStyle = "\x1b[96m"
		dimStyle = "\x1b[90m"
		reset = "\x1b[0m"
	}

	rows := make([]string, pitchRows)
	for i := range rows {
		rows[i] = strings.Repeat(" ", width)
	}
	rows[groundRow] = strings.Repeat("─", width)

	// 中圈线：让宽场地有球场层次。
	mid := width / 2
	for r := 1; r < groundRow; r += 2 {
		rows[r] = overlayStyled(rows[r], mid, dimStyle+"┊"+reset, 1)
	}
	rows[groundRow-1] = overlayStyled(rows[groundRow-1], mid, dimStyle+"○"+reset, 1)

	// 右侧球门（5 宽，8 行高：2 到地面），明显高于小人。
	for r := 2; r <= groundRow; r++ {
		box := "│   │"
		switch r {
		case 2:
			box = "┌───┐"
		case groundRow:
			box = "└───┘"
		}
		rows[r] = overlayStyled(rows[r], goalX, goalStyle+box+reset, goalW)
	}

	// 侧视小人（6 行）。
	for i, line := range renderPixelMascot(color) {
		y := mascotY + i
		if y >= groundRow {
			break
		}
		rows[y] = overlayStyled(rows[y], mascotX, line, mascotPixelWidth)
	}

	// 球：脚边上下弹；射门时飞向球门。
	ballX, ballY := ballHomeX, groundRow-1
	switch phase {
	case pitchIdle, pitchWorking:
		switch frame % 4 {
		case 0:
			ballX, ballY = ballHomeX, groundRow-1
		case 1:
			ballX, ballY = ballHomeX+1, groundRow-3
		case 2:
			ballX, ballY = ballHomeX+2, groundRow-5
		default:
			ballX, ballY = ballHomeX+1, groundRow-3
		}
	case pitchKick:
		ballX = ballHomeX + 2 + frame*6
		ballY = groundRow - 2 - frame
		if ballX > goalX+2 {
			ballX = goalX + 2
		}
		if ballY < 2 {
			ballY = 2
		}
	case pitchGoal:
		ballX, ballY = goalX+2, 4
	}
	if ballX >= 0 && ballX+ballW <= width-1 && ballY >= 0 && ballY < pitchRows {
		rows[ballY] = overlayStyled(rows[ballY], ballX, ballGlyph, ballW)
	}

	return strings.Join(rows, "\n")
}

// overlayStyled 按“显示列”覆盖，跳过 ANSI，不会切断已有颜色。
func overlayStyled(line string, col int, styled string, displayWidth int) string {
	if col < 0 {
		return line
	}
	var b strings.Builder
	display := 0
	runes := []rune(line)
	i := 0
	done := false
	skipANSI := func(start int) int {
		j := start + 1
		for j < len(runes) {
			c := runes[j]
			j++
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				break
			}
		}
		return j
	}
	for i < len(runes) {
		if runes[i] == 0x1b {
			j := skipANSI(i)
			b.WriteString(string(runes[i:j]))
			i = j
			continue
		}
		if !done && display == col {
			b.WriteString(styled)
			done = true
			skip := displayWidth
			for i < len(runes) && skip > 0 {
				if runes[i] == 0x1b {
					i = skipANSI(i)
					continue
				}
				i++
				skip--
			}
			continue
		}
		b.WriteRune(runes[i])
		i++
		display++
	}
	if !done {
		b.WriteString(styled)
	}
	return b.String()
}
