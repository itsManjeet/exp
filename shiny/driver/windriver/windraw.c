// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "_cgo_export.h"
#include "windriver.h"

static HBITMAP mkbitmap(HDC dc, RECT *r, VOID **ppvBits) {
	BITMAPINFO bi;
	LONG dx, dy;
	HBITMAP bitmap;

	dx = r->right - r->left;
	dy = r->bottom - r->top;

	ZeroMemory(&bi, sizeof (BITMAPINFO));
	bi.bmiHeader.biSize = sizeof (BITMAPINFOHEADER);
	bi.bmiHeader.biWidth = (LONG) dx;
	bi.bmiHeader.biHeight = -((LONG) dy);			// negative height to force top-down drawing
	bi.bmiHeader.biPlanes = 1;
	bi.bmiHeader.biBitCount = 32;
	bi.bmiHeader.biCompression = BI_RGB;
	bi.bmiHeader.biSizeImage = (DWORD) (dx * dy * 4);

	bitmap = CreateDIBSection(dc, &bi, DIB_RGB_COLORS, ppvBits, 0, 0);
	if (bitmap == NULL) {
		// TODO(andlabs)
	}
	return bitmap;
}

static void blend(HDC dc, HBITMAP bitmap, RECT *dr, LONG sdx, LONG sdy, BYTE op) {
	HDC compatibleDC;
	HBITMAP prevBitmap;
	BLENDFUNCTION blendfunc;

	compatibleDC = CreateCompatibleDC(dc);
	if (compatibleDC == NULL) {
		// TODO(andlabs)
	}
	prevBitmap = SelectObject(compatibleDC, bitmap);
	if (prevBitmap == NULL) {
		// TODO(andlabs)
	}

	// Note for Go conversion: the BLENDFUNCTION must be
	// passed as a 32-bit value, not as a pointer.
	if (op == 0) {		// draw.Over
		ZeroMemory(&blendfunc, sizeof (BLENDFUNCTION));
		blendfunc.BlendOp = AC_SRC_OVER;
		blendfunc.BlendFlags = 0;
		blendfunc.SourceConstantAlpha = 255;		// only use per-pixel alphas
		blendfunc.AlphaFormat = AC_SRC_ALPHA;	// premultiplied
		if (AlphaBlend(dc, dr->left, dr->top,
			dr->right - dr->left, dr->bottom - dr->top,
			compatibleDC, 0, 0, sdx, sdy,
			blendfunc) == FALSE) {
			// TODO
		}
	} else {		// draw.Src
		if (BitBlt(dc, dr->left, dr->top,
			dr->right - dr->left, dr->bottom - dr->top,
			compatibleDC, 0, 0,
			SRCCOPY) == 0) {
			// TODO
		}
	}

	// TODO(andlabs): error check these?
	SelectObject(compatibleDC, prevBitmap);
	DeleteDC(compatibleDC);
}

// TODO(andlabs): Upload

void fill(HDC dc, RECT r, COLORREF color, BYTE op) {
	HBITMAP bitmap;
	VOID *ppvBits;
	uint32_t *colors;
	LONG i, nPixels;

	bitmap = mkbitmap(dc, &r, &ppvBits);
	colors = (uint32_t *) ppvBits;
	nPixels = (r.right - r.left) * (r.bottom - r.top);
	for (i = 0; i < nPixels; i++) {
		*colors = color;
		colors++;
	}
	blend(dc, bitmap, &r, r.right - r.left, r.bottom - r.top, op);
	// TODO(andlabs): check errors?
	DeleteObject(bitmap);
}

// TODO(andlabs): Draw
