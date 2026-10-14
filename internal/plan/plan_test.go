package plan

import "testing"

func TestBeltRoundTrip(t *testing.T) {
	dirs := []string{"north", "east", "south", "west"}
	valid := 0
	for _, in := range dirs {
		for _, out := range dirs {
			rot, variant, ok := BeltPose(in, out)
			if !ok {
				continue
			}
			valid++
			gotIn, gotOut := BeltIO(rot, variant)
			if gotIn != in || gotOut != out {
				t.Fatalf("in %s out %s -> rot %d var %d -> %s %s", in, out, rot, variant, gotIn, gotOut)
			}
		}
	}
	if valid != 12 {
		t.Fatalf("valid belt poses = %d", valid)
	}
}

func TestRouteStraight(t *testing.T) {
	belts, ok := route(occMap{}, pos{5, 5}, "south", pos{5, 2}, "north")
	if !ok || len(belts) != 4 {
		t.Fatalf("straight ok=%v len=%d", ok, len(belts))
	}
	for _, belt := range belts {
		in, out := BeltIO(belt.Rotation, belt.RotationVariant)
		if in != "south" || out != "north" {
			t.Fatalf("belt at %d,%d goes %s -> %s", belt.X, belt.Y, in, out)
		}
	}
}

func TestStoryShapes(t *testing.T) {
	type level struct {
		shape  string
		reward string
	}
	levels := []level{
		{"CuCuCuCu", "reward_cutter_and_trash"},
		{"----CuCu", "no_reward"},
		{"RuRuRuRu", "reward_balancer"},
		{"RuRu----", "reward_rotater"},
		{"Cu----Cu", "reward_tunnel"},
		{"Cu------", "reward_painter"},
		{"CrCrCrCr", "reward_rotater_ccw"},
		{"RbRb----", "reward_mixer"},
		{"CpCpCpCp", "reward_merger"},
		{"ScScScSc", "reward_stacker"},
		{"CgScScCg", "reward_miner_chainable"},
		{"CbCbCbRb:CwCwCwCw", "reward_blueprints"},
		{"RpRpRpRp:CwCwCwCw", "reward_underground_belt_tier_2"},
		{"--Cg----:--Cr----", "reward_belt_reader"},
		{"SrSrSrSr:CyCyCyCy", "reward_storage"},
		{"SrSrSrSr:CyCyCyCy:SwSwSwSw", "reward_cutter_quad"},
		{"CbRbRbCb:CwCwCwCw:WbWbWbWb", "reward_painter_double"},
		{"Sg----Sg:CgCgCgCg:--CyCy--", "reward_rotater_180"},
		{"CpRpCp--:SwSwSwSw", "reward_splitter"},
		{"RuCw--Cw:----Ru--", "reward_wires_painter_and_levers"},
		{"CrCwCrCw:CwCrCwCr:CrCwCrCw:CwCrCwCr", "reward_filter"},
		{"Cg----Cr:Cw----Cw:Sy------:Cy----Cy", "reward_constant_signal"},
		{"CcSyCcSy:SyCcSyCc:CcSyCcSy", "reward_display"},
		{"CcRcCcRc:RwCwRwCw:Sr--Sw--:CyCyCyCy", "reward_logic_gates"},
		{"Rg--Rg--:CwRwCwRw:--Rg--Rg", "reward_virtual_processing"},
		{"CbCuCbCu:Sr------:--CrSrCr:CwCwCwCw", "reward_freeplay"},
	}
	var rewards []string
	for i, level := range levels {
		op, err := Solve(level.shape, UnlockFromRewards(rewards))
		if err != nil {
			t.Fatalf("level %d %s: %v", i+1, level.shape, err)
		}
		got, err := Eval(op)
		if err != nil {
			t.Fatalf("level %d eval: %v", i+1, err)
		}
		if got.Form.Hash() != level.shape {
			t.Fatalf("level %d got %s", i+1, got.Form.Hash())
		}
		rewards = append(rewards, level.reward)
	}
}

func TestLayoutReachesHub(t *testing.T) {
	shapes := []string{"CuCuCuCu", "----CuCu", "Cu------", "CrCrCrCr"}
	rewards := [][]string{
		nil,
		{"reward_cutter_and_trash"},
		{"reward_cutter_and_trash", "reward_rotater"},
		{"reward_cutter_and_trash", "reward_rotater", "reward_painter"},
	}
	for i, shape := range shapes {
		plan, err := Compile(Request{
			Level:     i + 1,
			Shape:     shape,
			Rewards:   rewards[i],
			Resources: testResources(),
		}, 8, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Error != "" {
			t.Fatalf("%s: %s", shape, plan.Error)
		}
		if !reachesHub(plan.Placements) {
			t.Fatalf("%s did not reach the hub (%d placements)", shape, len(plan.Placements))
		}
	}
}

func testResources() []Resource {
	var out []Resource
	shapes := []string{"CuCuCuCu", "RuRuRuRu", "SuSuSuSu", "WuWuWuWu"}
	colors := []string{"red", "green", "blue"}
	for s, key := range shapes {
		for i := 0; i < 30; i++ {
			out = append(out, Resource{X: 48 + s*4, Y: 40 + i*2, Kind: "shape", Key: key})
		}
	}
	for c, key := range colors {
		for i := 0; i < 30; i++ {
			out = append(out, Resource{X: 70 + c*3, Y: 40 + i*2, Kind: "color", Key: key})
		}
	}
	return out
}

func TestSouthFeedIsStraight(t *testing.T) {
	built, err := Compile(Request{
		Level:     6,
		Shape:     "Cu------",
		Rewards:   []string{"reward_cutter_and_trash", "reward_rotater"},
		Resources: testResources(),
	}, 8, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if built.Error != "" {
		t.Fatal(built.Error)
	}
	var feed *Placement
	for i := range built.Placements {
		p := &built.Placements[i]
		if p.ID == "belt" && p.X == 0 && p.Y == 2 {
			feed = p
		}
	}
	if feed == nil {
		t.Fatal("中枢南侧没有传送带")
	}
	in, out := BeltIO(feed.Rotation, feed.RotationVariant)
	if in != "south" || out != "north" {
		t.Fatalf("入口方向是 %s -> %s", in, out)
	}
}

func reachesHub(places []Placement) bool {
	type building struct {
		id  string
		x   int
		y   int
		rot int
	}
	belts := map[pos]Placement{}
	var buildings []building
	for _, p := range places {
		if p.ID == "belt" {
			belts[pos{p.X, p.Y}] = p
			continue
		}
		buildings = append(buildings, building{p.ID, p.X, p.Y, p.Rotation})
	}
	at := func(x, y int) (building, bool) {
		for _, b := range buildings {
			for _, tile := range footprint(b.id, b.x, b.y) {
				if tile.X == x && tile.Y == y {
					return b, true
				}
			}
		}
		return building{}, false
	}
	var walk func(x, y int, came string, seen map[pos]bool) bool
	walk = func(x, y int, came string, seen map[pos]bool) bool {
		if x >= -2 && x <= 1 && y >= -2 && y <= 1 {
			return true
		}
		here := pos{x, y}
		if seen[here] {
			return false
		}
		seen[here] = true
		if belt, ok := belts[here]; ok {
			in, out := BeltIO(belt.Rotation, belt.RotationVariant)
			if in != came {
				return false
			}
			nx, ny := step(x, y, out)
			next := map[pos]bool{}
			for k, v := range seen {
				next[k] = v
			}
			return walk(nx, ny, opposite(out), next)
		}
		b, ok := at(x, y)
		if !ok {
			return false
		}
		switch b.id {
		case "trash":
			return false
		case "rotater":
			if x != b.x || y != b.y || came != "south" {
				return false
			}
			return walk(b.x, b.y-1, "south", seen)
		case "cutter":
			if x != b.x || y != b.y || came != "south" {
				return false
			}
			left := map[pos]bool{}
			right := map[pos]bool{}
			for k, v := range seen {
				left[k] = v
				right[k] = v
			}
			return walk(b.x, b.y-1, "south", left) || walk(b.x+1, b.y-1, "south", right)
		case "painter":
			shapeIn := x == b.x && y == b.y && came == "south"
			colorIn := x == b.x && y == b.y-1 && came == "west"
			if !shapeIn && !colorIn {
				return false
			}
			return walk(b.x, b.y-2, "south", seen)
		case "stacker", "mixer":
			lower := x == b.x && y == b.y && came == "south"
			upper := x == b.x+1 && y == b.y && came == "south"
			if !lower && !upper {
				return false
			}
			return walk(b.x, b.y-1, "south", seen)
		default:
			return false
		}
	}
	for _, b := range buildings {
		if b.id != "miner" {
			continue
		}
		dx, dy, travel := minerDelta(b.rot)
		if walk(b.x+dx, b.y+dy, opposite(travel), map[pos]bool{}) {
			return true
		}
	}
	return false
}

func footprint(id string, x, y int) []pos {
	switch id {
	case "cutter", "stacker", "mixer":
		return []pos{{x, y}, {x + 1, y}}
	case "painter":
		return []pos{{x, y}, {x, y - 1}}
	default:
		return []pos{{x, y}}
	}
}
