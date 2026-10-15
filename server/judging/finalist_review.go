package judging

import "server/models"

// finalistReviewCandidates runs ONLY after active/seen/flag/busy/group eligibility
// filtering. nil means use the original assignment flow, including when nothing
// is shortlisted or every available finalist has reached its completed-view target.
func finalistReviewCandidates(projects []*models.Project, options *models.Options) []*models.Project {
	if !options.FinalistReview {
		return nil
	}
	target := options.FinalistTargetViews
	if target < 1 {
		target = 4
	}
	finalists := []*models.Project{}
	belowMinimum := []*models.Project{}
	for _, p := range projects {
		if p.Seen < options.MinViews {
			belowMinimum = append(belowMinimum, p)
		}
		if p.Prioritized && p.Seen < target {
			finalists = append(finalists, p)
		}
	}
	if len(finalists) == 0 {
		return nil
	}
	if len(belowMinimum) > 0 {
		return fewestSeenCandidates(belowMinimum)
	}
	return fewestSeenCandidates(finalists)
}

// Within coverage or finalist review, favor the least-covered eligible projects.
func fewestSeenCandidates(projects []*models.Project) []*models.Project {
	minSeen := projects[0].Seen
	for _, p := range projects {
		if p.Seen < minSeen {
			minSeen = p.Seen
		}
	}
	candidates := []*models.Project{}
	for _, p := range projects {
		if p.Seen == minSeen {
			candidates = append(candidates, p)
		}
	}
	return candidates
}
