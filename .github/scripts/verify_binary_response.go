//go:build ignore

// verify_binary_response.go 断言 GET /examples/binary 三条协商路径的 body 确实可解码，
// 且 data 内容完整。
//
// 单看 Content-Type 不足以证明协商成功：曾出现过 Content-Type 正确、但 data 被静默
// 丢弃的情况——RespInfoPB.Reset 以 structpb.NewValue 转换 data，该函式不接受自定义
// struct，而 Reset 的两处 if err == nil 都没有 else 分支，转换失败时 data 静默留为
// nil（response/response_proto_impl.go:109-117）。当时 Protobuf body 只剩 4 bytes，
// code 与 data 全失，仅 msg 存活。本脚本因此逐个解码并断言 data 的两个键都在。
//
// 以 `go run ./.github/scripts/verify_binary_response.go` 执行；build tag ignore
// 使其不参与 `go build ./...` 与 `go vet ./...`。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	responsepb "github.com/lamxy/fiberhouse/response/pb"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func main() {
	msgpackPath := flag.String("msgpack", "", "path to the msgpack response body")
	protobufPath := flag.String("protobuf", "", "path to the protobuf response body")
	jsonPath := flag.String("json", "", "path to the json fallback response body")
	flag.Parse()

	var failures []string

	if err := verifyMsgpack(*msgpackPath); err != nil {
		failures = append(failures, fmt.Sprintf("msgpack: %v", err))
	}
	if err := verifyProtobuf(*protobufPath); err != nil {
		failures = append(failures, fmt.Sprintf("protobuf: %v", err))
	}
	if err := verifyJSON(*jsonPath); err != nil {
		failures = append(failures, fmt.Sprintf("json fallback: %v", err))
	}

	if len(failures) > 0 {
		for _, failure := range failures {
			fmt.Fprintln(os.Stderr, failure)
		}
		os.Exit(1)
	}

	fmt.Println("binary response negotiation verified: msgpack, protobuf and json fallback all decode with complete data")
}

// verifyMsgpack 断言 msgpack body 解出 code/msg/data，且 data 两个键都在。
func verifyMsgpack(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var envelope map[string]interface{}
	if err := msgpack.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if _, ok := envelope["code"]; !ok {
		return fmt.Errorf("envelope missing 'code': %v", envelope)
	}
	if msg, _ := envelope["msg"].(string); msg == "" {
		return fmt.Errorf("envelope missing 'msg': %v", envelope)
	}

	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("envelope 'data' missing or not a map: %v", envelope)
	}
	return verifyEchoPayload(data, "application/msgpack")
}

// verifyProtobuf 断言 protobuf body 解出 RespInfoProto，且 data 的 Any 能还原成
// 含两个键的 structpb.Value——这正是此前静默丢失的部分。
func verifyProtobuf(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	envelope := &responsepb.RespInfoProto{}
	if err := proto.Unmarshal(body, envelope); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if envelope.GetMsg() == "" {
		return fmt.Errorf("envelope missing 'msg'")
	}
	if envelope.GetData() == nil {
		return fmt.Errorf("envelope 'data' is nil — structpb conversion was silently dropped")
	}

	value := &structpb.Value{}
	if err := envelope.GetData().UnmarshalTo(value); err != nil {
		return fmt.Errorf("unmarshal data Any into structpb.Value: %w", err)
	}

	structValue := value.GetStructValue()
	if structValue == nil {
		return fmt.Errorf("data Value is not a struct: %v", value)
	}

	data := make(map[string]interface{}, len(structValue.GetFields()))
	for key, field := range structValue.GetFields() {
		data[key] = field.AsInterface()
	}
	return verifyEchoPayload(data, "application/x-protobuf")
}

// verifyJSON 断言未协商时回退 JSON，且 data 完整。
func verifyJSON(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var envelope struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	if envelope.Msg == "" {
		return fmt.Errorf("envelope missing 'msg': %s", body)
	}
	if envelope.Data == nil {
		return fmt.Errorf("envelope missing 'data': %s", body)
	}

	// 回退路径由 curl 的默认 Accept: */* 命中通配符分支。
	return verifyEchoPayload(envelope.Data, "*/*")
}

// verifyEchoPayload 断言 data 的 format 与 echo 两个键都存在且非空。
//
// format 回显的是 handler 读到的协商 MIME，因此可用来确认走的确实是预期分支。
func verifyEchoPayload(data map[string]interface{}, wantFormat string) error {
	format, _ := data["format"].(string)
	if format == "" {
		return fmt.Errorf("data missing 'format': %v", data)
	}
	if format != wantFormat {
		return fmt.Errorf("data 'format' = %q, want %q", format, wantFormat)
	}

	echo, _ := data["echo"].(string)
	if echo == "" {
		return fmt.Errorf("data missing 'echo': %v", data)
	}
	return nil
}
