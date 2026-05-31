package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestUserAgent_IncludesOSArchAndGoVersion(t *testing.T) {
	ua := UserAgent()
	if !strings.HasPrefix(ua, "nahook-cli/") {
		t.Errorf("expected User-Agent to start with nahook-cli/, got %q", ua)
	}
	if !strings.Contains(ua, runtime.GOOS) {
		t.Errorf("expected User-Agent to mention GOOS %q, got %q", runtime.GOOS, ua)
	}
	if !strings.Contains(ua, runtime.GOARCH) {
		t.Errorf("expected User-Agent to mention GOARCH %q, got %q", runtime.GOARCH, ua)
	}
	if !strings.Contains(ua, runtime.Version()) {
		t.Errorf("expected User-Agent to mention Go version %q, got %q", runtime.Version(), ua)
	}
}

func TestClientHeader_StructuredFormat(t *testing.T) {
	got := ClientHeader()
	if !strings.HasPrefix(got, "cli/") {
		t.Errorf("expected X-Nahook-Client to start with cli/, got %q", got)
	}
	if !strings.Contains(got, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("expected X-Nahook-Client to contain %q, got %q",
			runtime.GOOS+"/"+runtime.GOARCH, got)
	}
}
