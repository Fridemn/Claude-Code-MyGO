package ui

import (
	"fmt"
	"strings"
)

const reset = "\033[0m"

type rgb struct {
	r int
	g int
	b int
}

// RGB is an exported RGB color type
type RGB struct {
	R, G, B int
}

type palette struct {
	claude                rgb
	claudeShimmer         rgb // Lighter claude orange for shimmer effect
	claudeDim             rgb
	permission            rgb
	text                  rgb
	subtle                rgb
	muted                 rgb
	inputLine             rgb
	background            rgb
	promptBorder          rgb
	success               rgb
	error                 rgb
	warning               rgb
	info                  rgb
	userLabel             rgb
	userMessageBackground rgb
	assistantBackground   rgb
	commandBackground     rgb
	errorBackground       rgb
	noticeBackground      rgb
	panelBackground       rgb
	panelAccent           rgb
	panelBorder           rgb
}

// DarkPalette is the exported dark color palette for use in external packages
type DarkPalette struct {
	Warning rgb
	Error   rgb
	Muted   rgb
	Claude  rgb
}

// Dark is the exported dark palette
var Dark = DarkPalette{
	Warning: rgb{255, 193, 7},
	Error:   rgb{255, 107, 128},
	Muted:   rgb{134, 145, 160},
	Claude:  rgb{215, 119, 87},
}

var dark = palette{
	claude:                rgb{215, 119, 87},  // Claude orange
	claudeShimmer:         rgb{245, 149, 117}, // Lighter claude orange for shimmer effect
	claudeDim:             rgb{135, 81, 60},
	permission:            rgb{177, 185, 249},
	text:                  rgb{255, 255, 255},
	subtle:                rgb{80, 80, 80},
	muted:                 rgb{134, 145, 160},
	inputLine:             rgb{150, 150, 150},
	background:            rgb{10, 10, 10},
	promptBorder:          rgb{136, 136, 136},
	success:               rgb{78, 186, 101},
	error:                 rgb{255, 107, 128},
	warning:               rgb{255, 193, 7},
	info:                  rgb{115, 198, 255},
	userLabel:             rgb{122, 180, 232},
	userMessageBackground: rgb{55, 55, 55},
	assistantBackground:   rgb{28, 24, 23},
	commandBackground:     rgb{44, 50, 62},
	errorBackground:       rgb{60, 22, 30},
	noticeBackground:      rgb{33, 30, 52},
	panelBackground:       rgb{24, 33, 45},
	panelAccent:           rgb{92, 162, 255},
	panelBorder:           rgb{78, 122, 168},
}

// GoBlue is the Go gopher blue color
var GoBlue = rgb{0, 173, 239}

func bold(v string) string { return "\033[1m" + v + reset }

func dim(v string) string { return "\033[2m" + v + reset }

func fg(c rgb, v string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s%s", c.r, c.g, c.b, v, reset)
}

func bg(c rgb, v string) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm%s%s", c.r, c.g, c.b, v, reset)
}

func style(fgColor, bgColor *rgb, text string, isBold bool) string {
	var b strings.Builder
	if isBold {
		b.WriteString("\033[1m")
	}
	if fgColor != nil {
		b.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm", fgColor.r, fgColor.g, fgColor.b))
	}
	if bgColor != nil {
		b.WriteString(fmt.Sprintf("\033[48;2;%d;%d;%dm", bgColor.r, bgColor.g, bgColor.b))
	}
	b.WriteString(text)
	b.WriteString(reset)
	return b.String()
}

// Style is an exported version of style for use in external packages
func Style(fgColor, bgColor *rgb, text string, isBold bool) string {
	return style(fgColor, bgColor, text, isBold)
}

func badge(label string, fgColor, bgColor rgb) string {
	return style(&fgColor, &bgColor, " "+label+" ", true)
}

func line(width int, ch string) string {
	if width <= 0 {
		width = 76
	}
	return strings.Repeat(ch, width)
}

func padRight(v string, width int) string {
	if width <= len(v) {
		return v
	}
	return v + strings.Repeat(" ", width-len(v))
}

func box(title string, body []string, border rgb, titleFg rgb, bodyFg rgb, bodyBg *rgb) string {
	width := 76
	top := style(&border, nil, "┌"+line(width-2, "─")+"┐", false)
	bottom := style(&border, nil, "└"+line(width-2, "─")+"┘", false)
	out := []string{top}

	if strings.TrimSpace(title) != "" {
		left := style(&border, nil, "│ ", false)
		right := style(&border, nil, "│", false)
		content := style(&titleFg, bodyBg, padRight(title, width-3), true)
		out = append(out, left+content+right)
		out = append(out, style(&border, nil, "├"+line(width-2, "─")+"┤", false))
	}

	if len(body) == 0 {
		body = []string{""}
	}

	for _, row := range body {
		row = strings.ReplaceAll(row, "\t", "  ")
		for _, segment := range strings.Split(row, "\n") {
			left := style(&border, nil, "│ ", false)
			right := style(&border, nil, "│", false)
			content := style(&bodyFg, bodyBg, padRight(segment, width-3), false)
			out = append(out, left+content+right)
		}
	}

	out = append(out, bottom)
	return strings.Join(out, "\n")
}
