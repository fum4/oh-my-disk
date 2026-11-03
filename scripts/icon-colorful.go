//go:build ignore

// More colorful palette previews. Run: go run scripts/icon-colorful.go
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
		{{173, 119, 235}, {70, 205, 210}, {247, 170, 79}, {88, 145, 235}, {241, 115, 168}, {151, 213, 112}, {249, 211, 107}},
		{{246, 125, 111}, {75, 198, 183}, {179, 133, 238}, {242, 188, 76}, {105, 167, 241}, {237, 132, 193}, {166, 214, 111}},
		{{101, 169, 246}, {249, 188, 76}, {238, 116, 182}, {112, 207, 168}, {177, 140, 243}, {251, 143, 100}, {91, 210, 224}},
	}
	out := "build/icon-colorful"
	if e := os.MkdirAll(out, 0755); e != nil {
		panic(e)
	}
	sheet := image.NewNRGBA(image.Rect(0, 0, 864, 320))
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
