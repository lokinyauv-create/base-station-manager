//go:build linux

package main

import (
	"encoding/json"
	"math"
	"testing"
)

// A hand-drawn-like closed outline with 200 walls must come out at
// <= maxChaperoneWalls walls, and a second pass must change nothing.
func TestSimplifyChaperone(t *testing.T) {
	var walls []interface{}
	n := 200
	for i := 0; i < n; i++ {
		a, b := 2*math.Pi*float64(i)/float64(n), 2*math.Pi*float64(i+1)/float64(n)
		r := func(x float64) float64 { return 1.5 + 0.2*math.Sin(5*x) }
		walls = append(walls, makeWall(pt2{r(a) * math.Cos(a), r(a) * math.Sin(a)},
			pt2{r(b) * math.Cos(b), r(b) * math.Sin(b)}, 0, 2.43))
	}
	doc := map[string]interface{}{"jsonid": "chaperone_info", "version": 5.0,
		"universes": []interface{}{map[string]interface{}{"collision_bounds": walls, "universeID": "1"}}}
	blob, _ := json.Marshal(doc)
	s, before, after, ok := simplifyChaperoneBlob(string(blob))
	if !ok || before != n || after > maxChaperoneWalls || after < 20 {
		t.Fatalf("ok=%v %d -> %d", ok, before, after)
	}
	if _, b2, a2, ok2 := simplifyChaperoneBlob(s); ok2 || b2 != a2 {
		t.Fatal("second pass should be a no-op")
	}
}
