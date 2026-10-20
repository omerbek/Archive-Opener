// mkicon draws the application icon (an open cardboard box on an ochre
// tile) and writes it as a multi-size .ico file.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

type shape func(x, y float64) (color.NRGBA, bool)

func roundRect(x0, y0, x1, y1, r float64, c color.NRGBA) shape {
	return func(x, y float64) (color.NRGBA, bool) {
		if x < x0 || x > x1 || y < y0 || y > y1 {
			return c, false
		}
		cx := math.Max(x0+r, math.Min(x, x1-r))
		cy := math.Max(y0+r, math.Min(y, y1-r))
		return c, (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
	}
}

func poly(pts [][2]float64, c color.NRGBA) shape {
	return func(x, y float64) (color.NRGBA, bool) {
		in := false
		for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
			xi, yi, xj, yj := pts[i][0], pts[i][1], pts[j][0], pts[j][1]
			if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
				in = !in
			}
		}
		return c, in
	}
}

func render(size int, shapes []shape) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					// coordinates in a 0..1 design space
					x := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					var col color.NRGBA
					hit := false
					for _, s := range shapes {
						if c, ok := s(x, y); ok {
							col, hit = c, true
						}
					}
					if hit {
						fa := float64(col.A) / 255
						r += float64(col.R) * fa
						g += float64(col.G) * fa
						b += float64(col.B) * fa
						a += fa
					}
				}
			}
			n := float64(ss * ss)
			if a > 0 {
				img.SetNRGBA(px, py, color.NRGBA{
					R: uint8(r / a), G: uint8(g / a), B: uint8(b / a), A: uint8(a / n * 255),
				})
			}
		}
	}
	return img
}

func main() {
	ochre := color.NRGBA{0xA3, 0x56, 0x1C, 0xff}
	front := color.NRGBA{0xF3, 0xD9, 0xB5, 0xff}
	side := color.NRGBA{0xE2, 0xBC, 0x8A, 0xff}
	inside := color.NRGBA{0x6E, 0x3A, 0x12, 0xff}
	flap := color.NRGBA{0xEB, 0xCB, 0x9E, 0xff}
	strap := color.NRGBA{0xA3, 0x56, 0x1C, 0xff}
	arrow := color.NRGBA{0xff, 0xff, 0xff, 0xff}

	shapes := []shape{
		roundRect(0.03, 0.03, 0.97, 0.97, 0.2, ochre),
		// box opening (dark inside)
		poly([][2]float64{{0.22, 0.42}, {0.78, 0.42}, {0.72, 0.36}, {0.28, 0.36}}, inside),
		// flaps
		poly([][2]float64{{0.22, 0.42}, {0.28, 0.36}, {0.16, 0.26}, {0.10, 0.33}}, flap),
		poly([][2]float64{{0.78, 0.42}, {0.72, 0.36}, {0.84, 0.26}, {0.90, 0.33}}, side),
		// body
		poly([][2]float64{{0.22, 0.42}, {0.78, 0.42}, {0.78, 0.82}, {0.22, 0.82}}, front),
		poly([][2]float64{{0.22, 0.42}, {0.78, 0.42}, {0.78, 0.47}, {0.22, 0.47}}, side),
		// strap
		poly([][2]float64{{0.45, 0.47}, {0.55, 0.47}, {0.55, 0.82}, {0.45, 0.82}}, strap),
		// up arrow coming out of the box
		poly([][2]float64{{0.5, 0.08}, {0.64, 0.24}, {0.555, 0.24}, {0.555, 0.38}, {0.445, 0.38}, {0.445, 0.24}, {0.36, 0.24}}, arrow),
	}

	sizes := []int{16, 20, 24, 32, 40, 48, 64, 256}
	var images [][]byte
	for _, s := range sizes {
		var buf bytes.Buffer
		png.Encode(&buf, render(s, shapes))
		images = append(images, buf.Bytes())
	}
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for i, s := range sizes {
		w := byte(s)
		if s >= 256 {
			w = 0
		}
		out.Write([]byte{w, w, 0, 0})
		binary.Write(&out, binary.LittleEndian, uint16(1))
		binary.Write(&out, binary.LittleEndian, uint16(32))
		binary.Write(&out, binary.LittleEndian, uint32(len(images[i])))
		binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(images[i])
	}
	for _, im := range images {
		out.Write(im)
	}
	if err := os.WriteFile(os.Args[1], out.Bytes(), 0o644); err != nil {
		panic(err)
	}
	// also a PNG preview
	f, _ := os.Create(os.Args[1] + ".png")
	png.Encode(f, render(256, shapes))
	f.Close()
}
