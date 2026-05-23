package tivolibre

type Stream struct {
    StreamId int
    TuringKey []byte
    TuringBlockNumber int
    TuringCrypted int
}

func NewStream() *Stream {
    return &Stream{TuringKey: make([]byte, 16)}
}

func (s *Stream) SetStreamId(id int) { s.StreamId = id }

func (s *Stream) SetKey(key []byte) {
    if len(key) >= 16 {
        copy(s.TuringKey, key[:16])
    }
}

func (s *Stream) DoHeader() bool {
    key := s.TuringKey
    keyIsSet := true
    if (key[0]&0x80) == 0 { keyIsSet = false }
    if (key[1]&0x40) == 0 { keyIsSet = false }
    if (key[3]&0x20) == 0 { keyIsSet = false }
    if (key[4]&0x10) == 0 { keyIsSet = false }
    if (key[0xd]&0x2) == 0 { keyIsSet = false }
    if (key[0xf]&0x1) == 0 { keyIsSet = false }

    s.TuringBlockNumber = int(key[1]&0x3f) << 0x12
    s.TuringBlockNumber |= int(key[2]&0xff) << 0xa
    s.TuringBlockNumber |= int(key[3]&0xc0) << 0x2
    s.TuringBlockNumber |= int(key[3]&0x1f) << 0x3
    s.TuringBlockNumber |= int(key[4]&0xe0) >> 0x5

    s.TuringCrypted = int(key[0xb]&0x03) << 0x1e
    s.TuringCrypted |= int(key[0xc]&0xff) << 0x16
    s.TuringCrypted |= int(key[0xd]&0xfc) << 0xe
    s.TuringCrypted |= int(key[0xd]&0x01) << 0xf
    s.TuringCrypted |= int(key[0xe]&0xff) << 0x7
    s.TuringCrypted |= int(key[0xf]&0xfe) >> 0x1

    return keyIsSet
}
