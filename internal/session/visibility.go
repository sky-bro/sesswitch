package session

// VisibleSessions keeps provider-confirmed non-resumable records out of normal
// history. Retain an actionable request only when its original CLI is reachable.
// The registry remains intact: no saved data alone does not prove /side origin.
func VisibleSessions(items []Session) []Session {
	visible := make([]Session, 0, len(items))
	for _, item := range items {
		if item.Resume == "unavailable" {
			if item.Location == nil || (item.Task != nil && item.Task.Kind == "done") {
				continue
			}
			if item.State.Kind != "needs_approval" && item.State.Kind != "needs_input" {
				continue
			}
		}
		visible = append(visible, item)
	}
	return visible
}
