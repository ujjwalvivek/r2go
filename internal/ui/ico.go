package ui

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
)

// decodeICO decodes the largest image in a Windows .ico file. Entries are
// either embedded PNGs or 32- or 24-bit DIBs; other bit depths are skipped.
func decodeICO(data []byte) (image.Image, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:]) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		return nil, errors.New("not an icon file")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	bestArea, bestOffset, bestSize := -1, 0, 0
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		if entry+16 > len(data) {
			break
		}
		w, h := int(data[entry]), int(data[entry+1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		size := int(binary.LittleEndian.Uint32(data[entry+8:]))
		offset := int(binary.LittleEndian.Uint32(data[entry+12:]))
		if offset < 0 || size <= 0 || offset+size > len(data) {
			continue
		}
		if w*h > bestArea {
			bestArea, bestOffset, bestSize = w*h, offset, size
		}
	}
	if bestArea < 0 {
		return nil, errors.New("icon has no usable image")
	}
	blob := data[bestOffset : bestOffset+bestSize]
	if bytes.HasPrefix(blob, []byte("\x89PNG")) {
		return png.Decode(bytes.NewReader(blob))
	}
	return decodeDIB(blob)
}

// decodeDIB reads a bottom-up BITMAPINFOHEADER image as stored in an icon,
// whose height counts the colour rows and the AND mask together.
func decodeDIB(blob []byte) (image.Image, error) {
	if len(blob) < 40 {
		return nil, errors.New("icon image too short")
	}
	header := int(binary.LittleEndian.Uint32(blob[0:]))
	w := int(int32(binary.LittleEndian.Uint32(blob[4:])))
	h := int(int32(binary.LittleEndian.Uint32(blob[8:]))) / 2
	bpp := int(binary.LittleEndian.Uint16(blob[14:]))
	if w <= 0 || h <= 0 || w > 1024 || h > 1024 || (bpp != 32 && bpp != 24) {
		return nil, errors.New("unsupported icon image")
	}
	stride := ((w*bpp + 31) / 32) * 4
	pixels := blob[header:]
	if len(pixels) < stride*h {
		return nil, errors.New("icon image truncated")
	}
	maskStride := ((w + 31) / 32) * 4
	mask := pixels[stride*h:]
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := pixels[(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			if bpp == 32 {
				p := row[x*4:]
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: p[3]}
			} else {
				p := row[x*3:]
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: 255}
				if len(mask) >= maskStride*h && mask[(h-1-y)*maskStride+x/8]&(0x80>>(x%8)) != 0 {
					c.A = 0
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img, nil
}
