package qpdf

import (
	"bytes"
	"errors"
	"testing"
)

func TestNormalizeObjectID(t *testing.T) {
	test_cases := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"trailer", "trailer", false},
		{"TRAILER", "trailer", false},
		{"obj:4 0 R", "4,0", false},
		{"4 0 R", "4,0", false},
		{"4,0", "4,0", false},
		{" 4,0 ", "4,0", false},
		{"4", "4,0", false},
		{"123", "123,0", false},
		{"", "", true},
		{"-1", "", true},
		{"4 0 X", "", true},
		{"invalid", "", true},
		{"1,2,3", "", true},
	}

	for _, tc := range test_cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := NormalizeObjectID(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for input %q, got %q", tc.input, got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input %q: %v", tc.input, err)
				}
				if got != tc.expected {
					t.Fatalf("expected %q, got %q for input %q", tc.expected, got, tc.input)
				}
			}
		})
	}
}

func TestReadBounded(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		r := bytes.NewReader([]byte("small payload"))
		data, err := ReadBounded(r, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "small payload" {
			t.Fatalf("unexpected data: %s", string(data))
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		r := bytes.NewReader([]byte("large payload exceeding limit"))
		data, err := ReadBounded(r, 5)
		if err == nil {
			t.Fatalf("expected ErrStreamExceedsLimit")
		}
		if !errors.Is(err, ErrStreamExceedsLimit) {
			t.Fatalf("expected ErrStreamExceedsLimit, got %v", err)
		}
		if string(data) != "large" {
			t.Fatalf("expected 'large', got %s", string(data))
		}
	})

	t.Run("unlimited", func(t *testing.T) {
		r := bytes.NewReader([]byte("unlimited read"))
		data, err := ReadBounded(r, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "unlimited read" {
			t.Fatalf("expected 'unlimited read', got %s", string(data))
		}
	})
}
