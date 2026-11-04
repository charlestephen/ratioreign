package config

import "testing"

func TestEnsureAuthCredentialsGeneratesOnce(t *testing.T) {
	c := defaults()
	c.Auth.Password = ""
	gen, pw, err := c.EnsureAuthCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if !gen || pw == "" || c.Auth.Password != pw {
		t.Fatalf("expected generated password, got gen=%v pw=%q stored=%q", gen, pw, c.Auth.Password)
	}
	gen2, _, err := c.EnsureAuthCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if gen2 {
		t.Fatal("second EnsureAuthCredentials should not regenerate")
	}
}

func TestEnsureAuthCredentialsSkipsWhenDisabled(t *testing.T) {
	c := defaults()
	c.Auth.Enabled = false
	c.Auth.Password = ""
	gen, _, err := c.EnsureAuthCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if gen || c.Auth.Password != "" {
		t.Fatal("disabled auth should not generate a password")
	}
}
