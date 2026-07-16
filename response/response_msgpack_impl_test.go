package response

import (
	"errors"
	"net/http"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

type responseContextRecorder struct {
	status    int
	body      []byte
	jsonValue interface{}
	err       error
}

func (c *responseContextRecorder) GetCtx() interface{}      { return nil }
func (c *responseContextRecorder) GetHeader(string) string  { return "" }
func (c *responseContextRecorder) SetHeader(string, string) {}
func (c *responseContextRecorder) Send(status int, body []byte) error {
	c.status = status
	c.body = append([]byte(nil), body...)
	return c.err
}
func (c *responseContextRecorder) JSON(status int, value interface{}) error {
	c.status = status
	c.jsonValue = value
	return c.err
}

func TestParseMsgPackResponse_MissingAndWrongFieldsReturnErrors(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]interface{}
	}{
		{name: "missing code", payload: map[string]interface{}{"msg": "ok"}},
		{name: "missing msg", payload: map[string]interface{}{"code": int64(0)}},
		{name: "wrong code", payload: map[string]interface{}{"code": "zero", "msg": "ok"}},
		{name: "wrong msg", payload: map[string]interface{}{"code": int64(0), "msg": 7}},
		{name: "overflow code", payload: map[string]interface{}{"code": ^uint64(0), "msg": "ok"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := msgpack.Marshal(test.payload)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if _, err := ParseMsgPackResponse(encoded); err == nil {
				t.Fatal("ParseMsgPackResponse() error = nil")
			}
			if _, err := ParseMsgPackResponseWithType(encoded, &map[string]interface{}{}); err == nil {
				t.Fatal("ParseMsgPackResponseWithType() error = nil")
			}
		})
	}
}

type msgPackPayload struct {
	Name string `msgpack:"name"`
	Age  int    `msgpack:"age"`
}

func TestRespInfoMsgPack_SendRoundTripDefaultAndCustomStatus(t *testing.T) {
	recorder := &responseContextRecorder{}
	response := GetRespInfoMsgPack().(*RespInfoMagPack)
	response.Reset(7, "created", msgPackPayload{Name: "Ada", Age: 36})
	if err := response.SendWithCtx(recorder); err != nil {
		t.Fatalf("SendWithCtx() error = %v", err)
	}
	if recorder.status != http.StatusOK {
		t.Fatalf("SendWithCtx() status = %d, want %d", recorder.status, http.StatusOK)
	}
	var decoded msgPackPayload
	parsed, err := ParseMsgPackResponseWithType(recorder.body, &decoded)
	if err != nil {
		t.Fatalf("ParseMsgPackResponseWithType() error = %v", err)
	}
	if parsed.Code != 7 || parsed.Msg != "created" || decoded != (msgPackPayload{Name: "Ada", Age: 36}) {
		t.Fatalf("parsed response = %#v, data = %#v", parsed, decoded)
	}
	if response.GetCode() != 0 || response.GetMsg() != "" || response.GetData() != nil {
		t.Fatalf("SendWithCtx() did not release response: code=%d msg=%q data=%#v", response.GetCode(), response.GetMsg(), response.GetData())
	}

	customRecorder := &responseContextRecorder{}
	withoutData := GetRespInfoMsgPack().(*RespInfoMagPack)
	withoutData.Reset(8, "accepted", nil)
	if err := withoutData.SendWithCtx(customRecorder, http.StatusAccepted); err != nil {
		t.Fatalf("custom SendWithCtx() error = %v", err)
	}
	if customRecorder.status != http.StatusAccepted {
		t.Fatalf("custom status = %d, want %d", customRecorder.status, http.StatusAccepted)
	}
	parsed, err = ParseMsgPackResponse(customRecorder.body)
	if err != nil {
		t.Fatalf("ParseMsgPackResponse() error = %v", err)
	}
	if parsed.Code != 8 || parsed.Msg != "accepted" || parsed.Data != nil {
		t.Fatalf("parsed response without data = %#v", parsed)
	}
}

func TestRespInfoMsgPack_JsonAndResponseLifecycle(t *testing.T) {
	recorder := &responseContextRecorder{}
	response := GetRespInfoMsgPack().(*RespInfoMagPack)
	response.Reset(11, "json", map[string]interface{}{"ok": true})
	if err := response.JsonWithCtx(recorder, http.StatusCreated); err != nil {
		t.Fatalf("JsonWithCtx() error = %v", err)
	}
	if recorder.status != http.StatusCreated {
		t.Fatalf("JsonWithCtx() status = %d", recorder.status)
	}
	value, ok := recorder.jsonValue.(map[string]interface{})
	if !ok || value["code"] != 11 || value["msg"] != "json" || value["data"] == nil {
		t.Fatalf("JSON value = %#v", recorder.jsonValue)
	}

	withoutDataRecorder := &responseContextRecorder{}
	withoutData := GetRespInfoMsgPack().(*RespInfoMagPack)
	withoutData.Reset(12, "empty", nil)
	if err := withoutData.JsonWithCtx(withoutDataRecorder); err != nil {
		t.Fatalf("JsonWithCtx() without data error = %v", err)
	}
	withoutDataValue := withoutDataRecorder.jsonValue.(map[string]interface{})
	if _, exists := withoutDataValue["data"]; exists {
		t.Fatalf("JSON without data contains data field: %#v", withoutDataValue)
	}

	source := NewRespInfo(13, "source", "copied")
	target := GetRespInfoMsgPack().(*RespInfoMagPack)
	target.From(source, true)
	if target.GetCode() != 13 || target.GetMsg() != "source" || target.GetData() != "copied" {
		t.Fatalf("From() target = code=%d msg=%q data=%#v", target.GetCode(), target.GetMsg(), target.GetData())
	}
	if source.Code != 0 || source.Msg != "" || source.Data != nil {
		t.Fatalf("From() did not release source: %#v", source)
	}
	target.SuccessWithData("new data").ErrorCustom(14, "custom")
	if target.GetCode() != 14 || target.GetMsg() != "custom" || target.GetData() != "new data" {
		t.Fatalf("lifecycle target = code=%d msg=%q data=%#v", target.GetCode(), target.GetMsg(), target.GetData())
	}
	target.Release()

	pooled := GetRespInfoMsgPack().(*RespInfoMagPack)
	if pooled.GetCode() != 0 || pooled.GetMsg() != "" || pooled.GetData() != nil {
		t.Fatalf("pooled response was not reset: code=%d msg=%q data=%#v", pooled.GetCode(), pooled.GetMsg(), pooled.GetData())
	}
	pooled.Release()
}

func TestRespInfoMsgPack_PropagatesMarshalAndContextErrors(t *testing.T) {
	response := GetRespInfoMsgPack().(*RespInfoMagPack)
	response.Reset(1, "unsupported", make(chan int))
	if err := response.SendWithCtx(&responseContextRecorder{}); err == nil {
		t.Fatal("SendWithCtx() marshal error = nil")
	}
	if response.GetCode() != 0 || response.GetData() != nil {
		t.Fatal("marshal failure did not release response")
	}

	wantErr := errors.New("write failed")
	sendResponse := GetRespInfoMsgPack().(*RespInfoMagPack)
	sendResponse.Reset(2, "send", nil)
	if err := sendResponse.SendWithCtx(&responseContextRecorder{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("SendWithCtx() error = %v, want %v", err, wantErr)
	}
	jsonResponse := GetRespInfoMsgPack().(*RespInfoMagPack)
	jsonResponse.Reset(3, "json", nil)
	if err := jsonResponse.JsonWithCtx(&responseContextRecorder{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("JsonWithCtx() error = %v, want %v", err, wantErr)
	}
}

func TestParseMsgPackResponse_MalformedAndTypedDecodeErrors(t *testing.T) {
	if _, err := ParseMsgPackResponse([]byte{0xc1}); err == nil {
		t.Fatal("ParseMsgPackResponse(malformed) error = nil")
	}
	encoded, err := msgpack.Marshal(map[string]interface{}{
		"code": int64(1),
		"msg":  "ok",
		"data": map[string]interface{}{"age": "not an integer"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded msgPackPayload
	if _, err := ParseMsgPackResponseWithType(encoded, &decoded); err == nil {
		t.Fatal("ParseMsgPackResponseWithType() decode error = nil")
	}
	parsed, err := ParseMsgPackResponseWithType(encoded, nil)
	if err != nil || parsed.Data == nil {
		t.Fatalf("ParseMsgPackResponseWithType(nil) = (%#v, %v)", parsed, err)
	}
}
