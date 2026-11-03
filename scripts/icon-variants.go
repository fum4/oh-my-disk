//go:build ignore

// Preview alternative procedural CD case icons: go run scripts/icon-variants.go
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

type rgb struct{ r, g, b float64 }

func mix(a, b rgb, t float64) rgb {
	t = math.Max(0, math.Min(1, t))
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}
func rounded(x, y, cx, cy, w, h, r float64) float64 {
	dx, dy := math.Abs(x-cx)-(w/2-r), math.Abs(y-cy)-(h/2-r)
	return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - r
}
func pixel(v int, x, y float64) (rgb, bool) {
	d := rounded(x, y, 512, 512, 912, 912, 150)
	if d >= 0 {
		return rgb{}, false
	}
	t := y / 1024
	var c rgb
	switch v {
	case 1: // Classic jewel case, silver-lavender disc.
		c = mix(rgb{57, 49, 83}, rgb{30, 26, 48}, t)
		if x < 203 {
			c = mix(rgb{92, 77, 126}, rgb{52, 43, 78}, t)
		}
		if math.Abs(x-203) < 5 {
			c = rgb{177, 158, 211}
		}
		if d > -4 {
			c = rgb{139, 124, 167}
		}
	case 2: // Midnight case with a mosaic data disc.
		c = mix(rgb{29, 34, 53}, rgb{15, 18, 31}, t)
		if x < 175 {
			c = rgb{38, 44, 63}
		}
		if math.Abs(x-175) < 4 {
			c = rgb{91, 107, 140}
		}
		if d > -3 {
			c = rgb{77, 89, 115}
		}
	case 3: // Frosted case, cool blue iridescence.
		c = mix(rgb{211, 225, 235}, rgb{145, 163, 185}, t)
		if x < 199 {
			c = mix(rgb{166, 187, 208}, rgb{118, 139, 165}, t)
		}
		if math.Abs(x-199) < 6 {
			c = rgb{235, 245, 250}
		}
		if d > -5 {
			c = rgb{237, 246, 251}
		}
	case 4: // Cream case and graphite disc.
		c = mix(rgb{244, 235, 216}, rgb{213, 201, 178}, t)
		if x < 193 {
			c = rgb{193, 180, 153}
		}
		if math.Abs(x-193) < 4 {
			c = rgb{156, 143, 119}
		}
		if d > -4 {
			c = rgb{171, 156, 130}
		}
	case 5: // Plum case and luminous continuous rings.
		c = mix(rgb{51, 32, 70}, rgb{23, 18, 37}, t)
		if x < 168 {
			c = rgb{65, 42, 85}
		}
		if math.Abs(x-168) < 5 {
			c = rgb{164, 122, 194}
		}
		if d > -4 {
			c = rgb{113, 79, 143}
		}
	}
	cx, cy, radius := 585.0, 512.0, 301.0
	if v == 2 {
		cx = 570
		radius = 319
	}
	if v == 5 {
		cx = 566
		radius = 325
	}
	dx, dy := x-cx, y-cy
	r := math.Hypot(dx, dy)
	a := math.Atan2(dy, dx)
	if r < radius {
		switch v {
		case 1:
			c = mix(rgb{228, 218, 246}, rgb{139, 130, 181}, (dy+radius)/(2*radius))
			beam := math.Pow(math.Abs(math.Cos(a+0.6)), 18)
			c = mix(c, rgb{246, 247, 255}, beam*.7)
			if r > radius-4 {
				c = rgb{185, 169, 216}
			}
			if r < 98 {
				c = rgb{100, 85, 138}
			}
			if r < 89 {
				c = rgb{197, 181, 227}
			}
			if r < 42 {
				c = rgb{36, 30, 53}
			}
			if r < 13 {
				c = rgb{161, 142, 196}
			}
		case 2:
			colors := []rgb{{121, 110, 188}, {153, 130, 205}, {97, 165, 179}, {178, 155, 218}, {98, 126, 185}, {139, 157, 207}}
			c = rgb{22, 25, 43}
			if r > 119 {
				ring := int((r - 119) / 67)
				count := []int{8, 13, 19}[ring]
				pos := (a + math.Pi) / (2 * math.Pi) * float64(count)
				if math.Mod(r-119, 67) > 4 && pos-math.Floor(pos) > .022 {
					c = colors[(int(pos)*5+ring*2)%len(colors)]
				}
			}
			if r < 99 {
				c = rgb{183, 169, 222}
			}
			if r < 47 {
				c = rgb{24, 26, 45}
			}
			if r < 13 {
				c = rgb{183, 169, 222}
			}
		case 3:
			c = mix(rgb{239, 249, 250}, rgb{134, 185, 213}, (dy+radius)/(2*radius))
			c = mix(c, rgb{185, 164, 225}, math.Pow(math.Abs(math.Sin(a+.3)), 12)*.65)
			c = mix(c, rgb{255, 255, 255}, math.Pow(math.Abs(math.Cos(a+.5)), 22)*.85)
			if r > radius-4 {
				c = rgb{233, 249, 255}
			}
			if r < 96 {
				c = rgb{136, 158, 190}
			}
			if r < 85 {
				c = rgb{207, 228, 239}
			}
			if r < 39 {
				c = rgb{112, 135, 164}
			}
		case 4:
			c = mix(rgb{62, 64, 66}, rgb{23, 25, 28}, (dy+radius)/(2*radius))
			groove := math.Mod(r, 13)
			if groove < 1.5 && r > 108 {
				c = mix(c, rgb{139, 137, 130}, .22)
			}
			c = mix(c, rgb{137, 138, 133}, math.Pow(math.Abs(math.Cos(a+.7)), 30)*.22)
			if r < 96 {
				c = rgb{211, 185, 136}
			}
			if r < 86 {
				c = rgb{228, 205, 160}
			}
			if r < 38 {
				c = rgb{35, 35, 35}
			}
		case 5:
			c = rgb{31, 23, 48}
			for i := 0; i < 3; i++ {
				inner := 118.0 + float64(i)*68
				if r > inner && r < inner+57 {
					c = mix(rgb{168, 137, 218}, rgb{92, 181, 185}, (math.Sin(a+float64(i)*1.7)+1)/2)
				}
			}
			if r < 96 {
				c = rgb{196, 164, 230}
			}
			if r < 45 {
				c = rgb{34, 24, 51}
			}
			if r < 12 {
				c = rgb{196, 164, 230}
			}
		}
	}
	// Subtle jewel-case latch details on the two most literal cases.
	if v == 1 || v == 3 {
		if rounded(x, y, 929, 244, 9, 82, 4) < 0 || rounded(x, y, 929, 780, 9, 82, 4) < 0 {
			if v == 1 {
				c = rgb{124, 109, 154}
			} else {
				c = rgb{236, 246, 250}
			}
		}
	}
	return c, true
}
func main() {
	out := "build/icon-variants"
	if err := os.MkdirAll(out, 0755); err != nil {
		panic(err)
	}
	sheet := image.NewNRGBA(image.Rect(0, 0, 1440, 320))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{color.NRGBA{235, 236, 240, 255}}, image.Point{}, draw.Src)
	for v := 1; v <= 5; v++ {
		im := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
		for y := 0; y < 1024; y++ {
			for x := 0; x < 1024; x++ {
				var rr, gg, bb, aa float64
				for sy := 0; sy < 2; sy++ {
					for sx := 0; sx < 2; sx++ {
						c, ok := pixel(v, float64(x)+float64(sx)/2, float64(y)+float64(sy)/2)
						if ok {
							rr += c.r
							gg += c.g
							bb += c.b
							aa++
						}
					}
				}
				if aa > 0 {
					im.SetNRGBA(x, y, color.NRGBA{uint8(rr / aa), uint8(gg / aa), uint8(bb / aa), uint8(aa * 255 / 4)})
				}
			}
		}
		f, e := os.Create(fmt.Sprintf("%s/%d.png", out, v))
		if e != nil {
			panic(e)
		}
		if e = png.Encode(f, im); e != nil {
			panic(e)
		}
		f.Close()
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
				bg := uint32(235)
				a := aa / 16
				if aa > 0 {
					sheet.SetNRGBA((v-1)*288+16+x, 32+y, color.NRGBA{uint8((rr/16 + bg*(255-a)) / 255), uint8((gg/16 + 236*(255-a)) / 255), uint8((bb/16 + 240*(255-a)) / 255), 255})
				}
			}
		}
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
