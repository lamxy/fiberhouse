package jsoncodec

import (
	"bytes"
	"io"
	"strings"
	"testing"

	ginjson "github.com/gin-gonic/gin/codec/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fullJSONCodec interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte, interface{}) error
	MarshalIndent(any, string, string) ([]byte, error)
	NewEncoder(io.Writer) ginjson.Encoder
	NewDecoder(io.Reader) ginjson.Decoder
}

func TestJSONCodecs_RoundTripIndentStreamsAndMalformedInput(t *testing.T) {
	constructors := map[string]func() fullJSONCodec{
		"std":           func() fullJSONCodec { return StdJsonDefault() },
		"sonic escape":  func() fullJSONCodec { return SonicJsonEscape() },
		"sonic sorted":  func() fullJSONCodec { return SonicJsonSortEscape() },
		"sonic default": func() fullJSONCodec { return SonicJsonDefault() },
		"sonic std":     func() fullJSONCodec { return SonicJsonStd() },
		"sonic fastest": func() fullJSONCodec { return SonicJsonFastest() },
	}
	type payload struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}
	want := payload{Name: "fiberhouse", ID: 7}

	for name, constructor := range constructors {
		t.Run(name, func(t *testing.T) {
			codec := constructor()
			encoded, err := codec.Marshal(want)
			require.NoError(t, err)
			var decoded payload
			require.NoError(t, codec.Unmarshal(encoded, &decoded))
			assert.Equal(t, want, decoded)

			indented, err := codec.MarshalIndent(want, "", "  ")
			require.NoError(t, err)
			assert.Contains(t, string(indented), "\n")

			var stream bytes.Buffer
			require.NoError(t, codec.NewEncoder(&stream).Encode(want))
			decoded = payload{}
			require.NoError(t, codec.NewDecoder(&stream).Decode(&decoded))
			assert.Equal(t, want, decoded)

			assert.Error(t, codec.Unmarshal([]byte(`{"name":`), &decoded))
			assert.Error(t, codec.NewDecoder(strings.NewReader(`{"name":`)).Decode(&decoded))
		})
	}
}

func TestJSONSonicConstructorsApplyEscapeAndSortConfiguration(t *testing.T) {
	escaped, err := SonicJsonEscape().Marshal(map[string]string{"html": "<tag>"})
	require.NoError(t, err)
	assert.Contains(t, string(escaped), `\u003c`)

	sorted, err := SonicJsonSortEscape().Marshal(map[string]int{"z": 1, "a": 2})
	require.NoError(t, err)
	assert.Less(t, strings.Index(string(sorted), `"a"`), strings.Index(string(sorted), `"z"`))

	custom := SonicJsonDefault().SetCfg()
	assert.NotNil(t, custom.ConfigDefault)
	custom = custom.SetCfg(SonicJsonEscape().Config)
	escaped, err = custom.Marshal("<tag>")
	require.NoError(t, err)
	assert.Contains(t, string(escaped), `\u003c`)
}
