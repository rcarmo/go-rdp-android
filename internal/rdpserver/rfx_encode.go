package rdpserver

import (
	"encoding/binary"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const rfxTileSize = 64
const rfxTilePixels = rfxTileSize * rfxTileSize

const (
	rfxOffsetHL1 = 0
	rfxOffsetLH1 = 1024
	rfxOffsetHH1 = 2048
	rfxOffsetHL2 = 3072
	rfxOffsetLH2 = 3328
	rfxOffsetHH2 = 3584
	rfxOffsetHL3 = 3840
	rfxOffsetLH3 = 3904
	rfxOffsetHH3 = 3968
	rfxOffsetLL3 = 4032
)

type rfxYCoCgTile struct {
	Y  [rfxTilePixels]int16
	Co [rfxTilePixels]int16
	Cg [rfxTilePixels]int16
}

type rfxQuant struct {
	Values [10]uint8
}

var rfxDefaultQuant = rfxQuant{Values: [10]uint8{6, 6, 6, 6, 7, 7, 8, 8, 8, 9}}

func parseRFXQuant(raw []byte) (rfxQuant, bool) {
	var q rfxQuant
	if len(raw) != 5 {
		return q, false
	}
	for i := 0; i < 5; i++ {
		q.Values[i*2] = raw[i] >> 4
		q.Values[i*2+1] = raw[i] & 0x0f
	}
	return q, true
}

func quantizeRFXComponent(coeff *[rfxTilePixels]int16, q rfxQuant) {
	// Subband order follows MS-RDPRFX quant order:
	// LL3, HL3, LH3, HH3, HL2, LH2, HH2, HL1, LH1, HH1.
	rfxQuantizeRect(coeff, 0, 0, 8, 8, q.Values[0])
	rfxQuantizeRect(coeff, 8, 0, 8, 8, q.Values[1])
	rfxQuantizeRect(coeff, 0, 8, 8, 8, q.Values[2])
	rfxQuantizeRect(coeff, 8, 8, 8, 8, q.Values[3])

	rfxQuantizeRect(coeff, 16, 0, 16, 16, q.Values[4])
	rfxQuantizeRect(coeff, 0, 16, 16, 16, q.Values[5])
	rfxQuantizeRect(coeff, 16, 16, 16, 16, q.Values[6])

	rfxQuantizeRect(coeff, 32, 0, 32, 32, q.Values[7])
	rfxQuantizeRect(coeff, 0, 32, 32, 32, q.Values[8])
	rfxQuantizeRect(coeff, 32, 32, 32, 32, q.Values[9])
}

func rfxQuantizeRect(coeff *[rfxTilePixels]int16, x0, y0, w, h int, shift uint8) {
	if shift == 0 {
		return
	}
	if shift > 15 {
		shift = 15
	}
	for y := 0; y < h; y++ {
		row := (y0+y)*rfxTileSize + x0
		for x := 0; x < w; x++ {
			coeff[row+x] = coeff[row+x] >> shift
		}
	}
}

func forwardRFXDWT53(coeff *[rfxTilePixels]int16) {
	var tmp [rfxTilePixels]int32
	for i := 0; i < rfxTilePixels; i++ {
		tmp[i] = int32(coeff[i])
	}
	for levelSize := rfxTileSize; levelSize >= 16; levelSize >>= 1 {
		rfxDWT53Level(&tmp, levelSize)
	}
	for i := 0; i < rfxTilePixels; i++ {
		coeff[i] = int16(tmp[i])
	}
}

func rfxDWT53Level(buf *[rfxTilePixels]int32, n int) {
	var line [rfxTileSize]int32
	for y := 0; y < n; y++ {
		base := y * rfxTileSize
		copy(line[:n], buf[base:base+n])
		rfxDWT53Line(line[:n])
		copy(buf[base:base+n], line[:n])
	}
	for x := 0; x < n; x++ {
		for y := 0; y < n; y++ {
			line[y] = buf[y*rfxTileSize+x]
		}
		rfxDWT53Line(line[:n])
		for y := 0; y < n; y++ {
			buf[y*rfxTileSize+x] = line[y]
		}
	}
}

func serializeRFXComponentForRLGR(coeff *[rfxTilePixels]int16) [rfxTilePixels]int16 {
	var out [rfxTilePixels]int16
	copyRFXBand2D(&out, rfxOffsetHL1, coeff, 32, 0, 32)
	copyRFXBand2D(&out, rfxOffsetLH1, coeff, 0, 32, 32)
	copyRFXBand2D(&out, rfxOffsetHH1, coeff, 32, 32, 32)

	copyRFXBand2D(&out, rfxOffsetHL2, coeff, 16, 0, 16)
	copyRFXBand2D(&out, rfxOffsetLH2, coeff, 0, 16, 16)
	copyRFXBand2D(&out, rfxOffsetHH2, coeff, 16, 16, 16)

	copyRFXBand2D(&out, rfxOffsetHL3, coeff, 8, 0, 8)
	copyRFXBand2D(&out, rfxOffsetLH3, coeff, 0, 8, 8)
	copyRFXBand2D(&out, rfxOffsetHH3, coeff, 8, 8, 8)
	copyRFXBand2D(&out, rfxOffsetLL3, coeff, 0, 0, 8)
	differentialEncodeRFXLL3(out[rfxOffsetLL3 : rfxOffsetLL3+64])
	return out
}

func copyRFXBand2D(dst *[rfxTilePixels]int16, dstOff int, src *[rfxTilePixels]int16, x0, y0, size int) {
	i := dstOff
	for y := 0; y < size; y++ {
		row := (y0+y)*rfxTileSize + x0
		for x := 0; x < size; x++ {
			dst[i] = src[row+x]
			i++
		}
	}
}

func differentialEncodeRFXLL3(ll3 []int16) {
	if len(ll3) == 0 {
		return
	}
	prev := ll3[0]
	for i := 1; i < len(ll3); i++ {
		curr := ll3[i]
		ll3[i] = curr - prev
		prev = curr
	}
}

func rfxDWT53Line(line []int32) {
	n := len(line)
	if n < 2 || n%2 != 0 {
		return
	}
	half := n / 2
	var lo [rfxTileSize]int32
	var hi [rfxTileSize]int32
	for i := 0; i < half; i++ {
		lo[i] = line[i*2]
		hi[i] = line[i*2+1]
	}
	for i := 0; i < half; i++ {
		left := lo[i]
		right := lo[i]
		if i+1 < half {
			right = lo[i+1] // #nosec G602 -- guarded by i+1 < half
		}
		hi[i] -= (left + right) >> 1
	}
	for i := 0; i < half; i++ {
		left := hi[i]
		if i > 0 {
			left = hi[i-1] // #nosec G602 -- guarded by i > 0
		}
		right := hi[i]
		lo[i] += (left + right + 2) >> 2
	}
	copy(line[:half], lo[:half])
	copy(line[half:], hi[:half])
}

const (
	rfxBlockTypeSync          = 0xCCC0
	rfxBlockTypeCodecVersions = 0xCCC1
	rfxBlockTypeChannels      = 0xCCC2
	rfxBlockTypeContext       = 0xCCC3
	rfxBlockTypeFrameBegin    = 0xCCC4
	rfxBlockTypeFrameEnd      = 0xCCC5
	rfxBlockTypeRegion        = 0xCCC6
	rfxBlockTypeTileset       = 0xCAC2
	rfxBlockTypeTile          = 0xCAC3

	rfxSyncMagic = 0xCACCACCA
	rfxVersion10 = 0x0100
)

type rfxRLGRMode int

const (
	rfxRLGR1 rfxRLGRMode = 1
	rfxRLGR3 rfxRLGRMode = 3
)

type rfxBitWriter struct {
	buf   []byte
	acc   uint8
	nbits uint8
}

func (w *rfxBitWriter) writeBit(bit uint8) {
	w.acc = (w.acc << 1) | (bit & 1)
	w.nbits++
	if w.nbits == 8 {
		w.buf = append(w.buf, w.acc)
		w.acc = 0
		w.nbits = 0
	}
}

func (w *rfxBitWriter) writeBits(v uint32, n uint32) {
	for i := int(n) - 1; i >= 0; i-- {
		w.writeBit(uint8((v >> i) & 1))
	}
}

func (w *rfxBitWriter) writeUnaryZerosThenOne(zeros int) {
	for i := 0; i < zeros; i++ {
		w.writeBit(0)
	}
	w.writeBit(1)
}

func (w *rfxBitWriter) writeUnaryOnesThenZero(ones int) {
	for i := 0; i < ones; i++ {
		w.writeBit(1)
	}
	w.writeBit(0)
}

func (w *rfxBitWriter) bytes() []byte {
	out := make([]byte, len(w.buf))
	copy(out, w.buf)
	if w.nbits > 0 {
		out = append(out, w.acc<<(8-w.nbits))
	}
	if len(out) == 0 {
		return []byte{0}
	}
	return out
}

func encodeRFXRLGR(coeff []int16, mode rfxRLGRMode) ([]byte, bool) {
	if len(coeff) != rfxTilePixels {
		return nil, false
	}
	const (
		kpMax = 80
		lsgr  = 3
		upGR  = 4
		dnGR  = 6
		uqGR  = 3
		dqGR  = 3
	)
	k, kp := uint32(1), uint32(8)
	kr, krp := uint32(1), uint32(8)
	var bw rfxBitWriter
	idx := 0
	for idx < len(coeff) {
		if k != 0 {
			run := 0
			for idx+run < len(coeff) && coeff[idx+run] == 0 {
				run++
			}
			for run >= (1 << k) {
				bw.writeBit(0)
				run -= 1 << k
				kp += upGR
				if kp > kpMax {
					kp = kpMax
				}
				k = kp >> lsgr
			}
			bw.writeBit(1)
			if k > 0 {
				bw.writeBits(uint32(run), k)
			}
			idx += run
			if idx >= len(coeff) {
				break
			}
			v := coeff[idx]
			if v < 0 {
				bw.writeBit(1)
			} else {
				bw.writeBit(0)
			}
			mag := absCoeffMagnitudeMinusOne(v)
			nIdx := mag >> kr
			bw.writeUnaryOnesThenZero(int(nIdx))
			if kr > 0 {
				bw.writeBits(mag&((1<<kr)-1), kr)
			}
			if nIdx == 0 {
				if krp >= 2 {
					krp -= 2
				} else {
					krp = 0
				}
			} else if nIdx > 1 {
				krp += nIdx
				if krp > kpMax {
					krp = kpMax
				}
			}
			kr = krp >> lsgr
			if kp >= dnGR {
				kp -= dnGR
			} else {
				kp = 0
			}
			k = kp >> lsgr
			idx++
			continue
		}

		if mode == rfxRLGR1 {
			mag := rlgrInterleavedMag(coeff[idx])
			nIdx := mag >> kr
			bw.writeUnaryOnesThenZero(int(nIdx))
			if kr > 0 {
				bw.writeBits(mag&((1<<kr)-1), kr)
			}
			if nIdx == 0 {
				if krp >= 2 {
					krp -= 2
				} else {
					krp = 0
				}
			} else if nIdx > 1 {
				krp += nIdx
				if krp > kpMax {
					krp = kpMax
				}
			}
			kr = krp >> lsgr
			if mag == 0 {
				kp += uqGR
				if kp > kpMax {
					kp = kpMax
				}
			} else {
				if kp >= dqGR {
					kp -= dqGR
				} else {
					kp = 0
				}
			}
			k = kp >> lsgr
			idx++
			continue
		}

		v1 := uint32(0)
		if idx < len(coeff) {
			v1 = rlgrInterleavedMag(coeff[idx])
		}
		v2 := uint32(0)
		if idx+1 < len(coeff) {
			v2 = rlgrInterleavedMag(coeff[idx+1])
		}
		code := v1 + v2
		nIdx := code >> kr
		bw.writeUnaryOnesThenZero(int(nIdx))
		if kr > 0 {
			bw.writeBits(code&((1<<kr)-1), kr)
		}
		nbits := bitLen32(code)
		if nbits > 0 {
			if v1 > code {
				return nil, false
			}
			bw.writeBits(v1, uint32(nbits))
		}
		if nIdx == 0 {
			if krp >= 2 {
				krp -= 2
			} else {
				krp = 0
			}
		} else if nIdx > 1 {
			krp += nIdx
			if krp > kpMax {
				krp = kpMax
			}
		}
		kr = krp >> lsgr
		if v1 != 0 && v2 != 0 {
			if kp >= 2*dqGR {
				kp -= 2 * dqGR
			} else {
				kp = 0
			}
		} else if v1 == 0 && v2 == 0 {
			kp += 2 * uqGR
			if kp > kpMax {
				kp = kpMax
			}
		}
		k = kp >> lsgr
		idx += 2
	}
	return bw.bytes(), true
}

func rlgrInterleavedMag(v int16) uint32 {
	if v == 0 {
		return 0
	}
	if v < 0 {
		return uint32((int32(-v) << 1) - 1)
	}
	return uint32(int32(v) << 1)
}

func absCoeffMagnitudeMinusOne(v int16) uint32 {
	iv := int32(v)
	if iv < 0 {
		iv = -iv
	}
	if iv <= 0 {
		return 0
	}
	return uint32(iv - 1)
}

func bitLen32(v uint32) int {
	n := 0
	for v > 0 {
		n++
		v >>= 1
	}
	return n
}

func buildRFXTileBlock(tileX, tileY uint16, yData, cbData, crData []byte) ([]byte, bool) {
	if len(yData) == 0 || len(cbData) == 0 || len(crData) == 0 {
		return nil, false
	}
	if len(yData) > 0xffff || len(cbData) > 0xffff || len(crData) > 0xffff {
		return nil, false
	}
	blockLen := 19 + len(yData) + len(cbData) + len(crData)
	if blockLen > rfxMaxEncodedPayloadLen {
		return nil, false
	}
	out := make([]byte, blockLen)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeTile)
	binary.LittleEndian.PutUint32(out[2:6], uint32(blockLen))
	out[6] = 0 // quantIdxY
	out[7] = 0 // quantIdxCb
	out[8] = 0 // quantIdxCr
	binary.LittleEndian.PutUint16(out[9:11], tileX)
	binary.LittleEndian.PutUint16(out[11:13], tileY)
	binary.LittleEndian.PutUint16(out[13:15], uint16(len(yData)))
	binary.LittleEndian.PutUint16(out[15:17], uint16(len(cbData)))
	binary.LittleEndian.PutUint16(out[17:19], uint16(len(crData)))
	off := 19
	off += copy(out[off:], yData)
	off += copy(out[off:], cbData)
	copy(out[off:], crData)
	return out, true
}

func buildRFXEncodedTile(src frame.Frame, tileX, tileY int, quantRaw []byte) ([]byte, bool) {
	if tileX%rfxTileSize != 0 || tileY%rfxTileSize != 0 {
		return nil, false
	}
	tile, ok := buildRFXYCoCgTile(src, tileX, tileY)
	if !ok {
		return nil, false
	}
	q := rfxDefaultQuant
	if len(quantRaw) > 0 {
		parsed, parsedOK := parseRFXQuant(quantRaw)
		if !parsedOK {
			return nil, false
		}
		q = parsed
	}

	y := tile.Y
	co := tile.Co
	cg := tile.Cg
	forwardRFXDWT53(&y)
	forwardRFXDWT53(&co)
	forwardRFXDWT53(&cg)
	quantizeRFXComponent(&y, q)
	quantizeRFXComponent(&co, q)
	quantizeRFXComponent(&cg, q)
	yp := serializeRFXComponentForRLGR(&y)
	cop := serializeRFXComponentForRLGR(&co)
	cgp := serializeRFXComponentForRLGR(&cg)
	yData, ok := encodeRFXRLGR(yp[:], rfxRLGR1)
	if !ok {
		return nil, false
	}
	coData, ok := encodeRFXRLGR(cop[:], rfxRLGR3)
	if !ok {
		return nil, false
	}
	cgData, ok := encodeRFXRLGR(cgp[:], rfxRLGR3)
	if !ok {
		return nil, false
	}
	return buildRFXTileBlock(uint16(tileX/rfxTileSize), uint16(tileY/rfxTileSize), yData, coData, cgData)
}

func buildRFXMessageSingleTile(src frame.Frame, width, height int, frameID uint32, tileX, tileY int, quantRaw []byte) ([]byte, bool) {
	if width <= 0 || height <= 0 || width > 0xffff || height > 0xffff {
		return nil, false
	}
	tileBlock, ok := buildRFXEncodedTile(src, tileX, tileY, quantRaw)
	if !ok {
		return nil, false
	}
	blocks := [][]byte{
		buildRFXSyncBlock(),
		buildRFXCodecVersionsBlock(),
		buildRFXChannelsBlock(uint16(width), uint16(height)),
		buildRFXContextBlock(uint16(width), uint16(height)),
		buildRFXFrameBeginBlock(frameID),
		buildRFXRegionBlock(uint16(tileX), uint16(tileY), rfxTileSize, rfxTileSize),
		buildRFXTilesetBlock(tileBlock, quantRaw),
		buildRFXFrameEndBlock(),
	}
	total := 0
	for _, b := range blocks {
		total += len(b)
	}
	if total <= 0 || total > rfxMaxEncodedPayloadLen {
		return nil, false
	}
	out := make([]byte, 0, total)
	for _, b := range blocks {
		out = append(out, b...)
	}
	return out, true
}

func buildRFXSyncBlock() []byte {
	out := make([]byte, 12)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeSync)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[6:10], rfxSyncMagic)
	binary.LittleEndian.PutUint16(out[10:12], rfxVersion10)
	return out
}

func buildRFXCodecVersionsBlock() []byte {
	out := make([]byte, 10)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeCodecVersions)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	out[6] = 1
	out[7] = 1
	binary.LittleEndian.PutUint16(out[8:10], rfxVersion10)
	return out
}

func buildRFXChannelsBlock(width, height uint16) []byte {
	out := make([]byte, 13)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeChannels)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	out[6] = 1
	out[7] = 0
	binary.LittleEndian.PutUint16(out[8:10], width)
	binary.LittleEndian.PutUint16(out[10:12], height)
	out[12] = 0
	return out
}

func buildRFXContextBlock(width, height uint16) []byte {
	out := make([]byte, 13)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeContext)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	out[6] = 0
	binary.LittleEndian.PutUint16(out[7:9], rfxTileSize)
	binary.LittleEndian.PutUint16(out[9:11], width)
	binary.LittleEndian.PutUint16(out[11:13], height)
	return out
}

func buildRFXFrameBeginBlock(frameID uint32) []byte {
	out := make([]byte, 14)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeFrameBegin)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	binary.LittleEndian.PutUint32(out[6:10], frameID)
	binary.LittleEndian.PutUint16(out[10:12], 1)
	binary.LittleEndian.PutUint16(out[12:14], 0)
	return out
}

func buildRFXRegionBlock(x, y uint16, w, h int) []byte {
	out := make([]byte, 17)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeRegion)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	out[6] = 1
	binary.LittleEndian.PutUint16(out[7:9], 1)
	binary.LittleEndian.PutUint16(out[9:11], x)
	binary.LittleEndian.PutUint16(out[11:13], y)
	binary.LittleEndian.PutUint16(out[13:15], uint16(w))
	binary.LittleEndian.PutUint16(out[15:17], uint16(h))
	return out
}

func buildRFXTilesetBlock(tileBlock, quantRaw []byte) []byte {
	quant := quantRaw
	if len(quant) == 0 {
		quant = []byte{0x66, 0x66, 0x77, 0x88, 0x89}
	}
	out := make([]byte, 0, 22+len(quant)+len(tileBlock))
	hdr := make([]byte, 20)
	binary.LittleEndian.PutUint16(hdr[0:2], rfxBlockTypeTileset)
	binary.LittleEndian.PutUint16(hdr[6:8], 0xCAC1)
	binary.LittleEndian.PutUint16(hdr[8:10], 0)
	binary.LittleEndian.PutUint16(hdr[10:12], 0)
	hdr[12] = 1
	hdr[13] = rfxTileSize
	binary.LittleEndian.PutUint16(hdr[14:16], 1)
	binary.LittleEndian.PutUint32(hdr[16:20], uint32(len(tileBlock)))
	out = append(out, hdr...)
	out = append(out, quant...)
	out = append(out, tileBlock...)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	return out
}

func buildRFXFrameEndBlock() []byte {
	out := make([]byte, 6)
	binary.LittleEndian.PutUint16(out[0:2], rfxBlockTypeFrameEnd)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out)))
	return out
}

func buildRFXYCoCgTile(src frame.Frame, tileX, tileY int) (rfxYCoCgTile, bool) {
	var out rfxYCoCgTile
	stride, ok := normalizedFrameStride(src)
	if !ok || tileX < 0 || tileY < 0 || tileX > src.Width || tileY > src.Height {
		return out, false
	}
	if tileX+rfxTileSize > src.Width || tileY+rfxTileSize > src.Height {
		return out, false
	}
	for y := 0; y < rfxTileSize; y++ {
		row := (tileY + y) * stride
		for x := 0; x < rfxTileSize; x++ {
			si := row + (tileX+x)*4
			di := y*rfxTileSize + x
			var r, g, b int16
			switch src.Format {
			case frame.PixelFormatRGBA8888:
				r = int16(src.Data[si+0])
				g = int16(src.Data[si+1])
				b = int16(src.Data[si+2])
			case frame.PixelFormatBGRA8888:
				b = int16(src.Data[si+0])
				g = int16(src.Data[si+1])
				r = int16(src.Data[si+2])
			default:
				return rfxYCoCgTile{}, false
			}
			co := r - b
			t := b + (co >> 1)
			cg := g - t
			yv := t + (cg >> 1)
			out.Y[di] = yv
			out.Co[di] = co
			out.Cg[di] = cg
		}
	}
	return out, true
}
