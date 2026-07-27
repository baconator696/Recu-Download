package playlist

import "testing"

// A uid value shorter than 6 chars used to panic with "slice bounds out of
// range [:6]" instead of falling back to returning the original url like
// the other malformed-parameter cases already do.
func TestAppendCheckShortUidDoesNotPanic(t *testing.T) {
	url := "https://example.com/frag.ts?uid=abc&expires=1234567890&request_id=abcdef"
	if _, err := appendCheck(url); err == nil {
		t.Errorf("expected an error for a short uid, got nil")
	}
}

func TestAppendCheckShortRequestIdDoesNotPanic(t *testing.T) {
	url := "https://example.com/frag.ts?uid=abcdefghi&expires=1234567890&request_id=ab"
	if _, err := appendCheck(url); err == nil {
		t.Errorf("expected an error for a short request_id, got nil")
	}
}

func TestAppendCheckShortExpiresDoesNotPanic(t *testing.T) {
	url := "https://example.com/frag.ts?uid=abcdefghi&expires=12&request_id=abcdef"
	if _, err := appendCheck(url); err == nil {
		t.Errorf("expected an error for a short expires, got nil")
	}
}

func TestAppendCheckWellFormed(t *testing.T) {
	url := "https://example.com/frag.ts?uid=abcdefghi&expires=1234567890&request_id=abcdef"
	appended, err := appendCheck(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appended == url {
		t.Errorf("expected the url to have a check param appended, got %q", appended)
	}
}
