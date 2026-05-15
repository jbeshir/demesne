package sandbox

import "testing"

func TestBuildNetworkPolicy(t *testing.T) {
	t.Run("none denies everything", func(t *testing.T) {
		p, err := BuildNetworkPolicy(EgressNone)
		if err != nil {
			t.Fatal(err)
		}
		if p.DefaultAction != "deny" {
			t.Fatalf("default action = %q, want deny", p.DefaultAction)
		}
		if len(p.Egress) != 0 {
			t.Fatalf("expected no egress rules, got %d", len(p.Egress))
		}
	})

	t.Run("package-managers allows registry hosts", func(t *testing.T) {
		p, err := BuildNetworkPolicy(EgressPackageManagers)
		if err != nil {
			t.Fatal(err)
		}
		if p.DefaultAction != "deny" {
			t.Fatalf("default action = %q, want deny", p.DefaultAction)
		}
		if len(p.Egress) != len(PackageManagerHosts) {
			t.Fatalf("expected %d egress rules, got %d", len(PackageManagerHosts), len(p.Egress))
		}
		for i, host := range PackageManagerHosts {
			if p.Egress[i].Action != "allow" {
				t.Fatalf("rule %d action = %q, want allow", i, p.Egress[i].Action)
			}
			if p.Egress[i].Target != host {
				t.Fatalf("rule %d target = %q, want %q", i, p.Egress[i].Target, host)
			}
		}
	})

	t.Run("unknown mode rejected", func(t *testing.T) {
		if _, err := BuildNetworkPolicy(EgressMode("anywhere")); err == nil {
			t.Fatal("expected error for unknown mode")
		}
	})
}
