//go:build ignore

// Original procedural artwork; no external assets. Run: go run scripts/icon.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	const size = 1024
	im := image.NewNRGBA(image.Rect(0, 0, size, size))
	palette := []color.NRGBA{{182, 152, 242, 255}, {126, 109, 217, 255}, {113, 151, 239, 255}, {94, 199, 203, 255}, {198, 164, 243, 255}, {148, 120, 219, 255}}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var rr, gg, bb, aa float64
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					px, py := float64(x)+float64(sx)/2, float64(y)+float64(sy)/2
					dx, dy := math.Abs(px-512)-306, math.Abs(py-512)-306
					distance := math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - 150
					c := color.NRGBA{}
					if distance < 0 {
						t := py / 1024
						c = color.NRGBA{uint8(31 - 11*t), uint8(32 - 10*t), uint8(55 - 17*t), 255}
						r := math.Hypot(px-512, py-512)
						angle := math.Atan2(py-512, px-512) + math.Pi
						if r > 143 && r < 359 {
							ring := int((r - 143) / 73)
							segments := []int{7, 12, 18}[ring]
							position := angle / (2 * math.Pi) * float64(segments)
							if r-143-float64(ring)*73 > 5 && position-math.Floor(position) > .018 {
								c = palette[(int(position)+ring*2)%len(palette)]
								shade := 1 - float64(ring)*.10
								c.R = uint8(float64(c.R) * shade)
								c.G = uint8(float64(c.G) * shade)
								c.B = uint8(float64(c.B) * shade)
							}
						}
						if r < 116 {
							c = color.NRGBA{211, 195, 250, 255}
						}
						if r < 57 {
							c = color.NRGBA{39, 34, 64, 255}
						}
						if math.Hypot(px-512, py-505) < 15 {
							c = color.NRGBA{182, 152, 242, 255}
						}
					}
					rr += float64(c.R) * float64(c.A) / 255
					gg += float64(c.G) * float64(c.A) / 255
					bb += float64(c.B) * float64(c.A) / 255
					aa += float64(c.A)
				}
			}
			if aa > 0 {
				im.SetNRGBA(x, y, color.NRGBA{uint8(rr * 255 / aa), uint8(gg * 255 / aa), uint8(bb * 255 / aa), uint8(aa / 4)})
			}
		}
	}
	if err := os.MkdirAll("build", 0755); err != nil {
		panic(err)
	}
	f, err := os.Create("build/appicon.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err = png.Encode(f, im); err != nil {
		panic(err)
	}
}
