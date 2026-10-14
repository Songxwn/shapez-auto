package plan

// 方向以屏幕坐标为准：北是 y 减小。旋转顺时针，与 shapez.io 一致。

var clockwise = []string{"north", "east", "south", "west"}

func opposite(dir string) string {
	switch dir {
	case "north":
		return "south"
	case "south":
		return "north"
	case "east":
		return "west"
	case "west":
		return "east"
	default:
		return ""
	}
}

func step(x, y int, dir string) (int, int) {
	switch dir {
	case "north":
		return x, y - 1
	case "south":
		return x, y + 1
	case "east":
		return x + 1, y
	case "west":
		return x - 1, y
	default:
		return x, y
	}
}

func turn(dir string, angle int) string {
	i := 0
	for i < 4 && clockwise[i] != dir {
		i++
	}
	if i == 4 {
		return ""
	}
	return clockwise[(i+angle/90)%4]
}

// BeltPose 根据进入方向和离开方向计算传送带旋转与转向。
// inDir 是物品从哪个方向进来，outDir 是往哪个方向出去。
func BeltPose(inDir, outDir string) (rotation, variant int, ok bool) {
	if inDir == "" || outDir == "" || inDir == outDir {
		return 0, 0, false
	}
	rot, known := map[string]int{"south": 0, "west": 90, "north": 180, "east": 270}[inDir]
	if !known {
		return 0, 0, false
	}
	locals := []string{"north", "west", "east"}
	for variant, local := range locals {
		if turn(local, rot) == outDir {
			return rot, variant, true
		}
	}
	return 0, 0, false
}

// BeltIO 由传送带旋转和转向得到进入、离开方向。
func BeltIO(rotation, variant int) (inDir, outDir string) {
	if variant < 0 || variant > 2 {
		return "", ""
	}
	localOut := []string{"north", "west", "east"}[variant]
	return turn("south", rotation), turn(localOut, rotation)
}

type pos struct{ X, Y int }

func minerDelta(rotation int) (dx, dy int, travel string) {
	switch rotation {
	case 0:
		return 0, -1, "north"
	case 90:
		return 1, 0, "east"
	case 180:
		return 0, 1, "south"
	case 270:
		return -1, 0, "west"
	default:
		return 0, 0, ""
	}
}
