package stdio

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// TestNormalized_ZeroValue verifies that normalizing an empty Streams struct
// replaces nil streams with an empty EOF reader and discard writers.
func TestNormalized_ZeroValue(t *testing.T) {
	t.Parallel()

	zero := Streams{}
	norm := zero.Normalized()

	// Verify original streams struct was not mutated.
	if zero.In != nil || zero.Out != nil || zero.ErrOut != nil {
		t.Errorf("zero Streams was mutated by Normalized(): %+v", zero)
	}

	// In must yield EOF immediately.
	buf := make([]byte, 8)
	n, err := norm.In.Read(buf)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Errorf("norm.In.Read() = (%d, %v), want (0, io.EOF)", n, err)
	}

	// Out must discard writes without error.
	written, err := norm.Out.Write([]byte("test output"))
	if err != nil || written != len("test output") {
		t.Errorf("norm.Out.Write() = (%d, %v), want (%d, nil)",
			written, err, len("test output"))
	}

	// ErrOut must discard writes without error.
	written, err = norm.ErrOut.Write([]byte("test err"))
	if err != nil || written != len("test err") {
		t.Errorf("norm.ErrOut.Write() = (%d, %v), want (%d, nil)",
			written, err, len("test err"))
	}
}

// TestNormalized_PreservesNonNilStreams verifies that existing streams preserve
// their exact pointer identity.
func TestNormalized_PreservesNonNilStreams(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("sample payload")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	streams := Streams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}

	norm := streams.Normalized()

	if norm.In != in {
		t.Errorf("norm.In was replaced; want preserved identity")
	}
	if norm.Out != out {
		t.Errorf("norm.Out was replaced; want preserved identity")
	}
	if norm.ErrOut != errOut {
		t.Errorf("norm.ErrOut was replaced; want preserved identity")
	}
}

// TestNormalized_PartialNil verifies that only nil fields are replaced while
// non-nil fields retain their identity.
func TestNormalized_PartialNil(t *testing.T) {
	t.Parallel()

	t.Run("only In provided", func(t *testing.T) {
		t.Parallel()
		in := strings.NewReader("input")
		norm := Streams{In: in}.Normalized()

		if norm.In != in {
			t.Errorf("norm.In was replaced; want preserved identity")
		}
		if norm.Out != io.Discard {
			t.Errorf("norm.Out = %v, want io.Discard", norm.Out)
		}
		if written, err := norm.Out.Write([]byte("data")); err != nil || written != 4 {
			t.Errorf("norm.Out.Write() = (%d, %v), want (4, nil)", written, err)
		}
		if norm.ErrOut != io.Discard {
			t.Errorf("norm.ErrOut = %v, want io.Discard", norm.ErrOut)
		}
		if written, err := norm.ErrOut.Write([]byte("err")); err != nil || written != 3 {
			t.Errorf("norm.ErrOut.Write() = (%d, %v), want (3, nil)", written, err)
		}
	})

	t.Run("only Out provided", func(t *testing.T) {
		t.Parallel()
		out := &bytes.Buffer{}
		norm := Streams{Out: out}.Normalized()

		buf := make([]byte, 1)
		if n, err := norm.In.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
			t.Errorf("norm.In.Read() = (%d, %v), want (0, io.EOF)", n, err)
		}
		if norm.Out != out {
			t.Errorf("norm.Out was replaced; want preserved identity")
		}
		if norm.ErrOut != io.Discard {
			t.Errorf("norm.ErrOut = %v, want io.Discard", norm.ErrOut)
		}
	})

	t.Run("only ErrOut provided", func(t *testing.T) {
		t.Parallel()
		errOut := &bytes.Buffer{}
		norm := Streams{ErrOut: errOut}.Normalized()

		buf := make([]byte, 1)
		if n, err := norm.In.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
			t.Errorf("norm.In.Read() = (%d, %v), want (0, io.EOF)", n, err)
		}
		if norm.Out != io.Discard {
			t.Errorf("norm.Out = %v, want io.Discard", norm.Out)
		}
		if norm.ErrOut != errOut {
			t.Errorf("norm.ErrOut was replaced; want preserved identity")
		}
	})
}

// TestNormalized_Idempotent verifies that repeated normalization preserves
// stream identity and does not re-wrap or allocate new streams.
func TestNormalized_Idempotent(t *testing.T) {
	t.Parallel()

	t.Run("zero streams", func(t *testing.T) {
		t.Parallel()
		first := Streams{}.Normalized()
		second := first.Normalized()

		if second.In != first.In {
			t.Errorf("second.In != first.In; normalization is not idempotent")
		}
		if second.Out != first.Out {
			t.Errorf("second.Out != first.Out; normalization is not idempotent")
		}
		if second.ErrOut != first.ErrOut {
			t.Errorf("second.ErrOut != first.ErrOut; normalization is not idempotent")
		}
	})

	t.Run("populated streams", func(t *testing.T) {
		t.Parallel()
		in := strings.NewReader("input")
		out := &bytes.Buffer{}
		errOut := &bytes.Buffer{}

		orig := Streams{In: in, Out: out, ErrOut: errOut}
		first := orig.Normalized()
		second := first.Normalized()

		if second.In != in {
			t.Errorf("second.In != in; want preserved original identity")
		}
		if second.Out != out {
			t.Errorf("second.Out != out; want preserved original identity")
		}
		if second.ErrOut != errOut {
			t.Errorf("second.ErrOut != errOut; want preserved original identity")
		}
	})
}

// TestNormalized_NoGlobalState verifies that concurrent calls across goroutines
// run cleanly without shared mutable state or data races.
func TestNormalized_NoGlobalState(t *testing.T) {
	t.Parallel()

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			s := Streams{}.Normalized()
			if s.In == nil || s.Out == nil || s.ErrOut == nil {
				t.Errorf("unexpected nil stream in concurrent normalization")
			}
			buf := make([]byte, 4)
			if n, err := s.In.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
				t.Errorf("concurrent In read failed: (%d, %v)", n, err)
			}
		}()
	}

	wg.Wait()
}
