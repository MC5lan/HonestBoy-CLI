package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Theme 是暗/亮/无色三套主题 token。
// 所有界面颜色都必须从这里取，渲染层不得再写死 ANSI 或 256 色号。
type Theme struct {
	Name                           string
	Fg, Dim, Accent, Success, Warn lipgloss.Color
	Danger                         lipgloss.Color
	CodeBg, CodeFg                 lipgloss.Color
	Border                         lipgloss.Border
	DialogBorder                   lipgloss.Color
	PanelBorder                    lipgloss.Color
	ComposerBg                     lipgloss.Color
	SelectionBg                    lipgloss.Color
}

func Load(name string) Theme {
	if name == "" {
		name = "auto"
	}
	if name == "auto" {
		if Detect() {
			name = "light"
		} else {
			name = "dark"
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		name = "none"
	}
	switch name {
	case "light":
		return Theme{Name: "light", Fg: "235", Dim: "245", Accent: "33", Success: "22", Warn: "130", Danger: "160",
			CodeBg: "255", CodeFg: "235", Border: lipgloss.RoundedBorder(),
			DialogBorder: "250", PanelBorder: "33", ComposerBg: "254", SelectionBg: "253"}
	case "none":
		return Theme{Name: "none", Fg: "", Dim: "", Accent: "", Success: "", Warn: "", Danger: "",
			CodeBg: "", CodeFg: "", Border: lipgloss.RoundedBorder(),
			DialogBorder: "", PanelBorder: "", ComposerBg: "", SelectionBg: ""}
	default:
		return Theme{Name: "dark", Fg: "252", Dim: "244", Accent: "39", Success: "46", Warn: "214", Danger: "196",
			CodeBg: "235", CodeFg: "252", Border: lipgloss.RoundedBorder(),
			DialogBorder: "240", PanelBorder: "39", ComposerBg: "237", SelectionBg: "236"}
	}
}

// Detect 猜终端是否亮色；猜不到默认暗色。
func Detect() bool {
	fgbg := os.Getenv("COLORFGBG")
	return fgbg != "" && fgbg != "15;0"
}

func (t Theme) Style() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Fg) }

func (t Theme) DimStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Dim) }

func (t Theme) AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
}

func (t Theme) SuccessStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Success) }

func (t Theme) WarnStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Warn) }

func (t Theme) DangerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(t.Danger) }

// UserStyle 是用户消息样式：加粗 › 提示符。
func (t Theme) UserStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Fg).Bold(true)
}

// ComposerStyle 是底部输入区样式：Codex 风格的深色背景块。
func (t Theme) ComposerStyle() lipgloss.Style {
	s := lipgloss.NewStyle().PaddingLeft(1)
	if t.Name == "none" || t.ComposerBg == "" {
		return s
	}
	return s.Background(t.ComposerBg)
}

// DialogBox 是对话区边框样式。
func (t Theme) DialogBox() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.Border).
		BorderForeground(t.DialogBorder).
		Padding(0, 1)
}

// PanelBox 是右侧可观测面板边框样式。
func (t Theme) PanelBox() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.Border).
		BorderForeground(t.PanelBorder).
		Padding(0, 1)
}
