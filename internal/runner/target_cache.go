package runner

import (
	"os"
	"strings"
)

func targetCacheable(target *Target) bool {
	return target.Spec().Cacheable()
}

func targetFresh(target *Target, root string, record StateRecord, fingerprint string) bool {
	if record.Fingerprint == "" || record.Fingerprint != fingerprint {
		return false
	}
	for _, output := range target.Outputs {
		if _, err := os.Stat(absPath(root, output)); err != nil {
			return false
		}
	}
	return true
}

func targetStaleStatus(
	target *Target,
	root string,
	record StateRecord,
	fingerprint string,
	parts map[string]string,
	force bool,
) string {
	if !targetCacheable(target) {
		return ""
	}
	reasons := targetStaleReasons(target, root, record, fingerprint, parts, force)
	if len(reasons) == 0 {
		return ""
	}
	return "stale: " + strings.Join(reasons, ", ")
}

func targetStaleReasons(
	target *Target,
	root string,
	record StateRecord,
	fingerprint string,
	parts map[string]string,
	force bool,
) []string {
	var reasons []string
	if force {
		reasons = append(reasons, "forced run")
	}
	if record.Fingerprint == "" {
		reasons = append(reasons, "no cache record")
	} else if record.Fingerprint != fingerprint {
		if len(record.FingerprintParts) == 0 {
			reasons = append(reasons, "fingerprint changed")
		} else {
			if record.FingerprintParts["inputs"] != "" &&
				record.FingerprintParts["inputs"] != parts["inputs"] {
				reasons = append(reasons, "changed input")
			}
			if record.FingerprintParts["env"] != "" &&
				record.FingerprintParts["env"] != parts["env"] {
				reasons = append(reasons, "changed env var")
			}
			previousOperation := record.FingerprintParts["operation"]
			if previousOperation == "" {
				previousOperation = record.FingerprintParts["command"]
			}
			currentOperation := parts["operation"]
			if currentOperation == "" {
				currentOperation = parts["command"]
			}
			if previousOperation != "" && previousOperation != currentOperation {
				reasons = append(reasons, "changed operation")
			}
			if record.FingerprintParts["dependencies"] != "" &&
				record.FingerprintParts["dependencies"] != parts["dependencies"] {
				reasons = append(reasons, "dependency fingerprint change")
			}
		}
	}
	for _, output := range target.Outputs {
		if _, err := os.Stat(absPath(root, output)); err != nil {
			reasons = append(reasons, "missing output")
			break
		}
	}
	if len(reasons) == 0 && record.Fingerprint != fingerprint {
		reasons = append(reasons, "fingerprint changed")
	}
	return dedupeStrings(reasons)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var deduped []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		deduped = append(deduped, value)
	}
	return deduped
}
