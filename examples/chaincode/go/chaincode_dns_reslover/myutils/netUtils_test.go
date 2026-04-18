package myutils

import (
	"testing"
)

func TestDNSHelpersRequireLiveBindServer(t *testing.T) {
	t.Skip("integration test: requires a live Bind9 server reachable from the test environment")
}
