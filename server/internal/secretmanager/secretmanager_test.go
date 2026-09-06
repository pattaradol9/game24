package secretmanager

import "testing"

func TestResolveName(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_PROJECT", "")

	cases := []struct {
		ref  string
		want string
	}{
		{"projects/my-proj/secrets/game24-encryption-key", "projects/my-proj/secrets/game24-encryption-key/versions/latest"},
		{"  projects/my-proj/secrets/game24-encryption-key\n", "projects/my-proj/secrets/game24-encryption-key/versions/latest"},
		{"projects/my-proj/secrets/game24-encryption-key/versions/3", "projects/my-proj/secrets/game24-encryption-key/versions/3"},
	}
	for _, c := range cases {
		got, err := ResolveName(c.ref)
		if err != nil || got != c.want {
			t.Fatalf("ResolveName(%q) = %q, %v; want %q", c.ref, got, err, c.want)
		}
	}
}

func TestResolveNameBareWithProject(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "proj-a")
	got, err := ResolveName("game24-encryption-key")
	if err != nil || got != "projects/proj-a/secrets/game24-encryption-key/versions/latest" {
		t.Fatalf("got %q, %v", got, err)
	}

	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_PROJECT", "proj-b")
	got, err = ResolveName("game24_key-1")
	if err != nil || got != "projects/proj-b/secrets/game24_key-1/versions/latest" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResolveNameErrors(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GOOGLE_PROJECT", "")

	bad := []string{
		"",
		"   ",
		"projects/p",             // missing secret
		"projects/p/secrets",     // truncated
		"projects/p/secrets/s/v", // bad version segment
		"game24 encryption key",  // invalid characters
		"with/slash",             // slash in bare name
		"bare-name",              // bare but no project env set
	}
	for _, ref := range bad {
		if _, err := ResolveName(ref); err == nil {
			t.Fatalf("ResolveName(%q) accepted invalid reference", ref)
		}
	}
}
