package exception

import (
	"errors"
	"net/http"
	"testing"

	"github.com/lamxy/fiberhouse/constant"
	"github.com/lamxy/fiberhouse/globalmanager"
	"github.com/lamxy/fiberhouse/response"
)

func installExceptionMap(t *testing.T, exceptions ExceptionMap) {
	t.Helper()
	manager := globalmanager.NewGlobalManagerOnce()
	registryKey := constant.RegisterKeyPrefix + "exceptions"
	wasRegistered := manager.IsRegistered(registryKey)
	var previous interface{}
	if wasRegistered {
		previous, _ = manager.Get(registryKey)
	}
	manager.Clear(registryKey)
	if !manager.Register(registryKey, func() (interface{}, error) { return exceptions, nil }) {
		t.Fatalf("register temporary exception map")
	}
	t.Cleanup(func() {
		manager.Clear(registryKey)
		if wasRegistered {
			manager.Register(registryKey, func() (interface{}, error) { return previous, nil })
		}
	})
}

func recoverPanic(t *testing.T, fn func()) interface{} {
	t.Helper()
	var recovered interface{}
	func() {
		defer func() { recovered = recover() }()
		fn()
	}()
	if recovered == nil {
		t.Fatal("function did not panic")
	}
	return recovered
}

type exceptionContextRecorder struct {
	status int
	code   int
	msg    string
	data   interface{}
	body   []byte
}

func (c *exceptionContextRecorder) GetCtx() interface{}      { return nil }
func (c *exceptionContextRecorder) GetHeader(string) string  { return "" }
func (c *exceptionContextRecorder) SetHeader(string, string) {}
func (c *exceptionContextRecorder) Send(status int, body []byte) error {
	c.status = status
	c.body = append([]byte(nil), body...)
	return nil
}
func (c *exceptionContextRecorder) JSON(status int, value interface{}) error {
	c.status = status
	if responseValue, ok := value.(interface {
		GetCode() int
		GetMsg() string
		GetData() interface{}
	}); ok {
		c.code = responseValue.GetCode()
		c.msg = responseValue.GetMsg()
		c.data = responseValue.GetData()
	}
	return nil
}

func TestThrow_UsesRequestedKeyAndErrorData(t *testing.T) {
	installExceptionMap(t, ExceptionMap{
		"requested": {Code: 4101, Msg: "requested message"},
	})

	recovered := recoverPanic(t, func() { Throw("requested", errors.New("details")) })
	exception, ok := recovered.(*Exception)
	if !ok {
		t.Fatalf("panic value type = %T, want *Exception", recovered)
	}
	defer exception.Release()
	if exception.Code != 4101 || exception.Msg != "requested message" || exception.Data != "details" {
		t.Fatalf("panic exception = %#v", exception)
	}
}

func TestVeThrow_UsesRequestedKeyAndData(t *testing.T) {
	installExceptionMap(t, ExceptionMap{
		"requested": {Code: 4201, Msg: "validation message"},
	})

	recovered := recoverPanic(t, func() { VeThrow("requested", "field") })
	exception, ok := recovered.(*ValidateException)
	if !ok {
		t.Fatalf("panic value type = %T, want *ValidateException", recovered)
	}
	defer exception.Release()
	if exception.Code != 4201 || exception.Msg != "validation message" || exception.Data != "field" {
		t.Fatalf("panic exception = %#v", exception)
	}
}

func TestGetAndVeGet_KnownUnknownAndConvenienceKeys(t *testing.T) {
	installExceptionMap(t, ExceptionMap{
		"known":            {Code: 4001, Msg: "known", Data: "default"},
		"InputParamError":  {Code: 4002, Msg: "input"},
		"NotFoundDocument": {Code: 4003, Msg: "not found"},
		"IllegalRequest":   {Code: 4004, Msg: "illegal"},
		"InternalError":    {Code: 5001, Msg: "internal"},
		"UnknownError":     {Code: 5002, Msg: "unknown configured"},
	})

	known := Get("known")
	if known.Code != 4001 || known.Msg != "known" || known.Data != "default" {
		t.Fatalf("Get(known) = %#v", known)
	}
	known.Release()
	unknown := Get("not-registered")
	if unknown.Code != constant.UnknownErrCode || unknown.Msg != constant.UnknownErrMsg {
		t.Fatalf("Get(unknown) = %#v", unknown)
	}
	unknown.Release()
	validation := VeGet("known")
	if validation.Code != 4001 || validation.Msg != "known" {
		t.Fatalf("VeGet(known) = %#v", validation)
	}
	validation.Release()

	for name, exception := range map[string]*Exception{
		"input":     GetInputError(),
		"not found": GetNotFoundDocument(),
		"illegal":   GetIllegalRequest(),
		"internal":  GetInternalError(),
		"unknown":   GetUnknownError(),
	} {
		if exception.Msg == "" {
			t.Fatalf("%s convenience exception has empty message", name)
		}
		exception.Release()
	}
	for name, exception := range map[string]*ValidateException{
		"input":     VeGetInputError(),
		"not found": VeGetNotFoundError(),
		"internal":  VeGetInternalError(),
		"unknown":   VeGetUnknownError(),
	} {
		if exception.Msg == "" {
			t.Fatalf("%s validation exception has empty message", name)
		}
		exception.Release()
	}
}

func TestGet_MissingRegistryPanicsWithUsefulError(t *testing.T) {
	manager := globalmanager.NewGlobalManagerOnce()
	registryKey := constant.RegisterKeyPrefix + "exceptions"
	wasRegistered := manager.IsRegistered(registryKey)
	var previous interface{}
	if wasRegistered {
		previous, _ = manager.Get(registryKey)
	}
	manager.Clear(registryKey)
	t.Cleanup(func() {
		manager.Clear(registryKey)
		if wasRegistered {
			manager.Register(registryKey, func() (interface{}, error) { return previous, nil })
		}
	})

	for name, fn := range map[string]func(){
		"Get":     func() { Get("known") },
		"Throw":   func() { Throw("known") },
		"VeGet":   func() { VeGet("known") },
		"VeThrow": func() { VeThrow("known") },
	} {
		t.Run(name, func(t *testing.T) {
			recovered := recoverPanic(t, fn)
			err, ok := recovered.(error)
			if !ok || err == nil {
				t.Fatalf("panic value = %#v, want error", recovered)
			}
		})
	}
}

func TestException_ResponseLifecycleAndContextStatus(t *testing.T) {
	exception := New(1001, "created", errors.New("initial"))
	if exception.Error() != "created" || exception.GetCode() != 1001 || exception.GetData() != "initial" {
		t.Fatalf("New() = %#v", exception)
	}
	exception.RespData(errors.New("details"))
	if exception.Data != "details" {
		t.Fatalf("RespData(error) = %#v", exception.Data)
	}
	exception.RespData(map[string]int{"field": 1})
	if _, ok := exception.Data.(map[string]int); !ok {
		t.Fatalf("RespData(value) type = %T", exception.Data)
	}
	exception.Reset(1002, "reset", "value")
	if exception.GetCode() != 1002 || exception.GetMsg() != "reset" || exception.GetData() != "value" {
		t.Fatalf("Reset() = %#v", exception)
	}
	exception.SuccessWithData("ignored")
	exception.ErrorCustom(1003, "custom")
	if exception.Code != 1003 || exception.Msg != "custom" {
		t.Fatalf("ErrorCustom() = %#v", exception)
	}

	source := response.NewRespInfo(1004, "source", "copied")
	exception.From(source, true)
	if exception.Code != 1004 || exception.Msg != "source" || exception.Data != "copied" {
		t.Fatalf("From() = %#v", exception)
	}
	if source.Code != 0 || source.Msg != "" || source.Data != nil {
		t.Fatalf("released source = %#v", source)
	}
	exception.Release()

	jsonRecorder := &exceptionContextRecorder{}
	jsonException := New(1101, "json", "payload")
	if err := jsonException.JsonWithCtx(jsonRecorder); err != nil {
		t.Fatalf("JsonWithCtx() error = %v", err)
	}
	if jsonRecorder.status != http.StatusOK || jsonRecorder.code != 1101 || jsonRecorder.msg != "json" || jsonRecorder.data != "payload" {
		t.Fatalf("JSON record = %#v", jsonRecorder)
	}
	if jsonException.Code != 0 || jsonException.Msg != "" || jsonException.Data != nil {
		t.Fatalf("JsonWithCtx() did not release exception: %#v", jsonException)
	}

	sendRecorder := &exceptionContextRecorder{}
	sendException := New(1102, "send")
	if err := sendException.SendWithCtx(sendRecorder, http.StatusTeapot); err != nil {
		t.Fatalf("SendWithCtx() error = %v", err)
	}
	if sendRecorder.status != http.StatusTeapot || sendRecorder.code != 1102 {
		t.Fatalf("SendWithCtx() record = %#v", sendRecorder)
	}
}

func TestValidateException_ResponseLifecycleAndPanic(t *testing.T) {
	exception := NewVE(2001, "validation", errors.New("invalid"))
	if exception.Error() != "validation" || exception.GetData() != "invalid" {
		t.Fatalf("NewVE() = %#v", exception)
	}
	exception.RespData("field")
	exception.Reset(2002, "reset", "data")
	exception.SuccessWithData("ignored")
	exception.ErrorCustom(2003, "custom")
	if exception.GetCode() != 2003 || exception.GetMsg() != "custom" || exception.GetData() != "data" {
		t.Fatalf("validate lifecycle = %#v", exception)
	}
	source := response.NewRespInfo(2004, "source", "copied")
	exception.From(source, true)
	if exception.Code != 2004 || source.Code != 0 {
		t.Fatalf("ValidateException.From() target=%#v source=%#v", exception, source)
	}
	exception.Release()

	jsonRecorder := &exceptionContextRecorder{}
	jsonException := NewVE(2101, "json", "payload")
	if err := jsonException.JsonWithCtx(jsonRecorder, http.StatusUnprocessableEntity); err != nil {
		t.Fatalf("JsonWithCtx() error = %v", err)
	}
	if jsonRecorder.status != http.StatusUnprocessableEntity || jsonRecorder.code != 2101 {
		t.Fatalf("JSON record = %#v", jsonRecorder)
	}
	sendRecorder := &exceptionContextRecorder{}
	sendException := NewVE(2102, "send")
	if err := sendException.SendWithCtx(sendRecorder); err != nil {
		t.Fatalf("SendWithCtx() error = %v", err)
	}
	if sendRecorder.status != http.StatusOK || sendRecorder.code != 2102 {
		t.Fatalf("Send record = %#v", sendRecorder)
	}

	panicException := NewVE(2201, "panic")
	recovered := recoverPanic(t, panicException.Panic)
	if recovered != panicException {
		t.Fatalf("Panic() value = %#v", recovered)
	}
	panicException.Release()
	plainPanic := New(2202, "panic")
	recovered = recoverPanic(t, plainPanic.Panic)
	if recovered != plainPanic {
		t.Fatalf("Exception.Panic() value = %#v", recovered)
	}
	plainPanic.Release()
}
