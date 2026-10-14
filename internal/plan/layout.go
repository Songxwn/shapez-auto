package plan

import (
	"container/heap"
	"fmt"
)

// Placement 是游戏里的一次放置。坐标是格子，旋转单位是度。
type Placement struct {
	ID              string `json:"id"`
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Rotation        int    `json:"rotation"`
	RotationVariant int    `json:"rotationVariant"`
	Variant         string `json:"variant"`
}

// Resource 是地图上的一块资源。
type Resource struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Kind string `json:"kind"`
	Key  string `json:"key"`
}

type occMap map[pos]bool

func (m occMap) clone() occMap {
	out := make(occMap, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

type world struct {
	occ      occMap
	reserved occMap
	places   []Placement
	res      []Resource
	used     map[pos]bool
}

func newWorld(resources []Resource) *world {
	w := &world{
		occ:      occMap{},
		reserved: occMap{},
		used:     map[pos]bool{},
		res:      resources,
	}
	for x := -2; x <= 1; x++ {
		for y := -2; y <= 1; y++ {
			w.occ[pos{x, y}] = true
		}
	}
	for _, r := range resources {
		w.reserved[pos{r.X, r.Y}] = true
	}
	return w
}

func (w *world) blocked(p pos) bool {
	return w.occ[p] || w.reserved[p]
}

func (w *world) base() occMap {
	out := w.occ.clone()
	for p := range w.reserved {
		out[p] = true
	}
	return out
}

func inBounds(p pos) bool {
	return p.X >= -80 && p.X <= 160 && p.Y >= -80 && p.Y <= 220
}

// Layout 把加工树摆到地图上，并把成品送进中枢南侧。
func Layout(op *Op, resources []Resource) ([]Placement, error) {
	w := newWorld(resources)
	landing, err := w.layout(op)
	if err != nil {
		return nil, err
	}
	belts, ok := route(w.base(), landing.At, landing.In, pos{0, 2}, "north")
	if !ok {
		return nil, fmt.Errorf("无法把成品送到中枢")
	}
	w.commit(belts)
	return w.places, nil
}

type outlet struct {
	At pos
	In string
}

func (w *world) layout(op *Op) (outlet, error) {
	switch op.Kind {
	case "mineShape":
		return w.mine("shape", shapeKey(op.Shape))
	case "mineColor":
		return w.mine("color", op.Color)
	case "rotCW":
		return w.rotators(op)
	case "cutLeft":
		return w.cut(op, true)
	case "cutRight":
		return w.cut(op, false)
	case "paint":
		return w.paint(op)
	case "stack":
		return w.binary(op, "stacker")
	case "mix":
		return w.binary(op, "mixer")
	default:
		return outlet{}, fmt.Errorf("布局不支持 %s", op.Kind)
	}
}

func shapeKey(shape byte) string {
	switch shape {
	case 'C':
		return "CuCuCuCu"
	case 'R':
		return "RuRuRuRu"
	case 'S':
		return "SuSuSuSu"
	case 'W':
		return "WuWuWuWu"
	default:
		return ""
	}
}

func (w *world) mine(kind, key string) (outlet, error) {
	if key == "" {
		return outlet{}, fmt.Errorf("未知资源")
	}
	type face struct {
		rot    int
		dx, dy int
		travel string
	}
	faces := []face{
		{90, 1, 0, "east"},
		{180, 0, 1, "south"},
		{0, 0, -1, "north"},
		{270, -1, 0, "west"},
	}
	best := -1
	bestD := int(^uint(0) >> 1)
	for i, r := range w.res {
		p := pos{r.X, r.Y}
		if w.used[p] || r.Kind != kind || r.Key != key || w.occ[p] {
			continue
		}
		d := abs(r.X) + abs(r.Y)
		if d < bestD {
			bestD = d
			best = i
		}
	}
	if best < 0 {
		return outlet{}, fmt.Errorf("附近没有足够的 %s 资源", key)
	}
	origin := pos{w.res[best].X, w.res[best].Y}
	for _, f := range faces {
		landing := pos{origin.X + f.dx, origin.Y + f.dy}
		if !inBounds(landing) || w.blocked(landing) {
			continue
		}
		w.places = append(w.places, Placement{
			ID: "miner", X: origin.X, Y: origin.Y,
			Rotation: f.rot, Variant: "default",
		})
		w.occ[origin] = true
		w.used[origin] = true
		return outlet{At: landing, In: opposite(f.travel)}, nil
	}
	return outlet{}, fmt.Errorf("资源 (%d,%d) 四周都被挡住了", origin.X, origin.Y)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (w *world) rotators(op *Op) (outlet, error) {
	child, err := w.layout(op.A)
	if err != nil {
		return outlet{}, err
	}
	n := op.N
	if n <= 0 {
		n = 1
	}
	occupy := make([]pos, 0, n)
	for i := 0; i < n; i++ {
		occupy = append(occupy, pos{0, -i})
	}
	origin, belts, err := w.placeConnected(occupy, []pos{{0, -n}}, pos{0, 1}, "north", child)
	if err != nil {
		return outlet{}, fmt.Errorf("放不下旋转器: %w", err)
	}
	for i := 0; i < n; i++ {
		w.places = append(w.places, Placement{
			ID: "rotater", X: origin.X, Y: origin.Y - i, Variant: "default",
		})
		w.occ[pos{origin.X, origin.Y - i}] = true
	}
	w.commit(belts)
	return outlet{At: pos{origin.X, origin.Y - n}, In: "south"}, nil
}

func (w *world) cut(op *Op, left bool) (outlet, error) {
	child, err := w.layout(op.A)
	if err != nil {
		return outlet{}, err
	}
	keep := pos{1, -1}
	trash := pos{0, -1}
	if left {
		keep = pos{0, -1}
		trash = pos{1, -1}
	}
	origin, belts, err := w.placeConnected(
		[]pos{{0, 0}, {1, 0}, trash},
		[]pos{keep},
		pos{0, 1},
		"north",
		child,
	)
	if err != nil {
		return outlet{}, fmt.Errorf("放不下切割器: %w", err)
	}
	w.places = append(w.places, Placement{
		ID: "cutter", X: origin.X, Y: origin.Y, Variant: "default",
	})
	w.occ[pos{origin.X, origin.Y}] = true
	w.occ[pos{origin.X + 1, origin.Y}] = true
	trashAt := pos{origin.X + trash.X, origin.Y + trash.Y}
	w.places = append(w.places, Placement{
		ID: "trash", X: trashAt.X, Y: trashAt.Y, Variant: "default",
	})
	w.occ[trashAt] = true
	w.commit(belts)
	return outlet{At: pos{origin.X + keep.X, origin.Y + keep.Y}, In: "south"}, nil
}

func (w *world) paint(op *Op) (outlet, error) {
	shapeOut, err := w.layout(op.A)
	if err != nil {
		return outlet{}, err
	}
	colorOut, err := w.layout(op.B)
	if err != nil {
		return outlet{}, err
	}
	occupy := []pos{{0, 0}, {0, -1}}
	protect := []pos{{0, -2}}
	free := []pos{{0, 0}, {0, -1}, {0, 1}, {-1, -1}, {0, -2}}
	var origin pos
	var shapeBelts, colorBelts []Placement
	found := false
	for _, candidate := range w.candidates(free, shapeOut.At) {
		trial := w.base()
		for _, t := range occupy {
			trial[pos{candidate.X + t.X, candidate.Y + t.Y}] = true
		}
		for _, t := range protect {
			trial[pos{candidate.X + t.X, candidate.Y + t.Y}] = true
		}
		sb, ok1 := route(trial, shapeOut.At, shapeOut.In, pos{candidate.X, candidate.Y + 1}, "north")
		if !ok1 {
			continue
		}
		for _, belt := range sb {
			trial[pos{belt.X, belt.Y}] = true
		}
		cb, ok2 := route(trial, colorOut.At, colorOut.In, pos{candidate.X - 1, candidate.Y - 1}, "east")
		if !ok2 {
			continue
		}
		origin = candidate
		shapeBelts = sb
		colorBelts = cb
		found = true
		break
	}
	if !found {
		return outlet{}, fmt.Errorf("放不下染色器或接不上染料")
	}
	w.places = append(w.places, Placement{
		ID: "painter", X: origin.X, Y: origin.Y, Rotation: 270, Variant: "default",
	})
	w.occ[pos{origin.X, origin.Y}] = true
	w.occ[pos{origin.X, origin.Y - 1}] = true
	w.commit(shapeBelts)
	w.commit(colorBelts)
	return outlet{At: pos{origin.X, origin.Y - 2}, In: "south"}, nil
}

func (w *world) binary(op *Op, id string) (outlet, error) {
	lower, err := w.layout(op.A)
	if err != nil {
		return outlet{}, err
	}
	upper, err := w.layout(op.B)
	if err != nil {
		return outlet{}, err
	}
	occupy := []pos{{0, 0}, {1, 0}}
	free := []pos{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, -1}}
	var origin pos
	var leftBelts, rightBelts []Placement
	found := false
	for _, candidate := range w.candidates(free, lower.At) {
		trial := w.base()
		for _, t := range occupy {
			trial[pos{candidate.X + t.X, candidate.Y + t.Y}] = true
		}
		trial[pos{candidate.X, candidate.Y - 1}] = true
		lb, ok1 := route(trial, lower.At, lower.In, pos{candidate.X, candidate.Y + 1}, "north")
		if !ok1 {
			continue
		}
		for _, belt := range lb {
			trial[pos{belt.X, belt.Y}] = true
		}
		rb, ok2 := route(trial, upper.At, upper.In, pos{candidate.X + 1, candidate.Y + 1}, "north")
		if !ok2 {
			continue
		}
		origin = candidate
		leftBelts = lb
		rightBelts = rb
		found = true
		break
	}
	if !found {
		return outlet{}, fmt.Errorf("放不下 %s", id)
	}
	w.places = append(w.places, Placement{
		ID: id, X: origin.X, Y: origin.Y, Variant: "default",
	})
	w.occ[pos{origin.X, origin.Y}] = true
	w.occ[pos{origin.X + 1, origin.Y}] = true
	w.commit(leftBelts)
	w.commit(rightBelts)
	return outlet{At: pos{origin.X, origin.Y - 1}, In: "south"}, nil
}

// placeConnected 寻找能把 child 接到 socket 的位置。occupy 会变成建筑，protect 是要留给后续出料的空格。
func (w *world) placeConnected(occupy, protect []pos, socket pos, socketOut string, child outlet) (pos, []Placement, error) {
	must := append([]pos{}, occupy...)
	must = append(must, protect...)
	must = append(must, socket)
	for _, candidate := range w.candidates(must, child.At) {
		trial := w.base()
		for _, t := range occupy {
			trial[pos{candidate.X + t.X, candidate.Y + t.Y}] = true
		}
		for _, t := range protect {
			trial[pos{candidate.X + t.X, candidate.Y + t.Y}] = true
		}
		goal := pos{candidate.X + socket.X, candidate.Y + socket.Y}
		belts, ok := route(trial, child.At, child.In, goal, socketOut)
		if !ok {
			continue
		}
		return candidate, belts, nil
	}
	return pos{}, nil, fmt.Errorf("没有可连接的空位")
}

func (w *world) candidates(rel []pos, near pos) []pos {
	var out []pos
	for r := 0; r <= 120 && len(out) < 80; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if abs(dx) != r && abs(dy) != r {
					continue
				}
				origin := pos{near.X + dx, near.Y + dy}
				ok := true
				for _, t := range rel {
					p := pos{origin.X + t.X, origin.Y + t.Y}
					if !inBounds(p) || w.blocked(p) {
						ok = false
						break
					}
				}
				if ok {
					out = append(out, origin)
					if len(out) >= 80 {
						return out
					}
				}
			}
		}
	}
	return out
}

func (w *world) commit(belts []Placement) {
	w.places = append(w.places, belts...)
	for _, belt := range belts {
		w.occ[pos{belt.X, belt.Y}] = true
	}
}

type routeNode struct {
	x, y int
	in   string
	prev int
	g    int
}

type routeQ struct {
	i   int
	pri int
}

type routePQ []routeQ

func (q routePQ) Len() int           { return len(q) }
func (q routePQ) Less(i, j int) bool { return q[i].pri < q[j].pri }
func (q routePQ) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *routePQ) Push(x any)        { *q = append(*q, x.(routeQ)) }
func (q *routePQ) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}

func route(blocked occMap, start pos, startIn string, goal pos, goalOut string) ([]Placement, bool) {
	if !inBounds(start) || !inBounds(goal) || startIn == "" || goalOut == "" {
		return nil, false
	}
	if blocked[goal] || blocked[start] {
		return nil, false
	}
	queue := []routeNode{{x: start.X, y: start.Y, in: startIn, prev: -1, g: 0}}
	best := map[[3]int]int{{start.X, start.Y, dirCode(startIn)}: 0}
	pq := routePQ{{i: 0, pri: abs(start.X-goal.X) + abs(start.Y-goal.Y)}}
	heap.Init(&pq)
	dirs := []string{"north", "east", "south", "west"}
	pops := 0
	for pq.Len() > 0 {
		item := heap.Pop(&pq).(routeQ)
		cur := queue[item.i]
		key := [3]int{cur.x, cur.y, dirCode(cur.in)}
		if g, ok := best[key]; ok && g < cur.g {
			continue
		}
		pops++
		if pops > 100000 {
			return nil, false
		}
		if cur.x == goal.X && cur.y == goal.Y {
			if _, _, ok := BeltPose(cur.in, goalOut); ok {
				belts := rebuild(queue, item.i, goalOut)
				if len(belts) > 0 {
					return belts, true
				}
			}
			continue
		}
		for _, out := range dirs {
			if _, _, ok := BeltPose(cur.in, out); !ok {
				continue
			}
			nx, ny := step(cur.x, cur.y, out)
			next := pos{nx, ny}
			if !inBounds(next) || blocked[next] {
				continue
			}
			if onPath(queue, item.i, nx, ny) {
				continue
			}
			nextIn := opposite(out)
			nextKey := [3]int{nx, ny, dirCode(nextIn)}
			ng := cur.g + 1
			if old, ok := best[nextKey]; ok && ng >= old {
				continue
			}
			best[nextKey] = ng
			queue = append(queue, routeNode{x: nx, y: ny, in: nextIn, prev: item.i, g: ng})
			pri := ng + abs(nx-goal.X) + abs(ny-goal.Y)
			heap.Push(&pq, routeQ{i: len(queue) - 1, pri: pri})
		}
	}
	return nil, false
}

func onPath(queue []routeNode, at, x, y int) bool {
	for at >= 0 {
		if queue[at].x == x && queue[at].y == y {
			return true
		}
		at = queue[at].prev
	}
	return false
}

func dirCode(dir string) int {
	switch dir {
	case "north":
		return 0
	case "east":
		return 1
	case "south":
		return 2
	default:
		return 3
	}
}

func rebuild(queue []routeNode, at int, goalOut string) []Placement {
	var rev []Placement
	out := goalOut
	for at >= 0 {
		cur := queue[at]
		rot, variant, ok := BeltPose(cur.in, out)
		if !ok {
			return nil
		}
		rev = append(rev, Placement{
			ID: "belt", X: cur.x, Y: cur.y,
			Rotation: rot, RotationVariant: variant, Variant: "default",
		})
		if cur.prev < 0 {
			break
		}
		out = opposite(cur.in)
		at = cur.prev
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}
