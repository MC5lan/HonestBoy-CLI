package errs

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorCodes(t *testing.T) {
	if len(Errors) < 6 {
		t.Fatal("error catalog too small")
	}
	err := New(CfgInvalid, "bad %s", "key")
	if CodeOf(err) != CfgInvalid || !strings.Contains(err.Error(), "HB-CFG-001") {
		t.Fatalf("err = %v", err)
	}
	cause := errors.New("dial failed")
	wrapped := Wrap(NetUnavailable, cause, "connect")
	if CodeOf(wrapped) != NetUnavailable || !errors.Is(wrapped, cause) {
		t.Fatalf("wrap = %v", wrapped)
	}
	if CodeOf(errors.New("plain")) != "" {
		t.Fatal("plain error has code")
	}
	msg, ok := Explain("hb-net-001")
	if !ok || !strings.Contains(msg, "端点") {
		t.Fatalf("explain = %q %v", msg, ok)
	}
	if _, ok := Explain("HB-XXX-999"); ok {
		t.Fatal("unknown code accepted")
	}
}
