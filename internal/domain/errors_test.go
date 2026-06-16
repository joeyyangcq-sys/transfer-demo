package domain

import (
	"errors"
	"testing"
)

// TestAccountNotFound checks the message names the specific account and that the
// error still satisfies errors.Is(ErrAccountNotFound) via Unwrap, so the HTTP
// status mapping keeps treating it as a 404.
// TestAccountNotFound 验证消息带上具体账户 id，且通过 Unwrap 仍满足
// errors.Is(ErrAccountNotFound)，使 HTTP 状态码映射继续按 404 处理。
func TestAccountNotFound(t *testing.T) {
	err := AccountNotFound(123)

	if got, want := err.Error(), "account 123 not found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("errors.Is(%v, ErrAccountNotFound) = false, want true", err)
	}

	var notFound *AccountNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("errors.As(%v, *AccountNotFoundError) = false, want true", err)
	}
	if notFound.ID != 123 {
		t.Errorf("ID = %d, want 123", notFound.ID)
	}
}
