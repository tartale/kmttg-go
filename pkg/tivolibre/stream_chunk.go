package tivolibre

import (
    "fmt"
)

type ChunkType int

const (
    ChunkPlaintext ChunkType = iota
    ChunkEncrypted
)

type TivoStreamChunk struct {
    chunkSize int32
    dataSize  int32
    id        uint16
    ctype     ChunkType
    data      []byte
}

func NewTivoStreamChunk() *TivoStreamChunk { return &TivoStreamChunk{} }

func (c *TivoStreamChunk) IsEncrypted() bool { return c.ctype == ChunkEncrypted }

func (c *TivoStreamChunk) Read(cr *CountingReader) error {
    // read chunkSize (int)
    cs, err := cr.ReadInt32()
    if err != nil { return err }
    c.chunkSize = cs
    ds, err := cr.ReadInt32()
    if err != nil { return err }
    c.dataSize = ds
    id, err := cr.ReadUnsignedShort()
    if err != nil { return err }
    c.id = id
    t, err := cr.ReadUnsignedShort()
    if err != nil { return err }
    switch t {
    case 0:
        c.ctype = ChunkPlaintext
    case 1:
        c.ctype = ChunkEncrypted
    default:
        return fmt.Errorf("unsupported chunk type %d", t)
    }

    c.data = make([]byte, c.dataSize)
    // use Read which returns EOF if none
    if _, err := cr.Read(c.data); err != nil {
        return err
    }

    // padding bytes
    padding := int(c.chunkSize) - int(c.dataSize) - 12
    if padding > 0 {
        _, _ = cr.SkipBytes(padding)
    }
    return nil
}

func (c *TivoStreamChunk) DataString() string { return string(c.data) }

func (c *TivoStreamChunk) DecryptMetadata(mak string, offset int) error {
    if !c.IsEncrypted() {
        return nil
    }
    key := GetChunkKey(mak, c.data)
    td := NewTuringDecoder(key)
    stream := td.PrepareFrame(0, 0)
    td.SkipBytes(stream, offset)
    td.DecryptBytes(stream, c.data)
    return nil
}

// DecryptWithDecoder decrypts this chunk's data using an existing TuringDecoder and offset.
func (c *TivoStreamChunk) DecryptWithDecoder(td *TuringDecoder, offset int) error {
    if td == nil {
        return nil
    }
    stream := td.PrepareFrame(0, 0)
    td.SkipBytes(stream, offset)
    td.DecryptBytes(stream, c.data)
    return nil
}

// Key helpers
func (c *TivoStreamChunk) GetKey(mak string) []byte { return GetChunkKey(mak, c.data) }
func (c *TivoStreamChunk) GetMetadataKey(mak string) []byte { return GetMetadataKey(mak, c.data) }
