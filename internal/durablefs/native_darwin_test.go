//go:build darwin

package durablefs

import "testing"

func TestNativeRootPathCanonicalizesVarAlias(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/var", want: "/private/var"},
		{path: "/var/folders/example", want: "/private/var/folders/example"},
		{path: "/variable/example", want: "/variable/example"},
		{path: "/tmp/example", want: "/tmp/example"},
	} {
		if got := nativeRootPath(test.path); got != test.want {
			t.Errorf("nativeRootPath(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}
