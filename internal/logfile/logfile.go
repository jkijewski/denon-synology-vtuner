// Package logfile is an io.Writer that rotates a file by size.
package logfile

import (
	"fmt"
	"os"
	"sync"
)

// Writer appends to path and rotates it to path.1 ... path.N when it grows past MaxBytes.
type Writer struct {
	Path     string
	MaxBytes int64
	Keep     int
	mu       sync.Mutex
	f        *os.File
	size     int64
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, st.Size()
	return nil
}

func (w *Writer) rotate() error {
	if w.f != nil {
		w.f.Close()
		w.f = nil
	}
	os.Remove(fmt.Sprintf("%s.%d", w.Path, w.Keep))
	for i := w.Keep - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", w.Path, i), fmt.Sprintf("%s.%d", w.Path, i+1))
	}
	if w.Keep >= 1 {
		os.Rename(w.Path, w.Path+".1")
	} else {
		os.Remove(w.Path)
	}
	return w.open()
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.MaxBytes > 0 && w.size+int64(len(p)) > w.MaxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}
