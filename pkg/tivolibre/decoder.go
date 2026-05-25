package tivolibre

import (
	"errors"
	"io"
)

// Decoder is a high-level decoder for TiVo files.
type Decoder struct {
	MAK string
}

// NewDecoder constructs a new Decoder with the provided MAK.
func NewDecoder(mak string) *Decoder {
	return &Decoder{MAK: mak}
}

// Decode decodes TiVo data from input to output, extracting video/audio streams.
// The input should be positioned at the start of a TiVo file. Video data is
// written to output according to the stream format (PS or TS).
func (d *Decoder) Decode(input io.Reader, output io.Writer) error {
	ts := NewTivoStream(input, d.MAK)
	
	// Process metadata to set up TuringDecoder and determine stream format
	_, err := ts.ProcessMetadata()
	if err != nil {
		return err
	}
	
	// Get header to determine stream format and MPEG offset
	header := ts.GetHeader()
	if header == nil {
		return errors.New("failed to get stream header")
	}
	
	// Get TuringDecoder from metadata chunks
	var td *TuringDecoder
	chunks := ts.GetChunks()
	for _, chunk := range chunks {
		if !chunk.IsEncrypted() {
			td = NewTuringDecoder(chunk.GetMetadataKey(d.MAK))
			break
		}
	}
	if td == nil {
		td = NewTuringDecoder(nil) // fallback
	}
	
	// Create and execute the appropriate decoder based on format
	mpegOffset := header.MpegOffset()
	cr := ts.GetCountingReader()
	
	switch header.Format() {
	case FormatTransport:
		decoder := NewTransportStreamDecoder(td, mpegOffset, cr, output)
		return decoder.Process()
	case FormatProgram:
		decoder := NewProgramStreamDecoder(td)
		return decoder.Process(cr, output)
	default:
		return errors.New("unknown stream format")
	}
}

// GetMetadata extracts metadata from a TiVo stream without decoding video.
// Returns the metadata chunks as XML strings.
func (d *Decoder) GetMetadata(input io.Reader) ([]string, error) {
	ts := NewTivoStream(input, d.MAK)
	
	// Process and return metadata
	metadata, err := ts.ProcessMetadata()
	if err != nil {
		return nil, err
	}
	
	return metadata, nil
}

// BuildTuringDecoder is a small helper to construct a TuringDecoder from a key byte slice.
func (d *Decoder) BuildTuringDecoder(key []byte) *TuringDecoder {
	return NewTuringDecoder(key)
}

// DecryptMetadata decrypts a metadata chunk payload using the MAK and an offset.
// It returns a newly allocated decrypted byte slice.
func (d *Decoder) DecryptMetadata(chunkData []byte, offset int) ([]byte, error) {
	if d.MAK == "" {
		return nil, errors.New("MAK is required")
	}
	key := GetChunkKey(d.MAK, chunkData)
	td := NewTuringDecoder(key)
	stream := td.PrepareFrame(0, 0)
	td.SkipBytes(stream, offset)
	out := make([]byte, len(chunkData))
	copy(out, chunkData)
	td.DecryptBytes(stream, out)
	return out, nil
}
