package decrypter

// pesHeaderState carries parser state across TS-packet boundaries so a PES
// header that spans packets can still be measured correctly.
type pesHeaderState struct {
	finished            bool
	incompleteStartCode int // -1 = none; otherwise the 8-bit start-code byte we were inside
	trailingZeroBits    int // 0, 8, or 16 trailing zero bits at end of last buffer
	endsWithStartPrefix bool
}

func newPesHeaderState() pesHeaderState {
	return pesHeaderState{finished: true, incompleteStartCode: -1}
}

// MPEG-2 video / PES start codes.
const (
	startCodePrefix    = 0x000001
	startCodePrefixLen = 24

	scAncillary    = 0xF9
	scPicture      = 0x00
	scPictureGroup = 0xB8
	scExtension    = 0xB5
	scSequenceEnd  = 0xB7
	scSequenceHdr  = 0xB3
	scUserData     = 0xB2
)

func isPesHeader(v int) bool {
	return v == 0xBD || (v >= 0xC0 && v <= 0xEF)
}

func isSlice(v int) bool {
	return v >= 0x01 && v <= 0xAF
}

// parsePesHeaderLength measures the unencrypted PES + MPEG-video header bytes
// at the start of buf, given any continuation state from the previous packet.
// The state struct is mutated for the next call. The returned length may
// exceed len(buf), indicating the headers extend into the next TS packet.
func parsePesHeaderLength(buf []byte, state *pesHeaderState) int {
	p := &pesParser{
		buf:                   buf,
		priorTrailingZeroBits: state.trailingZeroBits,
	}
	p.run(state.incompleteStartCode, state.endsWithStartPrefix)

	state.finished = p.trailingZeroBits == 0 && !p.endsWithStartPrefix &&
		(p.incompleteStartCode == -1 || p.incompleteStartCode == scUserData)
	state.incompleteStartCode = p.incompleteStartCode
	state.trailingZeroBits = p.trailingZeroBits
	state.endsWithStartPrefix = p.endsWithStartPrefix

	if p.scrambled {
		return 0
	}
	bytes := p.bitLength / 8
	if p.bitLength%8 != 0 {
		bytes++
	}
	return bytes
}

type pesParser struct {
	buf                   []byte
	bitPos                int
	bitLength             int
	scrambled             bool
	priorTrailingZeroBits int
	trailingZeroBits      int
	endsWithStartPrefix   bool
	incompleteStartCode   int
	currentStartCode      int
	done                  bool // set true to stop the dispatch loop
}

func (p *pesParser) run(carryoverStartCode int, lastEndedWithPrefix bool) {
	p.incompleteStartCode = -1

	var startCodeValue int
	if carryoverStartCode != -1 {
		// We were already mid-header at the end of the previous packet; resume.
		startCodeValue = carryoverStartCode
	} else if lastEndedWithPrefix {
		// Previous packet ended right at the prefix; read just the 8-bit value.
		v, ok := p.getAndAdvanceBits(8)
		if !ok {
			p.endsWithStartPrefix = true
			return
		}
		startCodeValue = v
	} else {
		// Find the next start code prefix from scratch.
		if !p.nextStartCode() {
			return
		}
		if _, ok := p.getAndAdvanceBits(startCodePrefixLen - p.priorTrailingZeroBits); !ok {
			return
		}
		p.priorTrailingZeroBits = 0
		v, ok := p.getAndAdvanceBits(8)
		if !ok {
			p.endsWithStartPrefix = true
			return
		}
		startCodeValue = v
	}

	for !p.done {
		p.currentStartCode = startCodeValue
		p.dispatch(startCodeValue)
		if p.done {
			return
		}

		if !p.nextStartCode() {
			return
		}
		if _, ok := p.getAndAdvanceBits(startCodePrefixLen); !ok {
			return
		}
		v, ok := p.getAndAdvanceBits(8)
		if !ok {
			p.endsWithStartPrefix = true
			return
		}
		startCodeValue = v
	}
}

func (p *pesParser) dispatch(sc int) {
	switch {
	case sc == scAncillary, sc == scSequenceEnd:
		// no payload
	case sc == scExtension:
		p.parseExtensionHeader()
	case isPesHeader(sc):
		p.parsePesHeader()
	case sc == scPicture:
		p.parsePictureHeader()
	case sc == scPictureGroup:
		p.advanceBits(27)
	case sc == scSequenceHdr:
		p.parseSequenceHeader()
	case isSlice(sc):
		// Slice data is encrypted: back up the 32 bits we read for prefix+code and stop.
		p.rewind(32)
		p.done = true
	case sc == scUserData:
		p.parseUserData()
	default:
		// Unknown — rewind and stop.
		p.rewind(32)
		p.done = true
	}
}

func (p *pesParser) parsePesHeader() {
	p.advanceBits(16) // packet length
	if isPesHeader(p.currentStartCode) {
		p.parsePesHeaderExtension()
	}
}

func (p *pesParser) parsePesHeaderExtension() {
	p.advanceBits(2)
	if v, _ := p.getAndAdvanceBits(2); v > 0 {
		p.scrambled = true
	}
	p.advanceBits(12)
	dataLen, ok := p.getAndAdvanceBits(8)
	if !ok {
		p.markIncomplete()
		return
	}
	if !p.skipBytes(dataLen) {
		p.markIncomplete()
	}
}

func (p *pesParser) parsePictureHeader() {
	p.advanceBits(10)
	frameType, ok := p.getAndAdvanceBits(3)
	if !ok {
		p.markIncomplete()
		return
	}
	extra := 16
	if frameType == 2 || frameType == 3 {
		extra += 4
	}
	if frameType == 3 {
		extra += 4
	}
	p.advanceBits(extra)
	// Picture header may end with stuffing bytes. Consume them, but if we run
	// out of buffer, treat the remainder of the buffer as part of the header.
	for {
		v, ok := p.getAndAdvanceBits(1)
		if !ok {
			p.bitPos = len(p.buf) * 8
			p.bitLength = len(p.buf) * 8
			return
		}
		if v != 1 {
			break
		}
		if _, ok := p.getAndAdvanceBits(8); !ok {
			p.bitPos = len(p.buf) * 8
			p.bitLength = len(p.buf) * 8
			return
		}
	}
}

func (p *pesParser) parseSequenceHeader() {
	p.advanceBits(62)
	hasIQM, ok := p.getAndAdvanceBits(1)
	if !ok {
		p.markIncomplete()
		return
	}
	if hasIQM == 1 {
		if !p.skipBytes(64) {
			p.markIncomplete()
			return
		}
	}
	hasNIQM, ok := p.getAndAdvanceBits(1)
	if !ok {
		p.markIncomplete()
		return
	}
	if hasNIQM == 1 {
		if !p.skipBytes(64) {
			p.markIncomplete()
		}
	}
}

func (p *pesParser) parseExtensionHeader() {
	t, ok := p.getAndAdvanceBits(4)
	if !ok {
		p.markIncomplete()
		return
	}
	switch t {
	case 1: // sequence extension
		p.advanceBits(44)
	case 2: // sequence display extension
		p.advanceBits(3)
		hasColor, ok := p.getAndAdvanceBits(1)
		if !ok {
			p.markIncomplete()
			return
		}
		bits := 29
		if hasColor == 1 {
			bits += 24
		}
		p.advanceBits(bits)
	case 8: // picture coding extension
		p.advanceBits(29)
		composite, ok := p.getAndAdvanceBits(1)
		if !ok {
			p.markIncomplete()
			return
		}
		if composite == 1 {
			p.advanceBits(20)
		}
	default:
		p.rewind(32)
		p.done = true
	}
}

func (p *pesParser) parseUserData() {
	for {
		v, ok := p.peekBits(startCodePrefixLen)
		if !ok {
			p.markIncomplete()
			return
		}
		if v == startCodePrefix {
			return
		}
		p.advanceBits(8)
	}
}

// nextStartCode byte-aligns and searches for the next 0x000001 prefix. The bit
// position will be left pointing AT the prefix's first bit when found.
func (p *pesParser) nextStartCode() bool {
	if !p.byteAlign() {
		return false
	}
	startCodeLength := 0
	v, ok := p.peekBits(startCodePrefixLen - p.priorTrailingZeroBits)
	if !ok {
		p.computeTrailingZeros()
		return false
	}
	for v == 0 {
		p.advanceBits(8)
		startCodeLength += 8
		v, ok = p.peekBits(startCodePrefixLen)
		if !ok {
			p.computeTrailingZeros()
			if startCodeLength > 0 {
				p.rewind(startCodeLength)
			}
			return false
		}
	}
	if v == startCodePrefix {
		return true
	}
	if startCodeLength > 0 {
		p.rewind(startCodeLength)
	}
	return false
}

func (p *pesParser) byteAlign() bool {
	for p.bitPos%8 != 0 {
		v, ok := p.peekBits(1)
		if !ok {
			return false
		}
		if v == 1 {
			return false
		}
		p.advanceBits(1)
	}
	return true
}

func (p *pesParser) computeTrailingZeros() {
	n := len(p.buf)
	if n > 0 && p.buf[n-1] == 0 {
		p.trailingZeroBits += 8
	}
	if n > 1 && p.buf[n-2] == 0 {
		p.trailingZeroBits += 8
	}
}

func (p *pesParser) markIncomplete() {
	p.incompleteStartCode = p.currentStartCode
	p.done = true
}

// peekBits reads up to 32 bits from the current bit position without advancing.
func (p *pesParser) peekBits(n int) (int, bool) {
	if n == 0 {
		return 0, true
	}
	if n > 32 {
		return 0, false
	}
	bitPos := p.bitPos
	alignDelta := bitPos % 8
	value := 0
	bitsToRead := n

	if alignDelta > 0 {
		bytePos := bitPos / 8
		if bytePos >= len(p.buf) {
			return 0, false
		}
		data := int(p.buf[bytePos])
		// Mask out bits already consumed in this byte.
		for i := 0; i < alignDelta; i++ {
			data &^= 1 << (7 - i)
		}
		bitsAvail := 8 - alignDelta
		take := bitsAvail
		if take > bitsToRead {
			take = bitsToRead
		}
		bitsToRead -= take
		if bitsToRead == 0 {
			// Shift right by the unused bits at the end of this byte.
			data >>= bitsAvail - take
			return data, true
		}
		value = data
		bitPos = (bytePos + 1) * 8
	}

	for bitsToRead > 0 {
		bytePos := bitPos / 8
		if bytePos >= len(p.buf) {
			return 0, false
		}
		value = (value << 8) | int(p.buf[bytePos])
		bitPos += 8
		bitsToRead -= 8
	}
	if bitsToRead < 0 {
		value >>= -bitsToRead
	}
	return value, true
}

func (p *pesParser) getAndAdvanceBits(n int) (int, bool) {
	v, ok := p.peekBits(n)
	if !ok {
		return 0, false
	}
	p.advanceBits(n)
	return v, true
}

func (p *pesParser) advanceBits(n int) {
	p.bitPos += n
	p.bitLength += n
}

func (p *pesParser) rewind(n int) {
	p.bitPos -= n
	p.bitLength -= n
}

func (p *pesParser) skipBytes(n int) bool {
	bits := n * 8
	if p.bitPos+bits > len(p.buf)*8 {
		return false
	}
	p.advanceBits(bits)
	return true
}
