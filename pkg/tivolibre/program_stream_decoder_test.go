package tivolibre

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestProgramStreamDecoder_DecryptsPESPayload(t *testing.T) {
	// prepare a deterministic 20-byte key with bits set so Stream.DoHeader() is true
	key := make([]byte, 20)
	key[0] = 0x80
	key[1] = 0x40
	key[3] = 0x20
	key[4] = 0x10
	key[13] = 0x02
	key[15] = 0x01

	streamId := byte(0xE0)

	// register stream
	s := NewStream()
	s.SetStreamId(int(streamId))
	s.SetKey(key)
	// ensure block number is computed
	s.DoHeader()

	encoder := NewTuringDecoder(key)
	psd := NewProgramStreamDecoder(NewTuringDecoder(key))
	psd.RegisterStream(streamId, s)

	// payload to encrypt
	plaintext := []byte("HELLO_PROGRAM_STREAM")
	enc := make([]byte, len(plaintext))
	copy(enc, plaintext)

	// encrypt in-place using a fresh Turing decoder
	turing := encoder.PrepareFrame(s.StreamId, s.TuringBlockNumber)
	encoder.DecryptBytes(turing, enc)

	// build minimal PES: start code + streamId + length + flags/headerlen(3 bytes) + payload
	packetLen := uint16(len(enc) + 3) // header_data_length 0, still include 3 bytes of flags
	hdr := make([]byte, 6)
	hdr[0] = 0x00
	hdr[1] = 0x00
	hdr[2] = 0x01
	hdr[3] = streamId
	binary.BigEndian.PutUint16(hdr[4:6], packetLen)
	// optional flags: 0x80 0x00 0x00 (no pts, header_data_length=0)
	opt := []byte{0x80, 0x00, 0x00}

	var packet bytes.Buffer
	packet.Write(hdr)
	packet.Write(opt)
	packet.Write(enc)

	// process
	var out bytes.Buffer
	if err := psd.Process(bytes.NewReader(packet.Bytes()), &out); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// find payload in output: search for plaintext sequence
	outBytes := out.Bytes()
	if !bytes.Contains(outBytes, plaintext) {
		// for debugging, write the output to t.Log
		if len(outBytes) < 200 {
			t.Logf("output bytes: %x", outBytes)
		}
		t.Fatalf("decrypted payload not found in output")
	}

	// Additional sanity: ensure output contains start code
	if idx := bytes.Index(outBytes, []byte{0x00,0x00,0x01}); idx < 0 {
		t.Fatalf("output missing PES start code")
	}
}
