// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package windriver

import (
	"image"
	"syscall"
	"unsafe"
)

func mkbitmap(dc syscall.Handle, r *_RECT) (syscall.Handle, *byte, error) {
	dx := r.Right - r.Left
	dy := r.Bottom - r.Top

	var bi _BITMAPINFO
	bi.Header.Size = uint32(unsafe.Sizeof(bi.Header))
	bi.Header.Width = dx
	bi.Header.Height = -dy // negative height to force top-down drawing
	bi.Header.Planes = 1
	bi.Header.BitCount = 32
	bi.Header.Compression = _BI_RGB
	bi.Header.SizeImage = uint32(dx * dy * 4)

	var ppvBits *byte
	bitmap, err := _CreateDIBSection(dc, &bi, _DIB_RGB_COLORS, &ppvBits, 0, 0)
	if err != nil {
		return 0, nil, err
	}
	return bitmap, ppvBits, nil
}

func blend(dc syscall.Handle, bitmap syscall.Handle, dr *_RECT, sdx int32, sdy int32) error {
	compatibleDC, err := _CreateCompatibleDC(dc)
	if err != nil {
		return err
	}
	prevBitmap, err := _SelectObject(compatibleDC, bitmap)
	if err != nil {
		return err
	}

	var blendfunc _BLENDFUNCTION
	blendfunc.BlendOp = _AC_SRC_OVER
	blendfunc.BlendFlags = 0
	blendfunc.SourceConstantAlpha = 255   // only use per-pixel alphas
	blendfunc.AlphaFormat = _AC_SRC_ALPHA // premultiplied
	err = _AlphaBlend(dc, dr.Left, dr.Top,
		dr.Right-dr.Left, dr.Bottom-dr.Top,
		compatibleDC, 0, 0, sdx, sdy,
		blendfunc.ToUintptr())
	if err != nil {
		return err
	}

	_, err = _SelectObject(compatibleDC, prevBitmap)
	if err != nil {
		return err
	}
	return _DeleteDC(compatibleDC)
}

func upload(dc syscall.Handle, rgba *image.RGBA, tdx int32, tdy int32) error {
	dx, dy := int32(rgba.Rect.Max.X-rgba.Rect.Min.X), int32(rgba.Rect.Max.Y-rgba.Rect.Min.Y)
	rect := _RECT{
		Left:   0,
		Top:    0,
		Right:  dx,
		Bottom: dy,
	}
	bitmap, bitvalues, err := mkbitmap(dc, &rect)
	if err != nil {
		return err
	}
	defer _DeleteObject(bitmap)

	// Convert bitmap to Windows DIB format
	buf := uintptr(unsafe.Pointer(bitvalues))
	write := func(b byte) {
		*(*byte)(unsafe.Pointer(buf)) = b
		buf++
	}
	for y := 0; y < int(dy); y++ {
		for x := 0; x < int(dx); x++ {
			pix := rgba.Pix[rgba.PixOffset(x, y):]
			r, g, b, a := pix[0], pix[1], pix[2], pix[3]
			// 0xaaRRGGBB
			// see: https://msdn.microsoft.com/en-us/library/windows/desktop/dd183376%28v=vs.85%29.aspx
			// alpha: http://stackoverflow.com/a/1343551/98528
			// BUT: x86/x64 is little-endian, so reverse...
			write(b)
			write(g)
			write(r)
			write(a)
		}
	}

	tr := _RECT{
		Left:   tdx,
		Top:    tdy,
		Right:  tdx + dx,
		Bottom: tdy + dy,
	}
	return blend(dc, bitmap, &tr, dx, dy)
}

func fillSrc(dc syscall.Handle, r *_RECT, color _COLORREF) error {
	// COLORREF is 0x00BBGGRR; color is 0xAARRGGBB
	color = _RGB(byte((color >> 16)), byte((color >> 8)), byte(color))
	brush, err := _CreateSolidBrush(color)
	if err != nil {
		return err
	}
	err = _FillRect(dc, r, brush)
	if err != nil {
		return err
	}
	return _DeleteObject(brush)
}

func fillOver(dc syscall.Handle, r *_RECT, color _COLORREF) error {
	// AlphaBlend will stretch the input image (using StretchBlt's
	// COLORONCOLOR mode) to fill the output rectangle. Testing
	// this shows that the result appears to be the same as if we had
	// used a MxN bitmap instead.
	oneByOne := _RECT{
		Left:   0,
		Top:    0,
		Right:  1,
		Bottom: 1,
	}
	bitmap, bitvalues, err := mkbitmap(dc, &oneByOne)
	if err != nil {
		return err
	}
	*(*_COLORREF)(unsafe.Pointer(bitvalues)) = color
	err = blend(dc, bitmap, r, 1, 1)
	if err != nil {
		return err
	}
	return _DeleteObject(bitmap)
}

// TODO(andlabs): Draw
