package response

import (
	"testing"

	"google.golang.org/protobuf/proto"
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
