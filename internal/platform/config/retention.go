package config

import "fmt"

// RetentionConfig controls the data lifecycle jobs that permanently remove old rows and files.
type RetentionConfig struct {
	// Enabled (PURGE_ENABLED) registers the lifecycle jobs. They are off by default.
	Enabled bool `env:"PURGE_ENABLED" envDefault:"false"`
	// DryRun (PURGE_DRY_RUN) makes every job count what it would delete without deleting anything. It is on by
	// default: set PURGE_DRY_RUN=false once the counts look right.
	DryRun bool `env:"PURGE_DRY_RUN" envDefault:"true"`
	// SoftDeletedDays is how long a logically deleted row is kept before it is removed for good.
	SoftDeletedDays int `env:"PURGE_RETENTION_DAYS" envDefault:"30"`
	// ReadNotificationDays is how long read notifications are kept.
	ReadNotificationDays int `env:"PURGE_NOTIFICATION_RETENTION_DAYS" envDefault:"180"`
	// FingerprintDays is how long the known sign-in fingerprints are kept after their last use.
	FingerprintDays int `env:"PURGE_FINGERPRINT_RETENTION_DAYS" envDefault:"90"`
	// DeleteOrphans (PURGE_DELETE_ORPHANS) lets the orphan file check delete the files that have no attachment
	// row. When false the check only reports them.
	DeleteOrphans bool `env:"PURGE_DELETE_ORPHANS" envDefault:"false"`
}

// Validate rejects windows that would delete rows immediately or never make sense.
func (r RetentionConfig) Validate() error {
	for name, days := range map[string]int{
		"PURGE_RETENTION_DAYS":              r.SoftDeletedDays,
		"PURGE_NOTIFICATION_RETENTION_DAYS": r.ReadNotificationDays,
		"PURGE_FINGERPRINT_RETENTION_DAYS":  r.FingerprintDays,
	} {
		if days < 1 || days > 3650 {
			return fmt.Errorf("config: %s must be between 1 and 3650 days, got %d", name, days)
		}
	}
	return nil
}
