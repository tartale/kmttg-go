package tivolibre

import (
    "bufio"
    "encoding/binary"
    "io"
)

// CountingReader wraps an io.Reader and tracks the number of bytes read.
type CountingReader struct {
    r   *bufio.Reader
    pos int64
}

func NewCountingReader(rd io.Reader) *CountingReader {
    return &CountingReader{r: bufio.NewReader(rd), pos: 0}
}

func (c *CountingReader) Position() int64 { return c.pos }

func (c *CountingReader) Read(b []byte) (int, error) {
    n, err := io.ReadFull(c.r, b)
    if err != nil {
        return n, err
    }
    c.pos += int64(n)
    return n, nil
}

func (c *CountingReader) ReadByte() (byte, error) {
    b, err := c.r.ReadByte()
    if err != nil {
        return 0, err
    }
    c.pos++
    return b, nil
}

func (c *CountingReader) ReadInt32() (int32, error) {
    var v int32
    err := binary.Read(c.r, binary.BigEndian, &v)
    if err != nil {
        return 0, err
    }
    c.pos += 4
    return v, nil
}

func (c *CountingReader) ReadUnsignedShort() (uint16, error) {
    var v uint16
    err := binary.Read(c.r, binary.BigEndian, &v)
    if err != nil {
        return 0, err
    }
    c.pos += 2
    return v, nil
}

func (c *CountingReader) ReadUnsignedByte() (byte, error) {
    b, err := c.ReadByte()
    return b, err
}

func (c *CountingReader) SkipBytes(n int) (int, error) {
    skipped := 0
    buf := make([]byte, 4096)
    for n > 0 {
        toRead := n
        if toRead > len(buf) {
            toRead = len(buf)
        }
        r, err := c.r.Read(buf[:toRead])
        if r > 0 {
            skipped += r
            n -= r
            c.pos += int64(r)
        }
        if err != nil {
            if err == io.EOF {
                break
            }
            return skipped, err
        }
    }
    return skipped, nil
}
