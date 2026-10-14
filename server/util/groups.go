package util

// JudgeGroupWeights returns how heavily each group should be weighted when assigning judges.
// If proportional is false, all groups are weighted equally. Otherwise, each group is weighted
// by its number of projects, falling back to the configured group sizes if there are no projects yet.
func JudgeGroupWeights(numGroups int64, proportional bool, projectCounts []int64, groupSizes []int64) []float64 {
	weights := make([]float64, numGroups)
	if !proportional {
		for i := range weights {
			weights[i] = 1
		}
		return weights
	}

	// Weight by the number of projects in each group
	total := int64(0)
	for i := range weights {
		if i < len(projectCounts) {
			weights[i] = float64(projectCounts[i])
			total += projectCounts[i]
		}
	}

	// No projects yet, so use the configured group sizes
	if total == 0 {
		for i := range weights {
			if i < len(groupSizes) {
				weights[i] = float64(groupSizes[i])
			}
		}
	}

	return weights
}

// AllocateGroups returns the groups that the next n judges should be placed in,
// given the current number of judges in each group and the weight of each group.
// Each judge goes to the group with the highest weight / (2 * count + 1) (Sainte-Laguë),
// so judges are split in proportion to the weights. With equal weights, this simply
// fills the group with the fewest judges. Ties go to the lowest group number.
// If every weight is 0, all groups are weighted equally.
func AllocateGroups(counts []int64, weights []float64, n int) []int64 {
	numGroups := len(counts)
	if numGroups == 0 {
		return make([]int64, n)
	}

	// Copy counts so the caller's slice is not modified
	c := make([]int64, numGroups)
	copy(c, counts)

	// Fall back to equal weights if no group has a positive weight
	w := make([]float64, numGroups)
	total := 0.0
	for i := range w {
		if i < len(weights) && weights[i] > 0 {
			w[i] = weights[i]
			total += w[i]
		}
	}
	if total == 0 {
		for i := range w {
			w[i] = 1
		}
	}

	groups := make([]int64, n)
	for j := 0; j < n; j++ {
		best := 0
		bestScore := -1.0
		for i := 0; i < numGroups; i++ {
			score := w[i] / float64(2*c[i]+1)
			if score > bestScore {
				best = i
				bestScore = score
			}
		}
		groups[j] = int64(best)
		c[best]++
	}

	return groups
}

// TrackJudgeGroupWeights uses only this track's eligible active projects. Empty
// rooms receive no judges. Before projects exist, use configured sizes (or equal
// weights when proportional assignment is off) as a provisional allocation.
func TrackJudgeGroupWeights(numGroups int64, proportional bool, projectCounts []int64, groupSizes []int64) []float64 {
	weights := JudgeGroupWeights(numGroups, proportional, projectCounts, groupSizes)
	total := int64(0)
	for _, count := range projectCounts {
		total += count
	}
	if total > 0 {
		for i := range weights {
			if i >= len(projectCounts) || projectCounts[i] == 0 {
				weights[i] = 0
			}
		}
	}
	return weights
}

// AllocateTrackGroups first covers each eligible room, when staffing permits,
// then uses the existing proportional allocator with those judges counted.
// If fewer judges than rooms exist, the highest-weight uncovered rooms go first.
func AllocateTrackGroups(counts []int64, weights []float64, n int) []int64 {
	c := append([]int64(nil), counts...)
	groups := make([]int64, 0, n)
	for len(groups) < n {
		uncovered := -1
		for i, count := range c {
			if count == 0 && i < len(weights) && weights[i] > 0 &&
				(uncovered == -1 || weights[i] > weights[uncovered]) {
				uncovered = i
			}
		}
		if uncovered == -1 {
			break
		}
		groups = append(groups, int64(uncovered))
		c[uncovered]++
	}
	return append(groups, AllocateGroups(c, weights, n-len(groups))...)
}
