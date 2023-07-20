package clamd

import (
	"bytes"
	"encoding/binary"
)

func splitChunks(rawParts []byte, size int) [][]byte {
	var (
		length = len(rawParts) / size
		rem    = len(rawParts) % size
	)

	nItems := length
	if rem > 0 {
		nItems++
	}

	chunks := make([][]byte, 0, nItems)

	for i := 0; i < length; i++ {
		buf := new(bytes.Buffer)

		binary.Write(buf, binary.BigEndian, uint32(size))
		buf.Write(rawParts[i*size : i*size+size])
		chunks = append(chunks, buf.Bytes())
	}

	if rem > 0 {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, uint32(rem))
		buf.Write(rawParts[len(rawParts)-(rem):])
		chunks = append(chunks, buf.Bytes())
		buf.Reset()
	}

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, uint32(0))
	chunks = append(chunks, buf.Bytes())

	return chunks
}
