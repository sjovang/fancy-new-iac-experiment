package main

import "testing"

// TestMergeListingsPrecedence asserts that later libraries override earlier
// ones on the same file basename and that unique entries from either side
// are preserved. This is the inheritance contract that lets an org library
// extend or override Azure/Azure-Landing-Zones-Library without forking.
func TestMergeListingsPrecedence(t *testing.T) {
	base := &libraryFiles{
		architectures: map[string]string{"alz": "base/arch/alz"},
		archetypes: map[string]string{
			"corp":    "base/arche/corp",
			"online":  "base/arche/online",
		},
		assignments: map[string]string{
			"Enable-DDoS-VNET": "base/assign/ddos",
			"Deny-Public-IP":   "base/assign/denypub",
		},
		definitions: map[string]string{"Audit-PrivateLinkDnsZones": "base/def/audit"},
		defaultsURL: "base/defaults",
	}
	overlay := &libraryFiles{
		architectures: map[string]string{"myarch": "overlay/arch/myarch"},
		archetypes: map[string]string{
			"corp":             "overlay/arche/corp", // overrides base
			"my-extra":         "overlay/arche/my-extra",
		},
		assignments: map[string]string{
			"Deny-Public-IP":     "overlay/assign/denypub", // overrides
			"Custom-Assignment":  "overlay/assign/custom",
		},
		definitions: map[string]string{
			"Audit-PrivateLinkDnsZones": "overlay/def/audit", // overrides
			"My-Custom-Def":             "overlay/def/mine",
		},
		defaultsURL: "overlay/defaults",
	}

	merged := mergeListings([]*libraryFiles{base, overlay})

	cases := []struct {
		got, want, label string
	}{
		{merged.architectures["alz"], "base/arch/alz", "base-only architecture preserved"},
		{merged.architectures["myarch"], "overlay/arch/myarch", "overlay-only architecture added"},
		{merged.archetypes["corp"], "overlay/arche/corp", "overlay archetype overrides base"},
		{merged.archetypes["online"], "base/arche/online", "base-only archetype preserved"},
		{merged.archetypes["my-extra"], "overlay/arche/my-extra", "overlay-only archetype added"},
		{merged.assignments["Enable-DDoS-VNET"], "base/assign/ddos", "base-only assignment preserved"},
		{merged.assignments["Deny-Public-IP"], "overlay/assign/denypub", "overlay assignment overrides base"},
		{merged.assignments["Custom-Assignment"], "overlay/assign/custom", "overlay-only assignment added"},
		{merged.definitions["Audit-PrivateLinkDnsZones"], "overlay/def/audit", "overlay definition overrides base"},
		{merged.definitions["My-Custom-Def"], "overlay/def/mine", "overlay-only definition added"},
		{merged.defaultsURL, "overlay/defaults", "overlay defaults URL wins (per-key merge happens later in loadDefaults)"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.label, c.got, c.want)
		}
	}
}

// TestApplyAddRemove asserts the semantics used by archetypeOverrides.
func TestApplyAddRemove(t *testing.T) {
	out := applyAddRemove([]string{"a", "b", "c"}, []string{"d"}, []string{"b"})
	want := []string{"a", "c", "d"}
	if len(out) != len(want) {
		t.Fatalf("len(out)=%d, want %d (%v vs %v)", len(out), len(want), out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("out[%d]=%q, want %q", i, out[i], want[i])
		}
	}
}
