package telco

import "testing"

func TestSuite(t *testing.T) {
	suite := Suite()
	if len(suite) < 27 {
		t.Fatalf("expected at least 27 production profiles, got %d", len(suite))
	}

	// Core fast-diagnosis operators must be present
	for _, want := range []OperatorID{OperatorOADP, OperatorTALM, OperatorIDMS, OperatorACM} {
		if _, ok := ProfileByID(want); !ok {
			t.Errorf("missing operator %s in suite", want)
		}
	}
	if _, ok := ProfileByID(OperatorMCH); !ok {
		t.Error("missing legacy MCH alias")
	}
}

func TestCoreSuite(t *testing.T) {
	core := CoreSuite()
	if len(core) != 4 {
		t.Fatalf("expected 4 core operators, got %d", len(core))
	}
}

func TestProfileByPackage(t *testing.T) {
	cases := []struct {
		pkg  string
		want OperatorID
	}{
		{"redhat-oadp-operator", OperatorOADP},
		{"advanced-cluster-management", OperatorACM},
		{"multicluster-engine", OperatorMCE},
		{"odf-operator", OperatorODF},
		{"lvms-operator", OperatorLVMS},
		{"topology-aware-lifecycle-manager", OperatorTALM},
		{"kubernetes-nmstate-operator", OperatorNMState},
		{"sriov-network-operator", OperatorSRIOV},
	}

	for _, tc := range cases {
		p, ok := ProfileByPackage(tc.pkg)
		if !ok {
			t.Errorf("ProfileByPackage(%q): not found", tc.pkg)
			continue
		}
		if p.ID != tc.want {
			t.Errorf("ProfileByPackage(%q): got ID %s, want %s", tc.pkg, p.ID, tc.want)
		}
	}

	_, ok := ProfileByPackage("nonexistent")
	if ok {
		t.Fatal("expected false for unknown package")
	}
}

func TestPackageNames(t *testing.T) {
	names := PackageNames()
	// 27 OLM packages (excludes IDMS cluster config)
	if len(names) != 27 {
		t.Fatalf("expected 27 OLM packages, got %d", len(names))
	}

	seen := make(map[string]bool)
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate package name: %s", n)
		}
		seen[n] = true
	}
}

func TestODFPackageNames(t *testing.T) {
	odf := ODFPackageNames()
	if len(odf) != 11 {
		t.Fatalf("expected 11 ODF packages, got %d: %v", len(odf), odf)
	}
}

func TestProfilesByCategory(t *testing.T) {
	odf := ProfilesByCategory(CategoryODF)
	if len(odf) != 11 {
		t.Errorf("ODF category: got %d profiles, want 11", len(odf))
	}

	net := ProfilesByCategory(CategoryNetworking)
	if len(net) != 5 {
		t.Errorf("Networking category: got %d profiles, want 5", len(net))
	}
}

func TestAllProfilesUniquePackages(t *testing.T) {
	for _, p := range AllProfiles() {
		if p.PackageName == "" {
			continue // IDMS
		}
		if p.DefaultNS == "" {
			t.Errorf("%s (%s): missing DefaultNS", p.ID, p.PackageName)
		}
		if p.DisplayName == "" {
			t.Errorf("%s (%s): missing DisplayName", p.ID, p.PackageName)
		}
	}
}
