// Codec defines how session values and metadata (like creation time)
// are serialized to and from bytes, allowing them to be stored or transmitted.
// The package includes a default implementation using Go's `encoding/gob`.
package session

import (
	"bytes"
	"encoding/gob"
	"time"
)

// Codec is an interface for serializing and deserializing session data.
type Codec interface {
	// Decode decodes byte slice into the session creation time and values.
	Decode(data []byte) (createdAt time.Time, values map[string]any, err error)

	// Encode encodes the creation time and session values into a byte slice.
	Encode(createdAt time.Time, values map[string]any) (data []byte, err error)
}

// Ensure gobCodec implements Codec.
var _ Codec = gobCodec{}

// gobCodec is a Codec implementation using Go's encoding/gob. It serializes
// a gobData struct containing the creation time and session values.
type gobCodec struct{}

type gobData struct {
	CreatedAt time.Time
	Values    map[string]any
}

// Encode serializes the creation time and session values into a byte slice
// using gob encoding.
func (gobCodec) Encode(createdAt time.Time, values map[string]any) ([]byte, error) {

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(&gobData{CreatedAt: createdAt, Values: values})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decode deserializes the data into a creation time and session values
// using gob decoding.
func (gobCodec) Decode(data []byte) (time.Time, map[string]any, error) {

	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)

	var d gobData
	err := decoder.Decode(&d)
	return d.CreatedAt, d.Values, err
}
