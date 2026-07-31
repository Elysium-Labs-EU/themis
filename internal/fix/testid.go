package fix

// THEMIS-* test IDs shared between this package's fixes and
// internal/native's findings for the same check, so checkreport.Build's
// pairing can't drift out of sync between the two definitions.
const (
	Fail2banTestID           = "THEMIS-FAIL2BAN"
	UnattendedUpgradesTestID = "THEMIS-UNATTENDED-UPGRADES"
)
