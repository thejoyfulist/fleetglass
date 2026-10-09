package version

// Values are injected by the release build. Development builds remain identifiable.
var Version = "dev"
var Commit = "unknown"
var Date = "unknown"

func String() string { return "Fleetglass " + Version + " (commit " + Commit + ", built " + Date + ")" }
