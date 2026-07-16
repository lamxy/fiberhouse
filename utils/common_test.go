package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidConstant_ZeroCheckSupportsNonNilableKinds(t *testing.T) {
	if ValidConstant(0, true) {
		t.Fatal("ValidConstant(0, true) = true, want false")
	}
	if !ValidConstant(1, true) {
		t.Fatal("ValidConstant(1, true) = false, want true")
	}
	if ValidConstant("", true) {
		t.Fatal("ValidConstant(empty string, true) = true, want false")
	}
	if !ValidConstant("value", true) {
		t.Fatal("ValidConstant(non-empty string, true) = false, want true")
	}
}

func TestValidConstant_RejectsTypedNilWithoutPanicking(t *testing.T) {
	var pointer *int
	var values []string
	var mapping map[string]int
	var channel chan int
	var function func()
	for name, value := range map[string]interface{}{
		"pointer":  pointer,
		"slice":    values,
		"map":      mapping,
		"channel":  channel,
		"function": function,
	} {
		t.Run(name, func(t *testing.T) {
			if ValidConstant(value) {
				t.Fatalf("ValidConstant(%s typed nil) = true, want false", name)
			}
		})
	}
}

func TestJSONValidityAndUnicodeWhitespace(t *testing.T) {
	valid := `{"name":"Ada","items":[1,true,null]}`
	invalid := `{"name":}`
	if !JsonValidString(valid) || !JsonValidBytes([]byte(valid)) {
		t.Fatal("valid JSON was rejected")
	}
	if JsonValidString(invalid) || JsonValidBytes([]byte(invalid)) {
		t.Fatal("invalid JSON was accepted")
	}
	input := "\t Hello\u00a0\n世界\u2003  !\r\n"
	if got, want := NormalizeWhitespace(input), " Hello 世界 ! "; got != want {
		t.Fatalf("NormalizeWhitespace() = %q, want %q", got, want)
	}
	if got := NormalizeWhitespace("plain"); got != "plain" {
		t.Fatalf("NormalizeWhitespace(plain) = %q", got)
	}
}

func TestPathsAndFileExistence(t *testing.T) {
	if path := GetExecPath(); path == "" || !filepath.IsAbs(path) {
		t.Fatalf("GetExecPath() = %q, want absolute path", path)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if got := GetWD(); got != workingDirectory {
		t.Fatalf("GetWD() = %q, want %q", got, workingDirectory)
	}

	directory := t.TempDir()
	file := filepath.Join(directory, "present.txt")
	if err := os.WriteFile(file, []byte("present"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	if !FileExists(file) || !FileExists(directory) {
		t.Fatal("FileExists() rejected an existing path")
	}
	if FileExists(filepath.Join(directory, "missing.txt")) {
		t.Fatal("FileExists() accepted a missing path")
	}
}

func TestUnsafeConversions_EmptyAndNonEmpty(t *testing.T) {
	if got := UnsafeString(nil); got != "" {
		t.Fatalf("UnsafeString(nil) = %q", got)
	}
	if got := UnsafeBytes(""); len(got) != 0 {
		t.Fatalf("UnsafeBytes(empty) length = %d", len(got))
	}

	bytes := []byte("hello")
	convertedString := UnsafeString(bytes)
	if convertedString != "hello" {
		t.Fatalf("UnsafeString() = %q", convertedString)
	}
	bytes[0] = 'H'
	if convertedString != "Hello" {
		t.Fatalf("UnsafeString() did not share storage: %q", convertedString)
	}
	convertedBytes := UnsafeBytes("世界")
	if string(convertedBytes) != "世界" {
		t.Fatalf("UnsafeBytes() = %q", convertedBytes)
	}
}

func TestStackHelpers(t *testing.T) {
	stack := StackMsg()
	if !strings.Contains(stack, "TestStackHelpers") || !strings.Contains(stack, "goroutine") {
		t.Fatalf("StackMsg() missing expected frame: %q", stack)
	}
	captured := CaptureStack()
	if !strings.HasPrefix(captured, "stack trace:\n") || !strings.Contains(captured, "testing.tRunner") {
		t.Fatalf("CaptureStack() = %q", captured)
	}
	if lines := DebugStackLines(""); lines != nil {
		t.Fatalf("DebugStackLines(empty) = %#v, want nil", lines)
	}
	if lines := DebugStackLines("single line"); lines != nil {
		t.Fatalf("DebugStackLines(single line) = %#v, want nil", lines)
	}
	lines := DebugStackLines("first\n\tsecond")
	if len(lines) != 2 || lines[1] != "    second" {
		t.Fatalf("DebugStackLines() = %#v", lines)
	}
}

func TestValidConstant_AllZeroCapableKinds(t *testing.T) {
	if ValidConstant(nil) {
		t.Fatal("ValidConstant(nil) = true")
	}
	for name, value := range map[string]interface{}{
		"bool":      false,
		"int":       int(0),
		"uint":      uint(0),
		"float":     float64(0),
		"complex":   complex128(0),
		"string":    "",
		"array":     [1]int{},
		"struct":    struct{ Value int }{},
		"interface": interface{}(nil),
	} {
		t.Run(name, func(t *testing.T) {
			if ValidConstant(value, true) {
				t.Fatalf("ValidConstant(%s zero, true) = true", name)
			}
		})
	}
	value := 1
	for name, candidate := range map[string]interface{}{
		"bool":     true,
		"int":      -1,
		"uint":     uint(1),
		"float":    0.5,
		"complex":  complex(1, 2),
		"string":   "value",
		"array":    [1]int{1},
		"struct":   struct{ Value int }{Value: 1},
		"pointer":  &value,
		"slice":    []int{},
		"map":      map[string]int{},
		"channel":  make(chan int),
		"function": func() {},
	} {
		t.Run(name, func(t *testing.T) {
			if !ValidConstant(candidate, true) {
				t.Fatalf("ValidConstant(%s non-zero, true) = false", name)
			}
		})
	}
	if !ValidConstant(0) || !ValidConstant(0, false) {
		t.Fatal("zero-value checking was applied when not requested")
	}
}
