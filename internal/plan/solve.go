package plan

import (
	"fmt"

	"shapez-auto/internal/shape"
)

// Unlock 是当前关卡已经解锁的建筑。
type Unlock struct {
	Cutter  bool
	RotCW   bool
	RotCCW  bool
	Rot180  bool
	Painter bool
	Mixer   bool
	Stacker bool
	Tunnel  bool
}

// Op 是一棵加工树。求值结果要么是形状，要么是染料。
type Op struct {
	Kind  string
	Shape byte
	Color string
	N     int
	A     *Op
	B     *Op
}

// Value 是加工结果。
type Value struct {
	Form  shape.Form
	Color string
}

// Eval 按游戏规则演算加工树。
func Eval(op *Op) (Value, error) {
	if op == nil {
		return Value{}, fmt.Errorf("空加工")
	}
	switch op.Kind {
	case "mineShape":
		if op.Shape != 'C' && op.Shape != 'R' && op.Shape != 'S' && op.Shape != 'W' {
			return Value{}, fmt.Errorf("不能开采 %q", op.Shape)
		}
		return Value{Form: shape.Full(op.Shape, 'u')}, nil
	case "mineColor":
		if shape.ColorCode(op.Color) == 0 || op.Color == "uncolored" {
			return Value{}, fmt.Errorf("不能开采染料 %s", op.Color)
		}
		return Value{Color: op.Color}, nil
	case "cutLeft", "cutRight":
		in, err := Eval(op.A)
		if err != nil {
			return Value{}, err
		}
		if in.Form == nil {
			return Value{}, fmt.Errorf("切割对象不是形状")
		}
		left, right := shape.Cut(in.Form)
		if op.Kind == "cutLeft" {
			return Value{Form: left}, nil
		}
		return Value{Form: right}, nil
	case "rotCW", "rotCCW", "rot180":
		in, err := Eval(op.A)
		if err != nil {
			return Value{}, err
		}
		n := op.N
		if n <= 0 {
			n = 1
		}
		f := in.Form
		for i := 0; i < n; i++ {
			switch op.Kind {
			case "rotCW":
				f = shape.RotateCW(f)
			case "rotCCW":
				f = shape.RotateCCW(f)
			case "rot180":
				f = shape.Rotate180(f)
			}
		}
		return Value{Form: f}, nil
	case "paint":
		in, err := Eval(op.A)
		if err != nil {
			return Value{}, err
		}
		color := op.Color
		if op.B != nil {
			c, err := Eval(op.B)
			if err != nil {
				return Value{}, err
			}
			color = c.Color
		}
		code := shape.ColorCode(color)
		if code == 0 || code == 'u' {
			return Value{}, fmt.Errorf("无效染料 %s", color)
		}
		return Value{Form: shape.Paint(in.Form, code)}, nil
	case "mix":
		a, err := Eval(op.A)
		if err != nil {
			return Value{}, err
		}
		b, err := Eval(op.B)
		if err != nil {
			return Value{}, err
		}
		mixed, err := shape.Mix(a.Color, b.Color)
		if err != nil {
			return Value{}, err
		}
		return Value{Color: mixed}, nil
	case "stack":
		a, err := Eval(op.A)
		if err != nil {
			return Value{}, err
		}
		b, err := Eval(op.B)
		if err != nil {
			return Value{}, err
		}
		stacked, err := shape.Stack(a.Form, b.Form)
		if err != nil {
			return Value{}, err
		}
		return Value{Form: stacked}, nil
	default:
		return Value{}, fmt.Errorf("未知加工 %s", op.Kind)
	}
}

// Solve 用已解锁建筑拼出目标短码。
func Solve(key string, u Unlock) (*Op, error) {
	form, err := shape.Parse(key)
	if err != nil {
		return nil, err
	}
	op, err := solveForm(form, u)
	if err != nil {
		return nil, err
	}
	got, err := Eval(op)
	if err != nil {
		return nil, err
	}
	if got.Form.Hash() != key {
		return nil, fmt.Errorf("演算结果 %s 与目标 %s 不一致", got.Form.Hash(), key)
	}
	return op, nil
}

func solveForm(f shape.Form, u Unlock) (*Op, error) {
	op, err := solveMemo(f, u, map[string]memoEnt{})
	if err != nil {
		return nil, err
	}
	got, evalErr := Eval(op)
	if evalErr != nil {
		return nil, evalErr
	}
	if got.Form.Hash() != f.Hash() {
		return nil, fmt.Errorf("演算结果 %s 与目标 %s 不一致", got.Form.Hash(), f.Hash())
	}
	return op, nil
}

type memoEnt struct {
	op  *Op
	err error
	ok  bool
}

func solveMemo(f shape.Form, u Unlock, memo map[string]memoEnt) (*Op, error) {
	key := f.Hash()
	if ent, ok := memo[key]; ok {
		return ent.op, ent.err
	}
	op, err := solveOnce(f, u, memo)
	memo[key] = memoEnt{op: op, err: err, ok: true}
	return op, err
}

func solveOnce(f shape.Form, u Unlock, memo map[string]memoEnt) (*Op, error) {
	if len(f) == 0 {
		return nil, fmt.Errorf("空形状")
	}
	if len(f) == 1 {
		return solveLayer(f[0], u)
	}
	if !hasFloat(f) {
		op, err := stackLayers(f, u)
		if err == nil {
			return op, nil
		}
	}
	if left, right := halfSides(f); left != right && hasFloat(f) {
		if op, err := pinAndCut(f, left, u); err == nil {
			if got, evalErr := Eval(op); evalErr == nil && got.Form.Hash() == f.Hash() {
				return op, nil
			}
		}
	}
	if n := countCells(f); n <= 14 {
		if op, err := splitStack(f, u, memo); err == nil {
			return op, nil
		}
	}
	return nil, fmt.Errorf("还不会搭 %s", f.Hash())
}

func stackLayers(f shape.Form, u Unlock) (*Op, error) {
	if hasFloat(f) && len(f) > 1 {
		return nil, fmt.Errorf("有悬空层")
	}
	if !u.Stacker && len(f) > 1 {
		return nil, fmt.Errorf("这一关还没有堆叠器，无法做多层形状")
	}
	var acc *Op
	for i, layer := range f {
		part, err := solveLayer(layer, u)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			acc = part
			continue
		}
		acc = &Op{Kind: "stack", A: acc, B: part}
	}
	got, err := Eval(acc)
	if err != nil {
		return nil, err
	}
	if got.Form.Hash() != f.Hash() {
		return nil, fmt.Errorf("逐层堆叠会塌成 %s", got.Form.Hash())
	}
	return acc, nil
}

func hasFloat(f shape.Form) bool {
	for layer := 1; layer < len(f); layer++ {
		for q := 0; q < 4; q++ {
			if !f[layer][q].Empty() && f[layer-1][q].Empty() {
				return true
			}
		}
	}
	return false
}

func halfSides(f shape.Form) (left, right bool) {
	for _, layer := range f {
		if !layer[2].Empty() || !layer[3].Empty() {
			left = true
		}
		if !layer[0].Empty() || !layer[1].Empty() {
			right = true
		}
	}
	return left, right
}

func countCells(f shape.Form) int {
	n := 0
	for _, layer := range f {
		for q := 0; q < 4; q++ {
			if !layer[q].Empty() {
				n++
			}
		}
	}
	return n
}

// pinAndCut 在空的那一半加上会互相顶住的支撑，堆高后再切掉支撑，从而留下悬空象限。
func pinAndCut(f shape.Form, keepLeft bool, u Unlock) (*Op, error) {
	support := 0
	cut := "cutRight"
	if keepLeft {
		support = 0
		cut = "cutLeft"
	} else {
		support = 2
		cut = "cutRight"
	}
	var acc *Op
	for _, layer := range f {
		augmented := layer
		if !augmented[support].Empty() {
			return nil, fmt.Errorf("支撑象限已被占用")
		}
		augmented[support] = shape.Cell{S: 'R', C: 'u'}
		part, err := solveLayer(augmented, u)
		if err != nil {
			return nil, err
		}
		if acc == nil {
			acc = part
			continue
		}
		if !u.Stacker {
			return nil, fmt.Errorf("这一关还没有堆叠器，无法做多层形状")
		}
		acc = &Op{Kind: "stack", A: acc, B: part}
	}
	return &Op{Kind: cut, A: acc}, nil
}

type placedCell struct {
	layer int
	quad  int
	cell  shape.Cell
}

func splitStack(f shape.Form, u Unlock, memo map[string]memoEnt) (*Op, error) {
	if !u.Stacker {
		return nil, fmt.Errorf("这一关还没有堆叠器，无法做多层形状")
	}
	var cells []placedCell
	for layer := range f {
		for q := 0; q < 4; q++ {
			if !f[layer][q].Empty() {
				cells = append(cells, placedCell{layer, q, f[layer][q]})
			}
		}
	}
	n := len(cells)
	if n < 2 || n > 14 {
		return nil, fmt.Errorf("无法拆开 %s", f.Hash())
	}
	limit := 1 << n
	for mask := 1; mask < limit-1; mask++ {
		upper, okU := maskForm(f, cells, mask, true)
		lower, okL := maskForm(f, cells, mask, false)
		if !okU || !okL {
			continue
		}
		stacked, err := shape.Stack(lower, upper)
		if err != nil || stacked.Hash() != f.Hash() {
			continue
		}
		if countCells(lower) >= n || countCells(upper) >= n {
			continue
		}
		lowOp, lowErr := solveMemo(lower, u, memo)
		if lowErr != nil {
			continue
		}
		upOp, upErr := solveMemo(upper, u, memo)
		if upErr != nil {
			continue
		}
		return &Op{Kind: "stack", A: lowOp, B: upOp}, nil
	}
	return nil, fmt.Errorf("无法拆开 %s", f.Hash())
}

func maskForm(src shape.Form, cells []placedCell, mask int, upper bool) (shape.Form, bool) {
	layers := make(shape.Form, len(src))
	used := false
	minL, maxL := len(src), -1
	for i, cell := range cells {
		take := mask&(1<<i) != 0
		if take != upper {
			continue
		}
		layers[cell.layer][cell.quad] = cell.cell
		used = true
		if cell.layer < minL {
			minL = cell.layer
		}
		if cell.layer > maxL {
			maxL = cell.layer
		}
	}
	if !used {
		return nil, false
	}
	for layer := minL; layer <= maxL; layer++ {
		empty := true
		for q := 0; q < 4; q++ {
			if !layers[layer][q].Empty() {
				empty = false
				break
			}
		}
		if empty {
			return nil, false
		}
	}
	return layers[minL : maxL+1], true
}

func solveLayer(layer shape.Layer, u Unlock) (*Op, error) {
	filled := make([]int, 0, 4)
	for q := 0; q < 4; q++ {
		if !layer[q].Empty() {
			filled = append(filled, q)
		}
	}
	if len(filled) == 0 {
		return nil, fmt.Errorf("空层")
	}
	same := true
	first := layer[filled[0]]
	for _, q := range filled[1:] {
		cell := layer[q]
		if cell.S != first.S || cell.C != first.C {
			same = false
			break
		}
	}
	if same && len(filled) == 4 {
		return paintedMine(layer[0], u)
	}
	if same && len(filled) == 2 {
		if op, ok, err := solvePair(layer, filled[0], filled[1], u); ok || err != nil {
			return op, err
		}
	}
	if len(filled) == 1 {
		return quadrant(layer[filled[0]], filled[0], u)
	}
	if !u.Stacker {
		return nil, fmt.Errorf("这一关还没有堆叠器，无法拼接不同象限")
	}
	var acc *Op
	for _, q := range filled {
		part, err := quadrant(layer[q], q, u)
		if err != nil {
			return nil, err
		}
		if acc == nil {
			acc = part
			continue
		}
		acc = &Op{Kind: "stack", A: acc, B: part}
	}
	return acc, nil
}

func solvePair(layer shape.Layer, a, b int, u Unlock) (*Op, bool, error) {
	cell := layer[a]
	base, err := paintedMine(cell, u)
	if err != nil {
		return nil, true, err
	}
	if !u.Cutter {
		return nil, true, fmt.Errorf("这一关还没有切割器")
	}
	right := &Op{Kind: "cutRight", A: base}
	left := &Op{Kind: "cutLeft", A: base}
	switch {
	case a == 0 && b == 1:
		return right, true, nil
	case a == 2 && b == 3:
		return left, true, nil
	case a == 1 && b == 2:
		op, err := rotate(right, 1, u)
		return op, true, err
	case a == 0 && b == 3:
		op, err := rotate(left, 1, u)
		return op, true, err
	default:
		return nil, false, nil
	}
}

func paintedMine(cell shape.Cell, u Unlock) (*Op, error) {
	op := &Op{Kind: "mineShape", Shape: cell.S}
	if cell.C == 'u' || cell.C == 0 {
		return op, nil
	}
	if !u.Painter {
		return nil, fmt.Errorf("这一关还没有染色器")
	}
	color, err := colorOp(cell.C, u)
	if err != nil {
		return nil, err
	}
	return &Op{Kind: "paint", A: op, B: color}, nil
}

func quadrant(cell shape.Cell, index int, u Unlock) (*Op, error) {
	if !u.Cutter {
		return nil, fmt.Errorf("这一关还没有切割器")
	}
	base, err := paintedMine(cell, u)
	if err != nil {
		return nil, err
	}
	// 整块 -> 右半 -> 顺时针 -> 再切右半，得到右下象限，再转到目标象限。
	op := &Op{Kind: "cutRight", A: base}
	op, err = rotate(op, 1, u)
	if err != nil {
		return nil, err
	}
	op = &Op{Kind: "cutRight", A: op}
	k := (index - 1 + 4) % 4
	return rotate(op, k, u)
}

func rotate(op *Op, n int, u Unlock) (*Op, error) {
	if n == 0 {
		return op, nil
	}
	if !u.RotCW {
		return nil, fmt.Errorf("这一关还没有旋转器")
	}
	return &Op{Kind: "rotCW", N: n, A: op}, nil
}

func colorOp(code byte, u Unlock) (*Op, error) {
	switch code {
	case 'r', 'g', 'b':
		return &Op{Kind: "mineColor", Color: shape.ColorName(code)}, nil
	case 'y', 'p', 'c', 'w':
		if !u.Mixer {
			return nil, fmt.Errorf("这一关还没有混色器")
		}
	default:
		return nil, fmt.Errorf("不支持的颜色 %q", code)
	}
	switch code {
	case 'y':
		return mixCodes('r', 'g', u)
	case 'p':
		return mixCodes('r', 'b', u)
	case 'c':
		return mixCodes('g', 'b', u)
	case 'w':
		yellow, err := colorOp('y', u)
		if err != nil {
			return nil, err
		}
		blue, err := colorOp('b', u)
		if err != nil {
			return nil, err
		}
		return &Op{Kind: "mix", A: yellow, B: blue}, nil
	default:
		return nil, fmt.Errorf("不支持的颜色 %q", code)
	}
}

func mixCodes(a, b byte, u Unlock) (*Op, error) {
	left, err := colorOp(a, u)
	if err != nil {
		return nil, err
	}
	right, err := colorOp(b, u)
	if err != nil {
		return nil, err
	}
	return &Op{Kind: "mix", A: left, B: right}, nil
}

// UnlockFromRewards 根据已获得的关卡奖励生成解锁表。
func UnlockFromRewards(rewards []string) Unlock {
	var u Unlock
	for _, reward := range rewards {
		switch reward {
		case "reward_cutter_and_trash":
			u.Cutter = true
		case "reward_rotater":
			u.RotCW = true
		case "reward_rotater_ccw":
			u.RotCCW = true
		case "reward_rotater_180":
			u.Rot180 = true
		case "reward_painter":
			u.Painter = true
		case "reward_mixer":
			u.Mixer = true
		case "reward_stacker":
			u.Stacker = true
		case "reward_tunnel":
			u.Tunnel = true
		}
	}
	return u
}
