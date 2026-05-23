package tivolibre

import (
	"io"
)

type TivoStream struct {
	header *TivoStreamHeader
	chunks []*TivoStreamChunk
	mak    string
	cr     *CountingReader
}

func NewTivoStream(r io.Reader, mak string) *TivoStream {
	return &TivoStream{mak: mak, cr: NewCountingReader(r)}
}

// GetHeader returns the parsed TiVo stream header.
func (ts *TivoStream) GetHeader() *TivoStreamHeader {
	return ts.header
}

// GetChunks returns the metadata chunks.
func (ts *TivoStream) GetChunks() []*TivoStreamChunk {
	return ts.chunks
}

// GetCountingReader returns the underlying counting reader.
func (ts *TivoStream) GetCountingReader() *CountingReader {
	return ts.cr
}

// ProcessMetadata reads header and chunks, decrypting metadata where necessary.
// Returns slice of metadata chunk strings (XML) or error.
func (ts *TivoStream) ProcessMetadata() ([]string, error) {
	ts.header = NewTivoStreamHeader()
	if err := ts.header.Read(ts.cr); err != nil {
		return nil, err
	}
	num := ts.header.NumChunks()
	ts.chunks = make([]*TivoStreamChunk, num)
	var metaPos int64 = 0
	var metaDecoder *TuringDecoder
	var results []string
	for i := 0; i < num; i++ {
		chunk := NewTivoStreamChunk()
		// position where chunk payload starts (current position + header size)
		chunkDataPos := ts.cr.Position() + int64(TivoStreamChunkHeaderSize())
		if err := chunk.Read(ts.cr); err != nil {
			return nil, err
		}
		if chunk.IsEncrypted() {
			if metaDecoder != nil {
				offset := int(chunkDataPos - metaPos)
				_ = chunk.DecryptWithDecoder(metaDecoder, offset)
			}
			metaPos = chunkDataPos + int64(chunk.dataSize)
		} else {
			// plaintext chunk contains keys
			metaDecoder = NewTuringDecoder(chunk.GetMetadataKey(ts.mak))
		}
		results = append(results, chunk.DataString())
		ts.chunks[i] = chunk
	}
	return results, nil
}

func TivoStreamChunkHeaderSize() int { return 12 }
