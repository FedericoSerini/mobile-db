package cbor

import (
	"reflect"

	gocbor "github.com/fxamacker/cbor/v2"
)

var stringMapType = reflect.TypeOf(map[string]interface{}{})

type Codec struct {
	dm gocbor.DecMode
}

func NewCodec() *Codec {
	dm, err := gocbor.DecOptions{
		DefaultMapType: stringMapType,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return &Codec{dm: dm}
}

func (c *Codec) Encode(v any) ([]byte, error) {
	return gocbor.Marshal(v)
}

func (c *Codec) Decode(data []byte, v any) error {
	return c.dm.Unmarshal(data, v)
}
