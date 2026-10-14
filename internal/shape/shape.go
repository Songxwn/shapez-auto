// Package shape 实现 shapez.io 的形状短码，以及切割、旋转、染色、堆叠。
// 象限顺序与游戏一致：右上、右下、左下、左上。层从底到顶。
package shape

import (
	"fmt"
	"strings"
)

// Cell 是一个象限。S 为 C/R/S/W，C 为颜色短码；两者都为 0 表示空。
type Cell struct {
	S byte
	C byte
}

func (c Cell) Empty() bool { return c.S == 0 }

func (c Cell) eq(o Cell) bool { return c.S == o.S && c.C == o.C }

// Layer 是一层的四个象限。
type Layer [4]Cell

// Form 是从底到顶的层。
type Form []Layer

var colorName = map[byte]string{
	'r': "red",
	'g': "green",
	'b': "blue",
	'y': "yellow",
	'p': "purple",
	'c': "cyan",
	'w': "white",
	'u': "uncolored",
}

var nameColor = map[string]byte{}

func init() {
	for k, v := range colorName {
		nameColor[v] = k
	}
}

// ColorName 把颜色短码转成游戏里的颜色名。
func ColorName(c byte) string { return colorName[c] }

// ColorCode 把颜色名转成短码。
func ColorCode(name string) byte { return nameColor[name] }

// Parse 解析形状短码。
func Parse(key string) (Form, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("空形状")
	}
	parts := strings.Split(key, ":")
	if len(parts) > 4 {
		return nil, fmt.Errorf("层数超过 4: %s", key)
	}
	form := make(Form, 0, len(parts))
	for _, part := range parts {
		if len(part) != 8 {
			return nil, fmt.Errorf("层长度不是 8: %s", key)
		}
		var layer Layer
		any := false
		for q := 0; q < 4; q++ {
			sh := part[q*2]
			co := part[q*2+1]
			if sh == '-' && co == '-' {
				continue
			}
			if sh != 'C' && sh != 'R' && sh != 'S' && sh != 'W' {
				return nil, fmt.Errorf("非法形状 %q", sh)
			}
			if _, ok := colorName[co]; !ok {
				return nil, fmt.Errorf("非法颜色 %q", co)
			}
			layer[q] = Cell{S: sh, C: co}
			any = true
		}
		if !any {
			return nil, fmt.Errorf("空层: %s", key)
		}
		form = append(form, layer)
	}
	return form, nil
}

// Hash 生成短码。
func (f Form) Hash() string {
	if len(f) == 0 {
		return ""
	}
	var b strings.Builder
	for i, layer := range f {
		if i > 0 {
			b.WriteByte(':')
		}
		for q := 0; q < 4; q++ {
			cell := layer[q]
			if cell.Empty() {
				b.WriteString("--")
				continue
			}
			b.WriteByte(cell.S)
			b.WriteByte(cell.C)
		}
	}
	return b.String()
}

// Full 生成一整块同形同色的单层形状。color 用短码，u 表示无色。
func Full(shape, color byte) Form {
	var layer Layer
	for i := 0; i < 4; i++ {
		layer[i] = Cell{S: shape, C: color}
	}
	return Form{layer}
}

func clone(f Form) Form {
	out := make(Form, len(f))
	copy(out, f)
	return out
}

// RotateCW 顺时针转 90 度。
func RotateCW(f Form) Form {
	out := clone(f)
	for i, layer := range out {
		out[i] = Layer{layer[3], layer[0], layer[1], layer[2]}
	}
	return out
}

// RotateCCW 逆时针转 90 度。
func RotateCCW(f Form) Form {
	out := clone(f)
	for i, layer := range out {
		out[i] = Layer{layer[1], layer[2], layer[3], layer[0]}
	}
	return out
}

// Rotate180 转 180 度。
func Rotate180(f Form) Form {
	return RotateCW(RotateCW(f))
}

func filter(f Form, keep [4]bool) Form {
	var out Form
	for _, layer := range f {
		var next Layer
		any := false
		for q := 0; q < 4; q++ {
			if keep[q] && !layer[q].Empty() {
				next[q] = layer[q]
				any = true
			}
		}
		if any {
			out = append(out, next)
		}
	}
	return out
}

// Cut 按切割器朝北时的两个出口切开。
// left 是左出口（保留左下、左上），right 是右出口（保留右上、右下）。
func Cut(f Form) (left, right Form) {
	left = filter(f, [4]bool{false, false, true, true})
	right = filter(f, [4]bool{true, true, false, false})
	return left, right
}

// Paint 把所有象限染成同一颜色。color 为短码。
func Paint(f Form, color byte) Form {
	out := clone(f)
	for i, layer := range out {
		for q := 0; q < 4; q++ {
			if !layer[q].Empty() {
				out[i][q].C = color
			}
		}
	}
	return out
}

// Stack 把 upper 叠到 lower 上面，规则与 ShapeDefinition.cloneAndStackWith 相同。
func Stack(lower, upper Form) (Form, error) {
	if len(lower) == 0 || len(upper) == 0 {
		return nil, fmt.Errorf("不能堆叠空形状")
	}
	var highest [4]int
	for q := 0; q < 4; q++ {
		highest[q] = -1
	}
	for layer := len(lower) - 1; layer >= 0; layer-- {
		for q := 0; q < 4; q++ {
			if !lower[layer][q].Empty() && highest[q] < layer {
				highest[q] = layer
			}
		}
	}
	lowest := [4]int{4, 4, 4, 4}
	for layer := 0; layer < len(upper); layer++ {
		for q := 0; q < 4; q++ {
			if !upper[layer][q].Empty() && lowest[q] > layer {
				lowest[q] = layer
			}
		}
	}
	smallest := lowest[0] - highest[0]
	for q := 1; q < 4; q++ {
		gap := lowest[q] - highest[q]
		if gap < smallest {
			smallest = gap
		}
	}
	mergeAt := 1 - smallest
	if mergeAt < 0 {
		mergeAt = 0
	}
	merged := clone(lower)
	for len(merged) < mergeAt+len(upper) {
		merged = append(merged, Layer{})
	}
	for layer := 0; layer < len(upper); layer++ {
		at := mergeAt + layer
		for q := 0; q < 4; q++ {
			top := upper[layer][q]
			if top.Empty() {
				continue
			}
			if !merged[at][q].Empty() {
				return nil, fmt.Errorf("堆叠时象限冲突")
			}
			merged[at][q] = top
		}
	}
	if len(merged) > 4 {
		merged = merged[:4]
	}
	return merged, nil
}

// Mix 按光的加色混合两个颜色名，返回颜色名。
func Mix(a, b string) (string, error) {
	av, aok := nameColor[a]
	bv, bok := nameColor[b]
	if !aok || !bok || av == 'u' || bv == 'u' {
		return "", fmt.Errorf("无法混合 %s 与 %s", a, b)
	}
	bits := colorBits(av) | colorBits(bv)
	return colorName[bitsToCode(bits)], nil
}

func colorBits(c byte) int {
	switch c {
	case 'r':
		return 0b001
	case 'g':
		return 0b010
	case 'y':
		return 0b011
	case 'b':
		return 0b100
	case 'p':
		return 0b101
	case 'c':
		return 0b110
	case 'w':
		return 0b111
	default:
		return 0
	}
}

func bitsToCode(bits int) byte {
	switch bits {
	case 0b001:
		return 'r'
	case 0b010:
		return 'g'
	case 0b011:
		return 'y'
	case 0b100:
		return 'b'
	case 0b101:
		return 'p'
	case 0b110:
		return 'c'
	case 0b111:
		return 'w'
	default:
		return 'u'
	}
}
