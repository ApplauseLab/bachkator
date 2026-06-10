package runner

import "testing"

func TestTargetFingerprintIncludesLock(t *testing.T) {
	project := &Project{Root: t.TempDir()}
	target := shellTarget("shell/test", "true")
	withoutLock, _, err := targetFingerprintParts(project, target, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	target.SpecValue.Runtime.Lock = "postgres"
	withLock, _, err := targetFingerprintParts(project, target, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if withoutLock == withLock {
		t.Fatal("fingerprint should change when lock changes")
	}
}
