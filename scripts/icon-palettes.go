//go:build ignore

// Color palette previews. Run: go run scripts/icon-palettes.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
)

type col struct{ r, g, b float64 }
type tile struct {
	x, y, w, h float64
	c          col
}

func rounded(x, y, cx, cy, w, h, r float64) float64 {
	dx, dy := math.Abs(x-cx)-(w/2-r), math.Abs(y-cy)-(h/2-r)
	return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - r
}
func render(palette []col, output string) *image.NRGBA {
	tiles := []tile{
		{164, 164, 420, 420, palette[0]},
		{602, 164, 258, 201, palette[1]},
		{602, 383, 258, 201, palette[2]},
		{164, 602, 258, 258, palette[3]},
		{440, 602, 244, 258, palette[4]},
		{702, 602, 158, 120, palette[5]},
		{702, 740, 158, 120, palette[6]},
	}
	im := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			var rr, gg, bb, aa float64
			for sy := 0; sy < 3; sy++ {
				for sx := 0; sx < 3; sx++ {
					px, py := float64(x)+float64(sx)/3, float64(y)+float64(sy)/3
					d := rounded(px, py, 512, 512, 912, 912, 190)
					if d >= 0 {
						continue
					}
					t := py / 1024
					c := col{42 - 16*t, 35 - 13*t, 65 - 22*t}
					if d > -4 {
						c = col{105, 90, 140}
					}
					for _, b := range tiles {
						b.x = 512 + (b.x-512)*0.84
						b.y = 512 + (b.y-512)*0.84
						b.w *= 0.84
						b.h *= 0.84
						b.x += 7.5
						b.y += 7.5
						b.w -= 15
						b.h -= 15
						td := rounded(px, py, b.x+b.w/2, b.y+b.h/2, b.w, b.h, 26)
						if td < 0 {
							shade := 1.055 - .12*(py-b.y)/b.h
							c = col{b.c.r * shade, b.c.g * shade, b.c.b * shade}
							if td > -2 {
								c = col{c.r + 10, c.g + 10, c.b + 10}
							}
						}
					}
					rr += c.r
					gg += c.g
					bb += c.b
					aa++
				}
			}
			if aa > 0 {
				im.SetNRGBA(x, y, color.NRGBA{uint8(math.Min(255, rr/aa)), uint8(math.Min(255, gg/aa)), uint8(math.Min(255, bb/aa)), uint8(255 * aa / 9)})
			}
		}
	}
	f, e := os.Create(output)
	if e != nil {
		panic(e)
	}
	defer f.Close()
	if e = png.Encode(f, im); e != nil {
		panic(e)
	}
	return im
}

func main() {
	palettes := [][]col{
		{{77, 171, 218}, {90, 205, 205}, {91, 137, 210}, {72, 112, 177}, {150, 205, 230}, {120, 172, 221}, {110, 151, 192}},
		{{104, 175, 139}, {167, 199, 125}, {75, 143, 129}, {94, 126, 116}, {196, 210, 153}, {121, 180, 163}, {147, 164, 112}},
		{{226, 147, 109}, {236, 192, 122}, {204, 112, 116}, {166, 103, 130}, {241, 188, 157}, {207, 155, 124}, {190, 131, 164}},
		{{192, 137, 212}, {119, 189, 207}, {150, 147, 222}, {113, 162, 205}, {224, 168, 203}, {157, 205, 190}, {201, 181, 231}},
		{{190, 196, 211}, {143, 155, 178}, {167, 179, 198}, {113, 125, 151}, {218, 223, 233}, {147, 165, 185}, {181, 187, 204}},
	}
	out := "build/icon-palettes"
	if e := os.MkdirAll(out, 0755); e != nil {
		panic(e)
	}
	sheet := image.NewNRGBA(image.Rect(0, 0, 1440, 320))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.NRGBA{235, 236, 240, 255}}, image.Point{}, draw.Src)
	for i, p := range palettes {
		im := render(p, fmt.Sprintf("%s/%d.png", out, i+1))
		thumb := image.NewNRGBA(image.Rect(0, 0, 256, 256))
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				var rr, gg, bb, aa uint32
				for sy := 0; sy < 4; sy++ {
					for sx := 0; sx < 4; sx++ {
						c := im.NRGBAAt(x*4+sx, y*4+sy)
						a := uint32(c.A)
						rr += uint32(c.R) * a
						gg += uint32(c.G) * a
						bb += uint32(c.B) * a
						aa += a
					}
				}
				if aa > 0 {
					thumb.SetNRGBA(x, y, color.NRGBA{uint8(rr / aa), uint8(gg / aa), uint8(bb / aa), uint8(aa / 16)})
				}
			}
		}
		draw.Draw(sheet, image.Rect(i*288+16, 32, i*288+272, 288), thumb, image.Point{}, draw.Over)
	}
	f, e := os.Create(out + "/comparison.png")
	if e != nil {
		panic(e)
	}
	defer f.Close()
	if e = png.Encode(f, sheet); e != nil {
		panic(e)
	}
}
