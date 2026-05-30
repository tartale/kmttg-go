package decrypter

import (
	"crypto/sha1"
)

type turingDecoder struct {
	key     []byte
	streams map[int]*turingStream
}

type turingStream struct {
	qt      quickTuring
	blockID int
}

func newTuringDecoder(key []byte) *turingDecoder {
	cp := make([]byte, len(key))
	copy(cp, key)
	return &turingDecoder{
		key:     cp,
		streams: map[int]*turingStream{},
	}
}

func (td *turingDecoder) prepareFrame(streamID, blockID int) *turingStream {
	s, ok := td.streams[streamID]
	if !ok {
		s = &turingStream{blockID: -1}
		td.streams[streamID] = s
	}
	if s.blockID != blockID {
		td.key[16] = byte(streamID)
		td.key[17] = byte((blockID >> 16) & 0xFF)
		td.key[18] = byte((blockID >> 8) & 0xFF)
		td.key[19] = byte(blockID & 0xFF)
		turkey := sha1.Sum(td.key[:17])
		turiv := sha1.Sum(td.key)
		s.qt.init(turkey[:], turiv[:])
		s.blockID = blockID
	}
	return s
}
