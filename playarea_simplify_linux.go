//go:build linux
// +build linux

package main

// SteamVR's compositor re-uploads the chaperone wall mesh every frame while the
// bounds are visible, 480 bytes per wall. Above 136 walls (> 65535 bytes) that
// upload leaves the vkCmdUpdateBuffer path and goes through the 256 MB transfer
// ring, which fills in ~30 s and crashes vrcompositor ("Vulkan transfer ring
// buffer overflow", or a GPU fault on SteamVR 2.19.1) - the headset goes dark
// right after the controllers turn on. A hand-drawn Room Setup outline easily has
// 150+ walls, so zones are simplified (Douglas-Peucker on the floor outline,
// smallest tolerance that fits) before they are applied. 98 walls ≈ 6 mm error.

import (
	"encoding/json"
	"math"
)

const maxChaperoneWalls = 100

type pt2 struct{ x, z float64 }

func dist2(a, b pt2) float64 { return math.Hypot(a.x-b.x, a.z-b.z) }

func douglasPeucker(pts []pt2, eps float64) []pt2 {
	if len(pts) < 3 {
		return pts
	}
	a, b := pts[0], pts[len(pts)-1]
	dmax, idx := 0.0, 0
	dx, dz := b.x-a.x, b.z-a.z
	l := math.Hypot(dx, dz)
	for i := 1; i < len(pts)-1; i++ {
		p := pts[i]
		var d float64
		if l > 1e-9 {
			d = math.Abs(dz*(p.x-a.x)-dx*(p.z-a.z)) / l
		} else {
			d = dist2(p, a)
		}
		if d > dmax {
			dmax, idx = d, i
		}
	}
	if dmax > eps {
		left := douglasPeucker(pts[:idx+1], eps)
		right := douglasPeucker(pts[idx:], eps)
		return append(append([]pt2{}, left[:len(left)-1]...), right...)
	}
	return []pt2{a, b}
}

// simplifyRing simplifies a closed outline (no repeated last point).
func simplifyRing(pts []pt2, eps float64) []pt2 {
	far := 0
	for i := range pts {
		if dist2(pts[0], pts[i]) > dist2(pts[0], pts[far]) {
			far = i
		}
	}
	a := douglasPeucker(pts[:far+1], eps)
	closed := append(append([]pt2{}, pts[far:]...), pts[0])
	b := douglasPeucker(closed, eps)
	return append(append([]pt2{}, a[:len(a)-1]...), b[:len(b)-1]...)
}

type wallSeg struct {
	p, q   pt2
	bottom float64
	height float64
}

// simplifyChaperoneBlob returns the blob with every universe's collision_bounds
// cut to at most maxChaperoneWalls walls, the number of walls before/after, and
// whether anything changed. Blobs it does not understand are returned as is.
func simplifyChaperoneBlob(blob string) (string, int, int, bool) {
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(blob), &doc); err != nil {
		return blob, 0, 0, false
	}
	us, _ := doc["universes"].([]interface{})
	before, after, changed := 0, 0, false
	for _, ui := range us {
		u, _ := ui.(map[string]interface{})
		walls, _ := u["collision_bounds"].([]interface{})
		before += len(walls)
		segs, ok := parseWalls(walls)
		if !ok || len(segs) <= maxChaperoneWalls {
			after += len(walls)
			continue
		}
		// split into closed loops of connected walls
		var loops [][]wallSeg
		cur := []wallSeg{segs[0]}
		for _, s := range segs[1:] {
			if dist2(s.p, cur[len(cur)-1].q) < 1e-4 {
				cur = append(cur, s)
			} else {
				loops = append(loops, cur)
				cur = []wallSeg{s}
			}
		}
		loops = append(loops, cur)

		var out []interface{}
		for eps := 0.002; eps < 1; eps *= 1.25 {
			out = out[:0]
			for _, lp := range loops {
				isClosed := len(lp) > 3 && dist2(lp[0].p, lp[len(lp)-1].q) < 1e-4
				if !isClosed {
					for _, s := range lp {
						out = append(out, makeWall(s.p, s.q, s.bottom, s.height))
					}
					continue
				}
				pts := make([]pt2, len(lp))
				for i, s := range lp {
					pts[i] = s.p
				}
				sp := simplifyRing(pts, eps)
				for i := range sp {
					out = append(out, makeWall(sp[i], sp[(i+1)%len(sp)], lp[0].bottom, lp[0].height))
				}
			}
			if len(out) <= maxChaperoneWalls {
				break
			}
		}
		u["collision_bounds"] = out
		after += len(out)
		changed = true
	}
	if !changed {
		return blob, before, after, false
	}
	b, err := json.MarshalIndent(doc, "", "   ")
	if err != nil {
		return blob, before, before, false
	}
	return string(b), before, after, true
}

func num(v interface{}) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// parseWalls reads walls of the form [[x,y0,z],[x,y1,z],[x2,y1,z2],[x2,y0,z2]].
func parseWalls(walls []interface{}) ([]wallSeg, bool) {
	segs := make([]wallSeg, 0, len(walls))
	for _, wi := range walls {
		w, _ := wi.([]interface{})
		if len(w) != 4 {
			return nil, false
		}
		var c [4][3]float64
		for i := 0; i < 4; i++ {
			p, _ := w[i].([]interface{})
			if len(p) != 3 {
				return nil, false
			}
			for j := 0; j < 3; j++ {
				f, ok := num(p[j])
				if !ok {
					return nil, false
				}
				c[i][j] = f
			}
		}
		segs = append(segs, wallSeg{
			p: pt2{c[0][0], c[0][2]}, q: pt2{c[3][0], c[3][2]},
			bottom: c[0][1], height: c[1][1],
		})
	}
	return segs, len(segs) > 0
}

func makeWall(p, q pt2, bottom, height float64) []interface{} {
	return []interface{}{
		[]interface{}{p.x, bottom, p.z},
		[]interface{}{p.x, height, p.z},
		[]interface{}{q.x, height, q.z},
		[]interface{}{q.x, bottom, q.z},
	}
}
