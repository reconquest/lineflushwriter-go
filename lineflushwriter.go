package lineflushwriter

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"sync"
)

// Writer implements writer, that will proxy to specified `backend` writer only
// complete lines, e.g. that ends in newline. This writer is thread-safe.
type Writer struct {
	mutex   *sync.Mutex
	backend io.WriteCloser
	buffer  *bytes.Buffer

	newline rune

	ensureNewline bool
}

// New returns new Writer, that will proxy data to the `backend` writer,
// thread-safety is guaranteed via `lock`. Optionally, writer can ensure, that
// last line of output ends with newline, if `ensureNewline` is true.
func New(
	writer io.WriteCloser,
	lock *sync.Mutex,
	ensureNewline bool,
) *Writer {
	return &Writer{
		backend: writer,
		mutex:   lock,
		buffer:  &bytes.Buffer{},

		ensureNewline: ensureNewline,
	}
}

// Writer writes data into Writer.
//
// Signature matches with io.Writer's Write().
func (writer *Writer) Write(data []byte) (int, error) {
	writer.mutex.Lock()
	written, err := writer.buffer.Write(data)
	writer.mutex.Unlock()
	if err != nil {
		return written, err
	}

	var (
		reader = bufio.NewReader(writer.buffer)

		eofEncountered = false
	)

	for !eofEncountered {
		writer.mutex.Lock()
		line, err := reader.ReadString('\n')

		if err != nil {
			if err != io.EOF {
				writer.mutex.Unlock()
				return 0, err
			} else {
				eofEncountered = true
			}
		}

		var target io.Writer
		if eofEncountered {
			target = writer.buffer
		} else {
			target = writer.backend
		}

		written, err := io.WriteString(target, line)

		writer.mutex.Unlock()
		if err != nil {
			return written, err
		}
	}

	return written, nil
}

// Close flushes all remaining data and closes underlying backend writer.
// If `ensureNewLine` was specified and remaining data does not ends with
// newline, then newline will be added.
//
// Signature matches with io.WriteCloser's Close().
func (writer *Writer) Close() error {
	if writer.ensureNewline && writer.buffer.Len() > 0 {
		if !strings.HasSuffix(writer.buffer.String(), "\n") {
			_, err := writer.buffer.WriteString("\n")
			if err != nil {
				return err
			}
		}
	}

	_, err := writer.backend.Write(writer.buffer.Bytes())
	if err != nil {
		return err
	}

	return writer.backend.Close()
}
