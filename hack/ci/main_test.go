package main

import "testing"

const (
	testAhead     = "ahead"
	testIdentical = "identical"
	testDigest    = "abc123"
)

func TestValidateAncestry(t *testing.T) {
	tests := []struct {
		name     string
		statuses []string
		wantErr  bool
	}{
		{name: "both ahead", statuses: []string{testAhead, testAhead}},
		{name: "both identical", statuses: []string{testIdentical, testIdentical}},
		{name: "mixed accepted", statuses: []string{testAhead, testIdentical}},
		{name: "missing result", statuses: []string{testAhead}, wantErr: true},
		{name: "missing status", statuses: []string{testAhead, ""}, wantErr: true},
		{name: "diverged", statuses: []string{testAhead, "diverged"}, wantErr: true},
		{name: "behind", statuses: []string{"behind", testAhead}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAncestry(test.statuses)
			if (err != nil) != test.wantErr {
				t.Fatalf(
					"validateAncestry(%v) error = %v, wantErr %t",
					test.statuses,
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestValidateAssetDigest(t *testing.T) {
	tests := []struct {
		name      string
		published string
		digest    string
		wantErr   bool
	}{
		{name: "matching sha256", published: "sha256:" + testDigest, digest: testDigest},
		{name: "missing published digest", digest: testDigest, wantErr: true},
		{name: "mismatched digest", published: "sha256:def456", digest: testDigest, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAssetDigest(test.published, test.digest)
			if (err != nil) != test.wantErr {
				t.Fatalf(
					"validateAssetDigest(%q, %q) error = %v, wantErr %t",
					test.published,
					test.digest,
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestValidTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{tag: "v1.2.3", want: true},
		{tag: "v1.25.0-beta.8", want: true},
		{tag: "v1.2.3-RC.1", want: true},
		{tag: "1.2.3", want: false},
		{tag: "v1.2", want: false},
		{tag: "v1.2.3+build.1", want: false},
		{tag: "v1.2.3-", want: false},
	}
	for _, test := range tests {
		t.Run(test.tag, func(t *testing.T) {
			if got := validTag(test.tag); got != test.want {
				t.Fatalf("validTag(%q) = %t, want %t", test.tag, got, test.want)
			}
		})
	}
}
