package tivolibre

import (
	"bytes"
	"testing"
)

func TestTransportStreamDecoder_SkipsToAbsoluteMpegOffset(t *testing.T) {
	const initialPos = 19380
	const mpegOffset = 20480
	const gap = mpegOffset - initialPos

	// Build a stream where the reader is already advanced to initialPos,
	// then there are gap bytes before a valid TS packet.
	packet := make([]byte, TSFrameSize)
	packet[0] = 0x47
	packet[1] = 0x00
	packet[2] = 0x11
	packet[3] = 0x10
	packet[4] = 0x00

	data := make([]byte, initialPos+gap+TSFrameSize)
	copy(data[initialPos+gap:], packet)

	cr := NewCountingReader(bytes.NewReader(data))
	if _, err := cr.SkipBytes(initialPos); err != nil {
		t.Fatalf("failed to position reader: %v", err)
	}

	var out bytes.Buffer
	decoder := NewTransportStreamDecoder(NewTuringDecoder(nil), mpegOffset, cr, &out)
	if err := decoder.Process(); err != nil {
		t.Fatalf("process failed: %v", err)
	}

	outBytes := out.Bytes()
	if len(outBytes) != TSFrameSize {
		t.Fatalf("expected one TS packet of %d bytes, got %d bytes", TSFrameSize, len(outBytes))
	}
	if outBytes[0] != 0x47 {
		t.Fatalf("expected TS sync byte 0x47, got 0x%02x", outBytes[0])
	}
}
