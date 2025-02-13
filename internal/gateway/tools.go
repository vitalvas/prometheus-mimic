package gateway

import (
	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

func decompressSnappy(compressed []byte) ([]byte, error) {
	decompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}

func decompressZSTD(compressed []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, err
	}
	defer decoder.Close()

	decompressed, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}
