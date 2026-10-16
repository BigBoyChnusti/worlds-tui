package ui

import (
	"fmt"
	"image"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// A champion icon is drawn as truecolor half blocks: each cell shows two
// pixels stacked, the upper one as the foreground of "▀" and the lower one
// as the background. Plain text cells keep lipgloss width math and the
// Bubble Tea renderer working, unlike the kitty or sixel image protocols.
const (
	iconCols = 4 // cells, and pixels per row
	iconRows = 2 // lines; each holds two pixel rows
)

// rgb is one pixel of a downscaled icon.
type rgb struct{ r, g, b uint8 }

func (c rgb) hex() string { return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b) }

// champIcon holds the pixels of an icon, iconRows*2 high and iconCols wide.
type champIcon [iconRows * 2][iconCols]rgb

// newChampIcon downscales img to the icon size.
func newChampIcon(img image.Image) *champIcon {
	var ic champIcon
	px := downscale(img, iconCols, iconRows*2)
	for y := range ic {
		copy(ic[y][:], px[y])
	}
	return &ic
}

// downscale shrinks img to w x h pixels with a box filter: every output
// pixel is the average of the source pixels its box covers. Transparent
// pixels count as black, which matches the dark tile the icons sit on.
func downscale(img image.Image, w, h int) [][]rgb {
	b := img.Bounds()
	out := make([][]rgb, h)
	for y := range h {
		out[y] = make([]rgb, w)
		y0 := b.Min.Y + y*b.Dy()/h
		y1 := max(y0+1, b.Min.Y+(y+1)*b.Dy()/h)
		for x := range w {
			x0 := b.Min.X + x*b.Dx()/w
			x1 := max(x0+1, b.Min.X+(x+1)*b.Dx()/w)
			var r, g, bl, n uint64
			for sy := y0; sy < y1 && sy < b.Max.Y; sy++ {
				for sx := x0; sx < x1 && sx < b.Max.X; sx++ {
					// RGBA is alpha-premultiplied, 16 bits per channel.
					cr, cg, cb, _ := img.At(sx, sy).RGBA()
					r, g, bl, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), n+1
				}
			}
			if n > 0 {
				out[y][x] = rgb{uint8((r / n) >> 8), uint8((g / n) >> 8), uint8((bl / n) >> 8)}
			}
		}
	}
	return out
}

// iconBlank fills the icon's place when it is missing.
var iconBlank = strings.Repeat(" ", iconCols)

// iconLine renders line row (0 or 1) of an icon, iconCols cells wide. A
// missing icon, or a terminal without colour, gets blank cells instead.
func iconLine(ic *champIcon, row int) string {
	if ic == nil || row < 0 || row >= iconRows || lipgloss.ColorProfile() == termenv.Ascii {
		return iconBlank
	}
	var b strings.Builder
	for x := range iconCols {
		top, bottom := ic[row*2][x], ic[row*2+1][x]
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(top.hex())).
			Background(lipgloss.Color(bottom.hex())).
			Render("▀"))
	}
	return b.String()
}
