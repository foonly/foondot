package config

import "testing"

func TestValidateConfigSyncStrategy(t *testing.T) {
	for _, strategy := range []string{"manual", "local", "remote"} {
		if err := validateConfig(Config{SyncStrategy: strategy}); err != nil {
			t.Errorf("%s: unexpected error %v", strategy, err)
		}
	}
	for _, strategy := range []string{"remot", "Local", "theirs"} {
		if err := validateConfig(Config{SyncStrategy: strategy}); err == nil {
			t.Errorf("%s: expected an error", strategy)
		}
	}
}
