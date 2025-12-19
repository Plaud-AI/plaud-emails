package rdb

import (
	"encoding/json"

	"github.com/bytedance/sonic"
	"github.com/ugorji/go/codec"
)

// Codec defines how objects are serialized/deserialized for Redis values.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, out any) error
	Name() string
}

// jsonCodec uses encoding/json
type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)        { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, out any) error { return json.Unmarshal(data, out) }
func (jsonCodec) Name() string                         { return "stdjson" }

// sonicCodec uses bytedance/sonic (faster JSON)
type sonicCodec struct{}

func (sonicCodec) Marshal(v any) ([]byte, error)        { return sonic.Marshal(v) }
func (sonicCodec) Unmarshal(data []byte, out any) error { return sonic.Unmarshal(data, out) }
func (sonicCodec) Name() string                         { return "sonic" }

type msgpackCodec struct{ h *codec.MsgpackHandle }

func (c msgpackCodec) Marshal(v any) ([]byte, error) {
	var b []byte
	enc := codec.NewEncoderBytes(&b, c.h)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b, nil
}

func (c msgpackCodec) Unmarshal(data []byte, out any) error {
	dec := codec.NewDecoderBytes(data, c.h)
	return dec.Decode(out)
}

func (msgpackCodec) Name() string { return "msgpack" }

// defaultCodec is used when none is specified (global for the package)
var defaultCodec Codec = MsgPackCodec()

// DefaultCodec returns the current package-wide default codec, which is MsgPackCodec by default.
func DefaultCodec() Codec { return defaultCodec }

// SetDefaultCodec sets the default codec (nil ignored)
func SetDefaultCodec(c Codec) {
	if c != nil {
		defaultCodec = c
	}
}

// Built-in codec constructors
func JSONCodec() Codec  { return jsonCodec{} }
func SonicCodec() Codec { return sonicCodec{} }
func MsgPackCodec() Codec {
	return msgpackCodec{h: &codec.MsgpackHandle{}}
}
