package str

// Levenshtein returns the edit distance between a and b using a single rolling
// row, comparing by rune so multi-byte characters are measured correctly.
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr := make([]int, len(rb)+1)
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(rb)]
}

// Closest returns the candidate string closest to target by Levenshtein edit
// distance, provided that distance is at most maxDistance.
func Closest(target string, candidates []string, maxDistance int) (string, bool) {
	best := ""
	bestDist := maxDistance + 1
	for _, candidate := range candidates {
		if dist := Levenshtein(target, candidate); dist < bestDist {
			bestDist, best = dist, candidate
		}
	}
	if best == "" {
		return "", false
	}
	return best, true
}
