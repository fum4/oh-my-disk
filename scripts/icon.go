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
	palette := []color.NRGBA{{244, 244, 246, 255}, {190, 191, 198, 255}, {218, 219, 224, 255}, {138, 139, 147, 255}, {244, 244, 246, 255}, {190, 191, 198, 255}}
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
						c = color.NRGBA{uint8(28 - 14*t), uint8(29 - 14*t), uint8(34 - 16*t), 255}
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
							c = color.NRGBA{244, 244, 246, 255}
						}
						if r < 57 {
							c = color.NRGBA{20, 21, 26, 255}
						}
						if math.Hypot(px-512, py-505) < 15 {
							c = color.NRGBA{138, 139, 147, 255}
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
