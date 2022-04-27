package main

import (
	"embed"
	"flag"
	"image/png"
	"log"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
)

const (
	// Bit width of half-width characters.
	bitsHW = 32
	// Glyph width of half-width characters.
	glyphWidthHW = 4
	// Glyph height of half-width characters.
	glyphHeightHW = 8
	// Bit width of full-width characters.
	bitsFW = 64
	// Glyph width of full-width characters.
	glyphWidthFW = 8
	// Glyph height of full-width characters.
	glyphHeightFW = 8

	fw1FirstByteStart = 0x81
	fw1FirstByteEnd   = 0x9F
	fw2FirstByteStart = 0xE0
	fw2FirstByteEnd   = 0xEF
	fwSecondByteStart = 0x40
	fwSecondByteEnd   = 0x9F
)

//go:embed assets/*.png
var fontFiles embed.FS

// One glyph for half-width characters.
type GlyphHW uint32

// One glyph for full-width characters.
type GlyphFW uint64

type Font struct {
	// Half-width glyphs. It is keyed by a raw character code.
	glyphsHW *[256]GlyphHW
	// Full-width glyphs.
	glyphsFW1 *[31][189]GlyphFW
	// Full-width glyphs.
	glyphsFW2 *[16][189]GlyphFW
}

type CharClass uint8

const (
	charClassHW = iota
	charClassFW1
	charClassFW2
)

// Get character class.
func getCharClass(b byte) CharClass {
	if fw1FirstByteStart <= b && b <= fw1FirstByteEnd {
		return charClassFW1
	} else if fw2FirstByteStart <= b && b <= fw2FirstByteEnd {
		return charClassFW2
	} else {
		return charClassHW
	}
}

func glyphHWToglyphFW(gHW GlyphHW) GlyphFW {
	gFW := GlyphFW(0)
	for i := 0; i < bitsHW; i++ {
		if gHW&(1<<i) != 0 {
			j := i/4*8 + i%4
			gFW |= 1 << j
		}
	}
	return gFW
}

func utf8ToShiftJISReplacingUnsupported(in string) (string, error) {
	e := encoding.ReplaceUnsupported(japanese.ShiftJIS.NewEncoder())
	return e.String(in)
}

type Banner []string

func NewBanner(lines []string) (Banner, error) {
	b := make(Banner, len(lines))
	for i, line := range lines {
		lineShiftJIS, err := utf8ToShiftJISReplacingUnsupported(line)
		if err != nil {
			return nil, err
		}
		b[i] = lineShiftJIS
	}
	return b, nil
}

type Renderer struct {
	scr          tcell.Screen
	squareWidth  int
	squareHeight int
	bgStyle      tcell.Style
	fgStyle      tcell.Style
}

func NewRenderer(bgStyle, fgStyle tcell.Style) (*Renderer, error) {
	scr, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	err = scr.Init()
	if err != nil {
		return nil, err
	}

	r := &Renderer{scr, 0, 0, bgStyle, fgStyle}
	return r, nil
}

func (r *Renderer) ScreenSize() (int, int) {
	return r.scr.Size()
}

func (r *Renderer) SetSquareSize(w, h int) {
	r.squareWidth = w
	r.squareHeight = h
}

func (r *Renderer) Fini() {
	r.scr.Fini()
}

func (r *Renderer) Show() {
	r.scr.Show()
}

func (r *Renderer) PollEvent() tcell.Event {
	return r.scr.PollEvent()
}

func (r *Renderer) Sync() {
	r.scr.Sync()
}

func (r *Renderer) ClearScreen() {
	r.scr.SetStyle(r.bgStyle)
	r.scr.Clear()
}

func (r *Renderer) DrawSquare(sx, sy int) {
	w, h := r.squareWidth, r.squareHeight
	for dx := 0; dx < w; dx++ {
		for dy := 0; dy < h; dy++ {
			r.scr.SetContent(sx*w+dx, sy*h+dy, ' ', nil, r.fgStyle)
		}
	}
}

func drawGlyph(r *Renderer, g GlyphFW, sx, sy int) {
	for i := 0; i < bitsFW; i++ {
		filled := g&(1<<i) != 0
		if !filled {
			continue
		}
		dx := i % glyphWidthFW
		dy := i / glyphWidthFW
		r.DrawSquare(sx+dx, sy+dy)
	}
}

func calcGridWidth(s string) int {
	w := 0
	for i := 0; i < len(s); i++ {
		switch getCharClass(s[i]) {
		case charClassHW:
			w += 1
		case charClassFW1, charClassFW2:
			i++
			w += 2
		}
	}

	return w * glyphWidthHW
}

func calcSquareSizeAndOffset(r *Renderer, banner Banner) (int, int, []int, int) {
	scrW, scrH := r.ScreenSize()

	gridWidthMax := 0
	gridWidths := make([]int, len(banner))
	for i, line := range banner {
		gridWidths[i] = calcGridWidth(line)
		if gridWidthMax < gridWidths[i] {
			gridWidthMax = gridWidths[i]
		}
	}
	gridHeight := glyphHeightFW * len(banner)

	squareW := scrW / gridWidthMax
	squareH := scrH / gridHeight
	if squareW > squareH*8 {
		squareW = squareH * 8
	}
	if squareH > squareW {
		squareH = squareW
	}

	xOffsets := make([]int, len(banner))
	for i, gridWidth := range gridWidths {
		xOffsets[i] = (scrW/squareW - gridWidth) / 2
	}
	yOffset := (scrH/squareH - gridHeight) / 2

	return squareW, squareH, xOffsets, yOffset
}

func drawOneLine(r *Renderer, s string, xOffset, yOffset int, font *Font) {
	for i := 0; i < len(s); i++ {
		b := s[i]
		x := xOffset + i*glyphWidthHW
		y := yOffset
		var g GlyphFW
		switch getCharClass(b) {
		case charClassHW:
			g = glyphHWToglyphFW(font.glyphsHW[b])
		case charClassFW1:
			b2 := s[i+1]
			g = font.glyphsFW1[b-fw1FirstByteStart][b2-fwSecondByteStart]
			i++
		case charClassFW2:
			b2 := s[i+1]
			g = font.glyphsFW1[b-fw2FirstByteStart][b2-fwSecondByteStart]
			i++
		}
		drawGlyph(r, g, x, y)
	}
}

func drawBanner(r *Renderer, banner Banner, font *Font) {
	r.ClearScreen()

	sw, sh, xOffsets, yOffset := calcSquareSizeAndOffset(r, banner)
	r.SetSquareSize(sw, sh)

	for i, line := range banner {
		drawOneLine(r, line, xOffsets[i], yOffset+i*glyphHeightFW, font)
	}
}

func parseGlyphsHW(filePath string) (*[256]GlyphHW, error) {
	fp, err := fontFiles.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	img, err := png.Decode(fp)
	if err != nil {
		return nil, err
	}

	gs := [256]GlyphHW{}
	for dy := 0; dy < 16; dy++ {
		for dx := 0; dx < 16; dx++ {
			glyph := GlyphHW(0)
			for i := 0; i < bitsHW; i++ {
				x := dx*glyphWidthHW + i%glyphWidthHW
				y := dy*glyphHeightHW + i/glyphWidthHW
				r, g, b, _ := img.At(x, y).RGBA()
				if r == 0 && b == 0 && g == 0 {
					glyph |= 1 << i
				}
			}
			c := dy*16 + dx
			gs[c] = glyph
		}
	}
	return &gs, nil
}

func parseGlyphsFW(filePath string) (*[31][189]GlyphFW, *[16][189]GlyphFW, error) {
	fp, err := fontFiles.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer fp.Close()

	img, err := png.Decode(fp)
	if err != nil {
		return nil, nil, err
	}

	gs1 := [31][189]GlyphFW{}
	for dy := 0; dy < 62; dy++ {
		for dx := 0; dx < 94; dx++ {
			glyph := GlyphFW(0)
			for i := 0; i < bitsFW; i++ {
				x := dx*glyphWidthFW + i%glyphWidthFW
				y := dy*glyphHeightFW + i/glyphWidthFW
				r, g, b, _ := img.At(x, y).RGBA()
				if r == 0 && b == 0 && g == 0 {
					glyph |= 1 << i
				}
			}
			c1 := dy / 2
			c2 := dx + (fwSecondByteEnd-fwSecondByteStart)*(dy%2)
			gs1[c1][c2] = glyph
		}
	}

	yOffset := 31 * glyphHeightFW
	gs2 := [16][189]GlyphFW{}
	for dy := 0; dy < 16; dy++ {
		for dx := 0; dx < 94; dx++ {
			glyph := GlyphFW(0)
			for i := 0; i < bitsFW; i++ {
				x := dx*glyphWidthFW + i%glyphWidthFW
				y := dy*glyphHeightFW + i/glyphWidthFW + yOffset
				r, g, b, _ := img.At(x, y).RGBA()
				if r == 0 && b == 0 && g == 0 {
					glyph |= 1 << i
				}
			}
			c1 := dy / 2
			c2 := dx + (fwSecondByteEnd-fwSecondByteStart)*(dy%2)
			gs2[c1][c2] = glyph
		}
	}

	return &gs1, &gs2, nil
}

func prepareFont(fileHW, fileFW string) (*Font, error) {
	glyphsHW, err := parseGlyphsHW(fileHW)
	if err != nil {
		return nil, err
	}
	glyphsFW1, glyphsFW2, err := parseGlyphsFW(fileFW)
	if err != nil {
		return nil, err
	}
	return &Font{glyphsHW, glyphsFW1, glyphsFW2}, nil
}

func main() {
	var fontType = flag.String("f", "mincho", "Font (mincho or gothic)")
	flag.Parse()
	var fontFileHW string
	var fontFileFW string
	if *fontType == "mincho" {
		fontFileHW = "assets/misaki_gothic_2nd_4x8.png"
		fontFileFW = "assets/misaki_mincho.png"
	} else if *fontType == "gothic" {
		fontFileHW = "assets/misaki_gothic_2nd_4x8.png"
		fontFileFW = "assets/misaki_gothic_2nd.png"
	} else {
		log.Fatalf("Unknown font: %s", *fontType)
	}

	if flag.NArg() == 0 {
		return
	}

	font, err := prepareFont(fontFileHW, fontFileFW)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	r, err := NewRenderer(
		tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset),
		tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorOlive),
	)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer r.Fini()

	banner, err := NewBanner(flag.Args())
	if err != nil {
		log.Fatalf("%+v", err)
	}
	drawBanner(r, banner, font)

	for {
		r.Show()

		ev := r.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			drawBanner(r, banner, font)
			r.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC || ev.Rune() == 'q' {
				return
			}
		}
	}
}
