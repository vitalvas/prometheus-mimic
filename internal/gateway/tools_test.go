package gateway

import (
	"testing"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
)

func TestDecompressSnappy(t *testing.T) {
	// Test case: Valid snappy compressed data
	compressed := snappy.Encode(nil, []byte("test data"))
	decompressed, err := decompressSnappy(compressed)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(decompressed) != "test data" {
		t.Fatalf("expected 'test data', got %s", decompressed)
	}

	// Test case: Invalid snappy compressed data
	invalidCompressed := []byte{0x00, 0x01, 0x02}
	_, err = decompressSnappy(invalidCompressed)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
func TestDecompressZSTD(t *testing.T) {
	// Test case: Valid ZSTD compressed data
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("failed to create zstd encoder: %v", err)
	}
	defer encoder.Close()

	compressed := encoder.EncodeAll([]byte("test data"), nil)
	decompressed, err := decompressZSTD(compressed)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(decompressed) != "test data" {
		t.Fatalf("expected 'test data', got %s", decompressed)
	}

	// Test case: Invalid ZSTD compressed data
	invalidCompressed := []byte{0x00, 0x01, 0x02}
	_, err = decompressZSTD(invalidCompressed)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
