package fsevents

import "testing"

func TestCreatePath(t *testing.T) {
	ref, err := createPaths([]string{"/a", "/b"})
	if err != nil {
		t.Fatal(err)
	}

	if e := 2; CFArrayLen(ref) != e {
		t.Errorf("got: %d wanted: %d", CFArrayLen(ref), e)
	}
}
