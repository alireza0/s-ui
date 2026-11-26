package nodecert

import "testing"

func requireNodeTestSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
