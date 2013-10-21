// Copyright 2013 Sonia Keys.
// Licensed under MIT license.  See "license" file in this source tree.

// Quant provides an interface for image color quantizers.
package quant

import (
	"image"
	"image/color"
	"image/draw"
)

// Quantizer defines a color quantizer for images.
type Quantizer interface {
	// Image quantizes an image and returns a paletted image
	Image(image.Image) *image.Paletted
	// Palette quantizes an image and returns a Palette.  Note the type is
	// the Palette of this package and not image.Palette.
	Palette(image.Image) Palette
}

// Palette is a palette of color.Colors, just as color.Pallete of the standard
// library.
//
// It is defined as an interface here to allow more general implementations
// of Index, presumably ones that maintain some data structure to achieve
// performance advantages over linear search.
type Palette interface {
	Convert(color.Color) color.Color
	Index(color.Color) int
	ColorPalette() color.Palette
}

// LinearPalette implements the Palette interface with color.Palette
// and has no optimizations.
type LinearPalette struct {
	color.Palette
}

func (p LinearPalette) ColorPalette() color.Palette {
	return p.Palette
}

type Dither211 struct{}

// Dither211 satisfies draw.Drawer
var _ draw.Drawer = Dither211{}

func (d Dither211) Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
	pd, ok := dst.(*image.Paletted)
	if !ok {
		// dither211 currently requires a palette
		draw.Draw(dst, r, src, sp, draw.Src)
		return
	}
	// intersect r with both dst and src bounds, fix up sp.
	ir := r.Intersect(pd.Bounds()).
		Intersect(src.Bounds().Add(r.Min.Sub(sp)))
	if ir.Empty() {
		return // no work to do.
	}
	sp = ir.Min.Sub(r.Min)
	// get subimage of src
	sr := ir.Add(sp)
	if !sr.Eq(src.Bounds()) {
		s, ok := src.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			// dither211 currently works on whole images
			draw.Draw(dst, r, src, sp, draw.Src)
			return
		}
		src = s.SubImage(sr)
	}
	// dither211 currently returns a new image
	src = dither211(src, LinearPalette{pd.Palette})
	draw.Draw(dst, r, src, image.Point{}, draw.Src)
}

func dither211(i0 image.Image, p Palette) *image.Paletted {
	cp := p.ColorPalette()
	if len(cp) > 256 {
		return nil
	}
	b := i0.Bounds()
	pi := image.NewPaletted(b, cp)
	if b.Max.Y-b.Min.Y == 0 || b.Max.X-b.Min.X == 0 {
		return pi
	}
	// rt, dn hold diffused errors.
	// todo: rewrite with signed errors. not sure this unsigned math is valid.
	var rt color.RGBA64
	dn := make([]color.RGBA64, b.Max.X-b.Min.X+1)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		rt = dn[0]
		dn[0] = color.RGBA64{}
		for x := b.Min.X; x < b.Max.X; x++ {
			// full color from original image
			c0 := i0.At(x, y)
			r0, g0, b0, _ := c0.RGBA()
			// adjusted full color = original color + diffused error
			r0 += uint32(rt.R)
			g0 += uint32(rt.G)
			b0 += uint32(rt.B)
			if r0 > 0xffff {
				r0 = 0xffff
			}
			if g0 > 0xffff {
				g0 = 0xffff
			}
			if b0 > 0xffff {
				b0 = 0xffff
			}
			rt.R = uint16(r0)
			rt.G = uint16(g0)
			rt.B = uint16(b0)
			// nearest palette entry
			i := cp.Index(rt)
			pi.SetColorIndex(x, y, uint8(i))
			pr, pg, pb, _ := cp[i].RGBA()
			// error to be diffused = full color - palette color.
			// half of error goes right
			if uint16(pr) > rt.R {
				rt.R = 0
			} else {
				rt.R = (rt.R - uint16(pr)) / 2
			}
			if uint16(pg) > rt.G {
				rt.G = 0
			} else {
				rt.G = (rt.G - uint16(pg)) / 2
			}
			if uint16(pb) > rt.B {
				rt.B = 0
			} else {
				rt.B = (rt.B - uint16(pb)) / 2
			}
			// half goes down
			dn[x+1].R = rt.R / 2
			dn[x+1].G = rt.G / 2
			dn[x+1].B = rt.B / 2
			dn[x].R += dn[x+1].R
			dn[x].G += dn[x+1].G
			dn[x].B += dn[x+1].B
		}
	}
	return pi
}
