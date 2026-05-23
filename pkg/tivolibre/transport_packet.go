package tivolibre

import (
    "errors"
)

const TSFrameSize = 188

type TransportPacket struct {
    Raw []byte
    PID int
    PayloadUnitStart bool
    AdaptationFieldControl int
    PayloadOffset int
}

func ParseTransportPacket(raw []byte) (*TransportPacket, error) {
    if len(raw) != TSFrameSize {
        return nil, errors.New("invalid TS frame size")
    }
    if raw[0] != 0x47 { return nil, errors.New("invalid sync byte") }
    payloadUnitStart := (raw[1]&0x40) != 0
    pid := int(raw[1]&0x1F)<<8 | int(raw[2])
    afc := int((raw[3] & 0x30) >> 4)
    payloadOffset := 4
    if afc == 2 { // adaptation only
        payloadOffset = 4 + 1 + int(raw[4])
    } else if afc == 3 { // adaptation + payload
        // adaptation length in byte 4
        adapLen := int(raw[4])
        payloadOffset = 5 + adapLen
    }
    // pointer field handling when payloadUnitStart
    if payloadUnitStart && payloadOffset < TSFrameSize {
        pointer := int(raw[payloadOffset])
        payloadOffset += 1 + pointer
    }
    if payloadOffset > TSFrameSize { payloadOffset = TSFrameSize }
    return &TransportPacket{Raw: raw, PID: pid, PayloadUnitStart: payloadUnitStart, AdaptationFieldControl: afc, PayloadOffset: payloadOffset}, nil
}

func (p *TransportPacket) GetData() []byte {
    if p.PayloadOffset >= len(p.Raw) { return []byte{} }
    return p.Raw[p.PayloadOffset:]
}

func (p *TransportPacket) GetBytes() []byte { return p.Raw }

func (p *TransportPacket) NeedsDecoding() bool {
    // heuristic: if PID matches private data or not
    return true
}

func (p *TransportPacket) GetPID() int { return p.PID }

func (p *TransportPacket) SetPayload(data []byte) {
    // replace payload portion starting at PayloadOffset
    copy(p.Raw[p.PayloadOffset:], data)
}
