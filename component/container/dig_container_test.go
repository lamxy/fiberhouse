package container

import "testing"

type testDependency struct {
	value string
}

func TestNewDigContainerProvideAndInvoke(t *testing.T) {
	dc := NewDigContainer().Provide(func() *testDependency {
		return &testDependency{value: "resolved"}
	})
	if got := dc.GetErrorCount(); got != 0 {
		t.Fatalf("GetErrorCount() = %d, want 0; errors: %v", got, dc.GetProvideErrs())
	}

	var resolved *testDependency
	if err := dc.Invoke(func(dep *testDependency) {
		resolved = dep
	}); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if resolved == nil || resolved.value != "resolved" {
		t.Fatalf("resolved dependency = %#v, want value %q", resolved, "resolved")
	}
}

func TestWrapSetGet(t *testing.T) {
	wrap := NewWrap[*testDependency]()
	if got := wrap.Get(); got != nil {
		t.Fatalf("new Wrap.Get() = %#v, want nil", got)
	}

	want := &testDependency{value: "wrapped"}
	wrap.Set(want)
	if got := wrap.Get(); got != want {
		t.Fatalf("Wrap.Get() = %#v, want %#v", got, want)
	}
}
