package util

import "testing"

func TestPasswordHashVerifyAndStrongPasswordMessagesExact(t *testing.T) {
	hash, err := HashPassword("GoodPass123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "GoodPass123" || !VerifyPassword("GoodPass123", hash) || VerifyPassword("WrongPass123", hash) {
		t.Fatalf("password hash/verify mismatch: hash=%q", hash)
	}
	if errs := ValidateStrongPassword("short"); len(errs) == 0 {
		t.Fatal("short password should fail the strong password policy")
	}
	for _, valid := range []string{"Password1", "Password!", "1234567!"} {
		if errs := ValidateStrongPassword(valid); len(errs) != 0 {
			t.Fatalf("password %q should pass, got %#v", valid, errs)
		}
	}
}
