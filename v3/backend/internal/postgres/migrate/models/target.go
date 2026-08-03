package models

// Target describes the exact database identity allowed for one migration job.
type Target struct {
	Database      string
	Socket        string
	SessionRole   string
	EffectiveRole string
}
