package tivolibre

import (
    "fmt"
)

type TivoStreamHeader struct {
    fileType [4]byte
    dummy06  uint16
    mpegOffset int32
    numChunks  uint16
}

func NewTivoStreamHeader() *TivoStreamHeader {
    return &TivoStreamHeader{}
}

func (h *TivoStreamHeader) Read(cr *CountingReader) error {
    // first four bytes "TiVo"
    for i := 0; i < 4; i++ {
        b, err := cr.ReadByte()
        if err != nil {
            return err
        }
        h.fileType[i] = b
    }
    // next two bytes mystery
    _, err := cr.ReadUnsignedShort()
    if err != nil { return err }
    // next two bytes providence
    v, err := cr.ReadUnsignedShort()
    if err != nil { return err }
    h.dummy06 = v
    // next two bytes mystery
    _, err = cr.ReadUnsignedShort()
    if err != nil { return err }
    // next four bytes mpeg offset
    mo, err := cr.ReadInt32()
    if err != nil { return err }
    h.mpegOffset = mo
    // next two bytes numChunks
    nc, err := cr.ReadUnsignedShort()
    if err != nil { return err }
    h.numChunks = nc
    return nil
}

func (h *TivoStreamHeader) NumChunks() int { return int(h.numChunks) }
func (h *TivoStreamHeader) MpegOffset() int { return int(h.mpegOffset) }
func (h *TivoStreamHeader) Format() TivoStreamFormat {
    if (h.dummy06 & 0x20) == 0x20 { return FormatTransport }
    return FormatProgram
}

func (h *TivoStreamHeader) String() string {
    return fmt.Sprintf("TivoStreamHeader{fileType=%s, mpegOffset=0x%x, numChunks=%d}", string(h.fileType[:]), h.mpegOffset, h.numChunks)
}

type TivoStreamFormat int

const (
    FormatProgram TivoStreamFormat = iota
    FormatTransport
)
