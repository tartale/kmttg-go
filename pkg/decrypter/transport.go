package decrypter

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// transportStreamDecoder consumes 188-byte TS frames, tracking PAT/PMT/TiVo
// private packets so each video/audio PID has the right Turing key, then
// emits frames (decrypted where needed) to w.
type transportStreamDecoder struct {
	w             io.Writer
	turingDecoder *turingDecoder
	streams       map[int]*tsStream
	pmtPID        int
}

func decodeTransportStream(r io.Reader, w io.Writer, td *turingDecoder) error {
	dec := &transportStreamDecoder{
		w:             w,
		turingDecoder: td,
		streams:       map[int]*tsStream{},
		pmtPID:        -1,
	}
	dec.streams[0] = newTsStream(td) // PAT stream

	var pkt tsPacket
	for {
		n, err := io.ReadFull(r, pkt.buffer[:])
		if err == io.EOF {
			return nil
		}
		if err == io.ErrUnexpectedEOF {
			// Truncated trailing data — drop it.
			return nil
		}
		if err != nil {
			return fmt.Errorf("read ts packet: %w", err)
		}
		if n != tsFrameSize {
			return nil
		}

		pkt.reset()
		if err := pkt.parse(); err != nil {
			// Out-of-sync packet: pass it through unchanged so the file size
			// matches the source.
			if _, werr := dec.w.Write(pkt.buffer[:]); werr != nil {
				return werr
			}
			continue
		}

		switch pkt.packetType() {
		case packetTypePAT:
			if err := dec.processPAT(&pkt); err != nil {
				return err
			}
		case packetTypeAV:
			if pkt.pid == dec.pmtPID {
				pkt.isPmt = true
				if err := dec.processPMT(&pkt); err != nil {
					return err
				}
			} else if s, ok := dec.streams[pkt.pid]; ok && s.streamType == streamTypePrivateData {
				pkt.isTiVo = true
				if err := dec.processTiVoPacket(&pkt); err != nil {
					return err
				}
			}
		case packetTypeNull, packetTypeOther:
			// Pass through.
		}

		out := dec.processStreamPacket(&pkt)
		if _, err := dec.w.Write(out); err != nil {
			return err
		}
	}
}

func (dec *transportStreamDecoder) processStreamPacket(p *tsPacket) []byte {
	s, ok := dec.streams[p.pid]
	if !ok {
		s = newTsStream(dec.turingDecoder)
		s.streamType = streamTypeNotInPMT
		dec.streams[p.pid] = s
	}
	return s.processPacket(p)
}

func (dec *transportStreamDecoder) processPAT(p *tsPacket) error {
	if p.payloadStart {
		p.advanceData(1)
	}
	tableID, ok := p.readByteFromData()
	if !ok {
		return errors.New("PAT: truncated table id")
	}
	if tableID != 0 {
		return fmt.Errorf("PAT table id must be 0x00, got 0x%02x", tableID)
	}
	patField, ok := p.readUint16FromData()
	if !ok {
		return errors.New("PAT: truncated section length")
	}
	sectionLen := patField & 0x0FFF
	if patField&0xC000 != 0x8000 {
		return fmt.Errorf("PAT: bad misc field 0x%04x", patField)
	}
	// Stream ID
	if _, ok := p.readUint16FromData(); !ok {
		return errors.New("PAT: truncated stream id")
	}
	sectionLen -= 2
	if _, ok := p.readByteFromData(); !ok {
		return errors.New("PAT: truncated version")
	}
	sectionLen--
	if _, ok := p.readByteFromData(); !ok {
		return errors.New("PAT: truncated section number")
	}
	sectionLen--
	if _, ok := p.readByteFromData(); !ok {
		return errors.New("PAT: truncated last section number")
	}
	sectionLen--
	sectionLen -= 4 // CRC

	for sectionLen > 0 {
		// Program number
		if _, ok := p.readUint16FromData(); !ok {
			return errors.New("PAT: truncated program number")
		}
		sectionLen -= 2
		f, ok := p.readUint16FromData()
		if !ok {
			return errors.New("PAT: truncated pmt pid")
		}
		sectionLen -= 2
		dec.pmtPID = f & 0x1FFF
		if _, exists := dec.streams[dec.pmtPID]; !exists {
			dec.streams[dec.pmtPID] = newTsStream(dec.turingDecoder)
		}
	}
	return nil
}

func (dec *transportStreamDecoder) processPMT(p *tsPacket) error {
	if p.payloadStart {
		p.advanceData(1)
	}
	tableID, ok := p.readByteFromData()
	if !ok {
		return errors.New("PMT: truncated table id")
	}
	if tableID != 0x02 {
		return fmt.Errorf("PMT: bad table id 0x%02x", tableID)
	}
	pmtField, ok := p.readUint16FromData()
	if !ok {
		return errors.New("PMT: truncated section length")
	}
	if pmtField&0x8000 != 0x8000 {
		return errors.New("PMT: unknown syntax")
	}
	sectionLen := pmtField & 0x0FFF

	if _, ok := p.readUint16FromData(); !ok {
		return errors.New("PMT: truncated program number")
	}
	sectionLen -= 2
	if _, ok := p.readByteFromData(); !ok {
		return errors.New("PMT: truncated version")
	}
	sectionLen--
	if _, ok := p.readByteFromData(); !ok {
		return errors.New("PMT: truncated section number")
	}
	sectionLen--
	if _, ok := p.readByteFromData(); !ok {
		return errors.New("PMT: truncated last section")
	}
	sectionLen--
	if _, ok := p.readUint16FromData(); !ok {
		return errors.New("PMT: truncated pcr pid")
	}
	sectionLen -= 2
	progInfoField, ok := p.readUint16FromData()
	if !ok {
		return errors.New("PMT: truncated program info length")
	}
	sectionLen -= 2
	progInfoLen := progInfoField & 0x0FFF
	if progInfoLen > 0 {
		p.advanceData(progInfoLen)
		sectionLen -= progInfoLen
	}
	sectionLen -= 4 // CRC

	for sectionLen > 0 {
		streamTypeID, ok := p.readByteFromData()
		if !ok {
			return errors.New("PMT: truncated stream type id")
		}
		sectionLen--
		streamType := pmtStreamType(streamTypeID)
		f, ok := p.readUint16FromData()
		if !ok {
			return errors.New("PMT: truncated stream pid")
		}
		sectionLen -= 2
		streamPID := f & 0x1FFF
		f, ok = p.readUint16FromData()
		if !ok {
			return errors.New("PMT: truncated es info length")
		}
		sectionLen -= 2
		esInfoLen := f & 0x0FFF
		p.advanceData(esInfoLen)
		sectionLen -= esInfoLen

		if _, exists := dec.streams[streamPID]; !exists {
			s := newTsStream(dec.turingDecoder)
			s.streamType = streamType
			dec.streams[streamPID] = s
		}
	}
	return nil
}

func (dec *transportStreamDecoder) processTiVoPacket(p *tsPacket) error {
	fileType, ok := p.readUint32FromData()
	if !ok {
		return errors.New("TiVo private: truncated file type")
	}
	if fileType != 0x5469566F { // "TiVo"
		return fmt.Errorf("TiVo private: bad file type 0x%08x", fileType)
	}
	validator, ok := p.readUint16FromData()
	if !ok {
		return errors.New("TiVo private: truncated validator")
	}
	if validator != 0x8103 {
		return fmt.Errorf("TiVo private: bad validator 0x%04x", validator)
	}
	p.advanceData(3)
	streamLen, ok := p.readByteFromData()
	if !ok {
		return errors.New("TiVo private: truncated stream length")
	}
	for streamLen > 0 {
		packetID, ok := p.readUint16FromData()
		if !ok {
			return errors.New("TiVo private: truncated packet id")
		}
		streamLen -= 2
		streamID, ok := p.readByteFromData()
		if !ok {
			return errors.New("TiVo private: truncated stream id")
		}
		streamLen--
		p.advanceData(1) // reserved
		streamLen--

		key, ok := p.readBytesFromData(16)
		if !ok {
			return errors.New("TiVo private: truncated turing key")
		}
		streamLen -= 16

		s, exists := dec.streams[packetID]
		if !exists {
			return fmt.Errorf("TiVo private: no stream for pid 0x%04x", packetID)
		}
		s.streamID = streamID
		s.setKey(key)
	}
	return nil
}

// tsPacket represents a single 188-byte TS frame.
type tsPacket struct {
	buffer          [tsFrameSize]byte
	pid             int
	payloadStart    bool
	transportError  bool
	scrambled       bool
	hasAdaptation   bool
	headerLen       int
	pesHeaderOffset int
	isPmt           bool
	isTiVo          bool
	dataReadOffset  int
}

func (p *tsPacket) reset() {
	p.pesHeaderOffset = 0
	p.isPmt = false
	p.isTiVo = false
	p.dataReadOffset = 0
}

func (p *tsPacket) parse() error {
	if p.buffer[0] != tsSyncByte {
		return errors.New("invalid TS sync byte")
	}
	bits := binary.BigEndian.Uint32(p.buffer[0:4])
	p.transportError = (bits & 0x00800000) != 0
	p.payloadStart = (bits & 0x00400000) != 0
	p.pid = int((bits >> 8) & 0x1FFF)
	p.scrambled = (bits & 0xC0) != 0
	p.hasAdaptation = (bits & 0x20) != 0
	if p.transportError {
		return errors.New("transport error flag set")
	}
	p.headerLen = 4
	if p.hasAdaptation {
		adapLen := int(p.buffer[4])
		if adapLen > 0 {
			p.headerLen += 1 + adapLen
		} else {
			p.headerLen++
		}
	}
	if p.headerLen > tsFrameSize {
		p.headerLen = tsFrameSize
	}
	return nil
}

func (p *tsPacket) payloadLen() int { return tsFrameSize - p.headerLen }

func (p *tsPacket) needsDecoding() bool {
	return p.scrambled && p.headerLen+p.pesHeaderOffset < tsFrameSize
}

func (p *tsPacket) clearScrambledBits() { p.buffer[3] &^= 0xC0 }

func (p *tsPacket) advanceData(n int) { p.dataReadOffset += n }

func (p *tsPacket) readByteFromData() (int, bool) {
	pos := p.headerLen + p.dataReadOffset
	if pos >= tsFrameSize {
		return 0, false
	}
	p.dataReadOffset++
	return int(p.buffer[pos]), true
}

func (p *tsPacket) readUint16FromData() (int, bool) {
	pos := p.headerLen + p.dataReadOffset
	if pos+2 > tsFrameSize {
		return 0, false
	}
	p.dataReadOffset += 2
	return int(binary.BigEndian.Uint16(p.buffer[pos:])), true
}

func (p *tsPacket) readUint32FromData() (uint32, bool) {
	pos := p.headerLen + p.dataReadOffset
	if pos+4 > tsFrameSize {
		return 0, false
	}
	p.dataReadOffset += 4
	return binary.BigEndian.Uint32(p.buffer[pos:]), true
}

func (p *tsPacket) readBytesFromData(n int) ([]byte, bool) {
	pos := p.headerLen + p.dataReadOffset
	if pos+n > tsFrameSize {
		return nil, false
	}
	out := make([]byte, n)
	copy(out, p.buffer[pos:pos+n])
	p.dataReadOffset += n
	return out, true
}

const (
	packetTypePAT   = 0
	packetTypeAV    = 1
	packetTypeNull  = 2
	packetTypeOther = 3
)

func (p *tsPacket) packetType() int {
	switch {
	case p.pid == 0:
		return packetTypePAT
	case p.pid == 0x1FFF:
		return packetTypeNull
	case p.pid >= 0x0020 && p.pid <= 0x1FFE:
		return packetTypeAV
	}
	return packetTypeOther
}

// Stream-type constants returned by pmtStreamType.
const (
	streamTypeNone = iota
	streamTypeVideo
	streamTypeAudio
	streamTypePrivateData
	streamTypeOther
	streamTypeNotInPMT
)

func pmtStreamType(typeID int) int {
	switch typeID {
	case 0x01, 0x02, 0x10, 0x1B, 0x80, 0xEA:
		return streamTypeVideo
	case 0x03, 0x04, 0x11, 0x0F, 0x81, 0x8A:
		return streamTypeAudio
	case 0x97:
		return streamTypePrivateData
	case 0x00:
		return streamTypeNone
	}
	return streamTypeOther
}

// tsStream tracks decryption state for a single transport-stream PID.
type tsStream struct {
	decoder           *turingDecoder
	streamID          int
	streamType        int
	turingKey         [16]byte
	keySet            bool
	turingBlockNumber int

	pesBuffer            []byte
	nextPacketPesOffset  int
	lastPesHeaderState   pesHeaderState
}

func newTsStream(d *turingDecoder) *tsStream {
	return &tsStream{
		decoder:            d,
		pesBuffer:          make([]byte, tsFrameSize),
		lastPesHeaderState: newPesHeaderState(),
	}
}

func (s *tsStream) setKey(key []byte) {
	copy(s.turingKey[:], key)
	s.keySet = true
}

func (s *tsStream) processPacket(p *tsPacket) []byte {
	s.copyPayloadToPesBuffer(p)
	s.calculatePesHeaderOffset(p)
	if s.keySet && p.needsDecoding() {
		s.decryptPacket(p)
	}
	return p.buffer[:]
}

func (s *tsStream) copyPayloadToPesBuffer(p *tsPacket) {
	payload := p.buffer[p.headerLen:]
	if s.nextPacketPesOffset >= len(payload) {
		return
	}
	s.pesBuffer = s.pesBuffer[:0]
	s.pesBuffer = append(s.pesBuffer, payload[s.nextPacketPesOffset:]...)
}

func (s *tsStream) calculatePesHeaderOffset(p *tsPacket) {
	payloadLen := p.payloadLen()
	if s.nextPacketPesOffset < payloadLen {
		packetOffset := s.nextPacketPesOffset
		sum := packetOffset
		if sum > 0 || p.payloadStart || !s.lastPesHeaderState.finished {
			sum += parsePesHeaderLength(s.pesBuffer, &s.lastPesHeaderState)
		}
		bufLen := payloadLen - packetOffset
		if sum-packetOffset <= bufLen {
			p.pesHeaderOffset = sum
			s.nextPacketPesOffset = 0
		} else {
			s.nextPacketPesOffset = sum - payloadLen
			p.pesHeaderOffset = payloadLen
		}
	} else {
		s.nextPacketPesOffset -= payloadLen
		p.pesHeaderOffset = payloadLen
	}
	if p.pesHeaderOffset > payloadLen {
		p.pesHeaderOffset = payloadLen
	}
}

func (s *tsStream) decryptPacket(p *tsPacket) {
	if !s.computeTuringBlockNumber() {
		return
	}
	p.clearScrambledBits()
	start := p.headerLen + p.pesHeaderOffset
	if start >= tsFrameSize {
		return
	}
	stream := s.decoder.prepareFrame(s.streamID, s.turingBlockNumber)
	stream.qt.decrypt(p.buffer[start:])
}

// computeTuringBlockNumber mirrors Stream.doHeader() from the Java source:
// six sentinel bits must be set in the key, then the block number is packed
// out of bits scattered through the key. Returns false if the key looks
// uninitialized.
func (s *tsStream) computeTuringBlockNumber() bool {
	k := &s.turingKey
	if k[0x0]&0x80 == 0 || k[0x1]&0x40 == 0 || k[0x3]&0x20 == 0 ||
		k[0x4]&0x10 == 0 || k[0xD]&0x02 == 0 || k[0xF]&0x01 == 0 {
		return false
	}
	bn := int(k[0x1]&0x3F) << 18
	bn |= int(k[0x2]) << 10
	bn |= int(k[0x3]&0xC0) << 2
	bn |= int(k[0x3]&0x1F) << 3
	bn |= int(k[0x4]&0xE0) >> 5
	s.turingBlockNumber = bn
	return true
}
