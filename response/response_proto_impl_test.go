package response

import (
	"errors"
	"net/http"
	"testing"

	pb "github.com/lamxy/fiberhouse/response/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestRespInfoPBUsesResponsePBDescriptor(t *testing.T) {
	resp := GetRespInfoPB().(*RespInfoPB)
	defer resp.Release()

	resp.Reset(7, "moved", map[string]any{"ok": true})
	descriptor := resp.pb.ProtoReflect().Descriptor()
	if got, want := descriptor.ParentFile().Path(), "response/pb/resp_info.proto"; got != want {
		t.Fatalf("descriptor path = %q, want %q", got, want)
	}
	if got, want := string(descriptor.FullName()), "response.RespInfoProto"; got != want {
		t.Fatalf("message full name = %q, want %q", got, want)
	}

	body, err := proto.Marshal(resp.pb)
	if err != nil {
		t.Fatalf("marshal RespInfoProto: %v", err)
	}
	decoded := resp.pb.ProtoReflect().New().Interface()
	if err := proto.Unmarshal(body, decoded); err != nil {
		t.Fatalf("unmarshal RespInfoProto: %v", err)
	}
	message := decoded.ProtoReflect()
	if got := message.Get(descriptor.Fields().ByName("code")).Int(); got != 7 {
		t.Fatalf("round-trip code = %d, want 7", got)
	}
	if got := message.Get(descriptor.Fields().ByName("msg")).String(); got != "moved" {
		t.Fatalf("round-trip msg = %q, want %q", got, "moved")
	}
}

func TestRespInfoPB_SendWithCtxRoundTripAndStatus(t *testing.T) {
	recorder := &responseContextRecorder{}
	response := GetRespInfoPB().(*RespInfoPB)
	response.Reset(21, "protobuf", map[string]interface{}{"name": "Ada", "active": true})
	if err := response.SendWithCtx(recorder); err != nil {
		t.Fatalf("SendWithCtx() error = %v", err)
	}
	if recorder.status != http.StatusOK {
		t.Fatalf("SendWithCtx() status = %d, want %d", recorder.status, http.StatusOK)
	}
	decoded := &pb.RespInfoProto{}
	if err := proto.Unmarshal(recorder.body, decoded); err != nil {
		t.Fatalf("proto.Unmarshal() error = %v", err)
	}
	if decoded.Code != 21 || decoded.Msg != "protobuf" || decoded.Data == nil {
		t.Fatalf("decoded protobuf = %#v", decoded)
	}
	if response.GetCode() != 0 || response.GetMsg() != "" || response.GetData() != nil {
		t.Fatalf("SendWithCtx() did not release response: code=%d msg=%q data=%#v", response.GetCode(), response.GetMsg(), response.GetData())
	}

	customRecorder := &responseContextRecorder{}
	custom := GetRespInfoPB().(*RespInfoPB)
	custom.Reset(22, "accepted", nil)
	if err := custom.SendWithCtx(customRecorder, http.StatusAccepted); err != nil {
		t.Fatalf("custom SendWithCtx() error = %v", err)
	}
	if customRecorder.status != http.StatusAccepted {
		t.Fatalf("custom status = %d, want %d", customRecorder.status, http.StatusAccepted)
	}
}

func TestRespInfoPB_JsonWithCtxAndLifecycle(t *testing.T) {
	recorder := &responseContextRecorder{}
	response := GetRespInfoPB().(*RespInfoPB)
	response.Reset(31, "json", []interface{}{"one", true})
	if err := response.JsonWithCtx(recorder, http.StatusCreated); err != nil {
		t.Fatalf("JsonWithCtx() error = %v", err)
	}
	if recorder.status != http.StatusCreated {
		t.Fatalf("JsonWithCtx() status = %d", recorder.status)
	}
	value, ok := recorder.jsonValue.(map[string]interface{})
	if !ok || value["code"] != 31 || value["msg"] != "json" || value["data"] == nil {
		t.Fatalf("JSON value = %#v", recorder.jsonValue)
	}

	source := NewRespInfo(32, "source", map[string]interface{}{"copied": true})
	target := GetRespInfoPB().(*RespInfoPB)
	target.From(source, true)
	if target.GetCode() != 32 || target.GetMsg() != "source" || target.GetData() == nil {
		t.Fatalf("From() target = code=%d msg=%q data=%#v", target.GetCode(), target.GetMsg(), target.GetData())
	}
	if source.Code != 0 || source.Msg != "" || source.Data != nil {
		t.Fatalf("From() did not release source: %#v", source)
	}
	target.SuccessWithData("new data").ErrorCustom(33, "custom")
	if target.GetCode() != 33 || target.GetMsg() != "custom" || target.GetData() != "new data" {
		t.Fatalf("lifecycle target = code=%d msg=%q data=%#v", target.GetCode(), target.GetMsg(), target.GetData())
	}
	target.SuccessWithData(nil)
	if target.GetData() != nil {
		t.Fatalf("SuccessWithData(nil) data = %#v", target.GetData())
	}
	target.Release()

	pooled := GetRespInfoPB().(*RespInfoPB)
	if pooled.GetCode() != 0 || pooled.GetMsg() != "" || pooled.GetData() != nil {
		t.Fatalf("pooled response was not reset: code=%d msg=%q data=%#v", pooled.GetCode(), pooled.GetMsg(), pooled.GetData())
	}
	pooled.pb.Data = &anypb.Any{TypeUrl: "invalid", Value: []byte{0xff}}
	if pooled.GetData() != nil {
		t.Fatalf("GetData() invalid Any = %#v, want nil", pooled.GetData())
	}
	ReleaseRespInfoPB(pooled)
}

func TestRespInfoPB_PropagatesContextErrors(t *testing.T) {
	wantErr := errors.New("write failed")
	sendResponse := GetRespInfoPB().(*RespInfoPB)
	sendResponse.Reset(41, "send", nil)
	if err := sendResponse.SendWithCtx(&responseContextRecorder{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("SendWithCtx() error = %v, want %v", err, wantErr)
	}
	jsonResponse := GetRespInfoPB().(*RespInfoPB)
	jsonResponse.Reset(42, "json", nil)
	if err := jsonResponse.JsonWithCtx(&responseContextRecorder{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("JsonWithCtx() error = %v, want %v", err, wantErr)
	}
}
