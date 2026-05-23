package tivolibre

import (
    "errors"
)

type PesHeader struct {
    StreamId byte
    PacketLength int
    HeaderLength int
    PayloadOffset int
}

// ParsePesHeaderAt parses a PES header starting at offset in data and
// returns the PesHeader and the offset immediately after the header (payload start).
func ParsePesHeaderAt(data []byte, offset int) (*PesHeader, int, error) {
    if offset+6 > len(data) {
        return nil, 0, errors.New("not enough data for PES header")
    }
    if !(data[offset] == 0x00 && data[offset+1] == 0x00 && data[offset+2] == 0x01) {
        return nil, 0, errors.New("no PES start code at offset")
    }
    streamId := data[offset+3]
    pesLen := int(data[offset+4])<<8 | int(data[offset+5])

    // Minimal PES header parsing: if streamId denotes PES with optional header,
    // there's flags at offset+6..+8 and a header_data_length at offset+8.
    // We'll attempt to read header_data_length when present; otherwise assume no extra header.
    payloadOffset := offset + 6
    headerLen := 0
    if payloadOffset+3 <= len(data) {
        // check for PES optional header marker (2 bits 10 in first two bits of byte at payloadOffset)
        // We simplify: read header_data_length at payloadOffset+2
        headerDataLen := int(data[payloadOffset+2])
        headerLen = headerDataLen
        payloadOffset += 3 + headerLen
    }
    if payloadOffset > len(data) { payloadOffset = len(data) }

    return &PesHeader{StreamId: streamId, PacketLength: pesLen, HeaderLength: headerLen, PayloadOffset: payloadOffset}, payloadOffset, nil
}
