package vector

import (
	"math"
	"sort"
)

type Item struct {
	ThreadID int64
	Vector   []float64
}

type Neighbor struct {
	ThreadID int64   `json:"thread_id"`
	Score    float64 `json:"score"`
}

func Query(items []Item, query []float64, limit int, excludeThreadID int64) []Neighbor {
	if limit <= 0 {
		limit = 20
	}
	out := make([]Neighbor, 0, len(items))
	for _, item := range items {
		if item.ThreadID == excludeThreadID {
			continue
		}
		score := Cosine(query, item.Vector)
		if math.IsNaN(score) || math.IsInf(score, 0) || score <= 0 {
			continue
		}
		out = append(out, Neighbor{ThreadID: item.ThreadID, Score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ThreadID < out[j].ThreadID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func Cosine(left, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var leftMaxAbs, rightMaxAbs float64
	for index := range left {
		leftValue := left[index]
		rightValue := right[index]
		if math.IsNaN(leftValue) || math.IsNaN(rightValue) || math.IsInf(leftValue, 0) || math.IsInf(rightValue, 0) {
			return 0
		}
		leftMaxAbs = max(leftMaxAbs, math.Abs(leftValue))
		rightMaxAbs = max(rightMaxAbs, math.Abs(rightValue))
	}
	if leftMaxAbs == 0 || rightMaxAbs == 0 {
		return 0
	}
	var dot, leftMag, rightMag float64
	for index := range left {
		leftValue := left[index] / leftMaxAbs
		rightValue := right[index] / rightMaxAbs
		dot += leftValue * rightValue
		leftMag += leftValue * leftValue
		rightMag += rightValue * rightValue
	}
	if leftMag == 0 || rightMag == 0 {
		return 0
	}
	score := dot / (math.Sqrt(leftMag) * math.Sqrt(rightMag))
	if score > 1 {
		return 1
	}
	if score < -1 {
		return -1
	}
	return score
}
