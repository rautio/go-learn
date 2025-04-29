package main

import (
	"testing"
)

// Mock io.Reader for testing
type MockReader struct {
	data []byte
	err  error
}

func (m *MockReader) Read(p []byte) (int, error) {
		if m.err != nil {
				return 0, m.err
		}
		n := copy(p, m.data)
		return n, nil
}


func TestGenerateKey(t *testing.T) {
	mockReader := &MockReader{data: []byte{101, 112, 123, 134, 145, 156}}
	key, _ := generateKey(6, mockReader)
	expected := "NY9kvG"
	if key != expected {
		t.Errorf("expected key %s, got %s", expected, key)
	}
}