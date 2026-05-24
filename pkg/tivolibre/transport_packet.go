package tivolibre

import (
	"errors"
)

const TSFrameSize = 188

type TransportPacket struct {
	Raw                    []byte
	PID                    int
	PayloadUnitStart       bool
	ScramblingControl      int
	AdaptationFieldControl int
	PayloadOffset          int
	PesHeaderOffset        int
}

func ParseTransportPacket(raw []byte) (*TransportPacket, error) {
	if len(raw) != TSFrameSize {
		return nil, errors.New("invalid TS frame size")
	}
	if raw[0] != 0x47 {
		return nil, errors.New("invalid sync byte")
	}
	payloadUnitStart := (raw[1] & 0x40) != 0
	pid := int(raw[1]&0x1F)<<8 | int(raw[2])
	scramblingControl := int((raw[3] & 0xC0) >> 6)
	afc := int((raw[3] & 0x30) >> 4)
	payloadOffset := 4
	if afc == 2 { // adaptation only
		payloadOffset = 4 + 1 + int(raw[4])
	} else if afc == 3 { // adaptation + payload
		// adaptation length in byte 4
		adapLen := int(raw[4])
		payloadOffset = 5 + adapLen
	}
	// pointer field handling for PSI packets when payloadUnitStart.
	// PES packets begin with 0x000001, so we only consume the pointer field when
	// the data at the current offset does not already look like a PES start code.
	if payloadUnitStart && payloadOffset+3 <= TSFrameSize {
		if !(raw[payloadOffset] == 0x00 && raw[payloadOffset+1] == 0x00 && raw[payloadOffset+2] == 0x01) {
			pointer := int(raw[payloadOffset])
			payloadOffset += 1 + pointer
		}
	}
	if payloadOffset > TSFrameSize {
		payloadOffset = TSFrameSize
	}
	return &TransportPacket{Raw: raw, PID: pid, PayloadUnitStart: payloadUnitStart, ScramblingControl: scramblingControl, AdaptationFieldControl: afc, PayloadOffset: payloadOffset}, nil
}

func (p *TransportPacket) GetData() []byte {
	if p.PayloadOffset >= len(p.Raw) {
		return []byte{}
	}
	return p.Raw[p.PayloadOffset:]
}

func (p *TransportPacket) GetBytes() []byte { return p.Raw }

func (p *TransportPacket) NeedsDecoding() bool {
	return p.PayloadOffset < len(p.Raw) && p.ScramblingControl != 0
}

func (p *TransportPacket) GetPID() int { return p.PID }

func (p *TransportPacket) SetPayload(data []byte) {
	copy(p.Raw[p.PayloadOffset:], data)
}

func (p *TransportPacket) SetPayloadAt(offset int, data []byte) {
	copy(p.Raw[p.PayloadOffset+offset:], data)
}

func (p *TransportPacket) SetPesHeaderOffset(val int) {
	p.PesHeaderOffset = val
}

func (p *TransportPacket) GetPesHeaderOffset() int {
	return p.PesHeaderOffset
}

func (p *TransportPacket) ClearScrambled() {
	if len(p.Raw) >= 4 {
		p.Raw[3] &= 0x3F
	}
}
