// Package tivolibre decodes encrypted .TiVo files into raw MPEG transport
// streams. It is a Go port of the relevant pieces of net.straylightlabs.tivolibre
// (Java) and tivodecode-ng (C++).
package decrypter

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/tartale/kmttg-plus/go/pkg/config"
)

const (
	tsFrameSize     = 188
	tsSyncByte      = 0x47
	chunkHeaderSize = 12
)

type tivoHeader struct {
	dummy06    uint16
	mpegOffset uint32
	numChunks  uint16
}

func (h *tivoHeader) isTransportStream() bool {
	return h.dummy06&0x20 == 0x20
}

type tivoChunk struct {
	chunkSize uint32
	dataSize  uint32
	chunkType uint16
	data      []byte
}

func (c *tivoChunk) isEncrypted() bool { return c.chunkType == 1 }

func (c *tivoChunk) deriveVideoKey() []byte {
	h := sha1.New()
	h.Write([]byte(config.Values.MediaAccessKey))
	h.Write(c.data)
	return h.Sum(nil)
}

// Decrypt decrypts TiVo data from input to output. The input must be positioned
// at the start of a .TiVo file. The decrypted MPEG transport stream is written
// to output.
func Decrypt(input io.Reader, output io.Writer) error {
	br := bufio.NewReaderSize(input, 1<<20)
	bw := bufio.NewWriterSize(output, 1<<20)

	header, err := readTivoHeader(br)
	if err != nil {
		return err
	}

	bytesRead := int64(16)
	var videoDecoder *turingDecoder

	for i := 0; i < int(header.numChunks); i++ {
		chunk, err := readTivoChunk(br)
		if err != nil {
			return err
		}
		bytesRead += int64(chunk.chunkSize)
		if !chunk.isEncrypted() {
			videoDecoder = newTuringDecoder(chunk.deriveVideoKey())
		}
	}

	if bytesRead < int64(header.mpegOffset) {
		if _, err := io.CopyN(io.Discard, br, int64(header.mpegOffset)-bytesRead); err != nil {
			return fmt.Errorf("skip to mpeg offset: %w", err)
		}
	}

	if !header.isTransportStream() {
		return errors.New("program-stream format not supported")
	}
	if videoDecoder == nil {
		return errors.New("no plaintext chunk found in header")
	}

	if err := decodeTransportStream(br, bw, videoDecoder); err != nil {
		return err
	}

	return bw.Flush()
}

func readTivoHeader(r io.Reader) (*tivoHeader, error) {
	var buf [16]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, fmt.Errorf("read tivo header: %w", err)
	}
	if string(buf[0:4]) != "TiVo" {
		return nil, errors.New("tivolibre: input is not a TiVo file")
	}
	return &tivoHeader{
		dummy06:    binary.BigEndian.Uint16(buf[6:8]),
		mpegOffset: binary.BigEndian.Uint32(buf[10:14]),
		numChunks:  binary.BigEndian.Uint16(buf[14:16]),
	}, nil
}

func readTivoChunk(r io.Reader) (*tivoChunk, error) {
	var hdr [chunkHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read chunk header: %w", err)
	}
	c := &tivoChunk{
		chunkSize: binary.BigEndian.Uint32(hdr[0:4]),
		dataSize:  binary.BigEndian.Uint32(hdr[4:8]),
		chunkType: binary.BigEndian.Uint16(hdr[10:12]),
	}
	c.data = make([]byte, c.dataSize)
	if _, err := io.ReadFull(r, c.data); err != nil {
		return nil, fmt.Errorf("read chunk data: %w", err)
	}
	if padding := int(c.chunkSize) - int(c.dataSize) - chunkHeaderSize; padding > 0 {
		if _, err := io.CopyN(io.Discard, r, int64(padding)); err != nil {
			return nil, fmt.Errorf("skip chunk padding: %w", err)
		}
	}
	return c, nil
}
