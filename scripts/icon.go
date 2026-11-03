//go:build ignore

// Prism: the selected storage-block identity for oh-my-disk.
// Original procedural artwork. Run: go run scripts/icon.go
package main

import (
	"image"
	"image/color"
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
func main() {
	// Prism palette, in reading order: violet, turquoise, orange, blue, pink, lime, yellow.
	tiles := []tile{
		{164, 164, 420, 420, col{173, 119, 235}},
		{602, 164, 258, 201, col{70, 205, 210}},
		{602, 383, 258, 201, col{247, 170, 79}},
		{164, 602, 258, 258, col{88, 145, 235}},
		{440, 602, 244, 258, col{241, 115, 168}},
		{702, 602, 158, 120, col{151, 213, 112}},
		{702, 740, 158, 120, col{249, 211, 107}},
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
	f, e := os.Create("build/appicon.png")
	if e != nil {
		panic(e)
	}
	defer f.Close()
	if e = png.Encode(f, im); e != nil {
		panic(e)
	}
}
