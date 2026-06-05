package version

var (
	// Version is set by release builds with -ldflags "-X .../internal/version.Version=<version>".
	Version = "dev"
	// Commit is set by release builds with -ldflags "-X .../internal/version.Commit=<commit>".
	Commit = "unknown"
	// BuildDate is set by release builds with -ldflags "-X .../internal/version.BuildDate=<date>".
	BuildDate = "unknown"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

func Info() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}
}
