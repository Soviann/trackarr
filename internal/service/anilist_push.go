package service

// DeriveSeasonState maps PlexTracker's title status + per-season episode
// counts to AniList's MediaListStatus + progress.
//
// COMPLETED wins over DROPPED: if every episode is watched we report
// COMPLETED regardless of the title's dropped status.
func DeriveSeasonState(titleStatus string, totalEpisodes, watchedEpisodes int) (status string, progress int) {
	progress = watchedEpisodes

	if totalEpisodes > 0 && watchedEpisodes >= totalEpisodes {
		return "COMPLETED", progress
	}
	if titleStatus == "dropped" {
		return "DROPPED", progress
	}
	if watchedEpisodes == 0 {
		if titleStatus == "plan_to_watch" {
			return "PLANNING", 0
		}
		return "CURRENT", 0
	}
	return "CURRENT", progress
}

// ShouldPushRating returns true when the derived status warrants a rating
// push — the user has formed an opinion (finished or abandoned).
func ShouldPushRating(derivedStatus string) bool {
	return derivedStatus == "COMPLETED" || derivedStatus == "DROPPED"
}
