package plan

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

// Request 是模组上报的一局游戏。
type Request struct {
	Level          int        `json:"level"`
	Shape          string     `json:"shape"`
	Required       float64    `json:"required"`
	Delivered      float64    `json:"delivered"`
	ThroughputOnly bool       `json:"throughputOnly"`
	Rewards        []string   `json:"rewards"`
	Resources      []Resource `json:"resources"`
	DemoEnded      bool       `json:"demoEnded"`
	Freeplay       bool       `json:"freeplay"`
	Missed         int        `json:"missed"`
}

// Plan 是交给模组执行的方案。
type Plan struct {
	Action      string      `json:"action"`
	ID          string      `json:"id"`
	Level       int         `json:"level"`
	Shape       string      `json:"shape"`
	Source      string      `json:"source"`
	Comment     string      `json:"comment"`
	Warp        int         `json:"warp"`
	Placements  []Placement `json:"placements"`
	Error       string      `json:"error,omitempty"`
}

// Program 是外部模型返回的加工节点。验证通过才会替换内置求解。
type Program struct {
	Comment string `json:"comment"`
	Copies  int    `json:"copies,omitempty"`
	Nodes   []Node `json:"nodes"`
}

// Node 是模型可输出的加工步骤。
type Node struct {
	ID    string `json:"id"`
	Op    string `json:"op"`
	In    string `json:"in,omitempty"`
	Lower string `json:"lower,omitempty"`
	Upper string `json:"upper,omitempty"`
	Shape string `json:"shape,omitempty"`
	Color string `json:"color,omitempty"`
	N     int    `json:"n,omitempty"`
}

// Compile 求解并布局。override 非空且演算结果正确时优先使用。
// speedup 大于 0 时提高加速倍数，并尽量并行多条产线。
func Compile(req Request, warp int, override *Program, speedup int) (Plan, error) {
	if warp < 1 {
		warp = 8
	}
	if speedup < 0 {
		speedup = 0
	}
	if speedup > 3 {
		speedup = 3
	}
	if speedup > 0 {
		warp *= 1 + speedup
		if warp > 60 {
			warp = 60
		}
	}
	if req.ThroughputOnly && warp < 16 {
		warp = 16
	}
	copies := 1 + speedup
	if override != nil && override.Copies > copies {
		copies = override.Copies
	}
	if copies > 4 {
		copies = 4
	}
	plan := Plan{
		Action: "build",
		Level:  req.Level,
		Shape:  req.Shape,
		Warp:   warp,
		Source: "solver",
	}
	u := UnlockFromRewards(req.Rewards)
	op, err := Solve(req.Shape, u)
	if err != nil {
		plan.Action = "error"
		plan.Error = err.Error()
		plan.Comment = "内置求解失败"
		plan.ID = planKey(req, plan.Comment)
		return plan, nil
	}
	plan.Comment = "按关卡目标自动排出切割、旋转、染色和堆叠"
	chosen := op
	if override != nil && len(override.Nodes) > 0 {
		alt, altErr := ParseProgram(override.Nodes)
		if altErr == nil {
			got, evalErr := Eval(alt)
			if evalErr == nil && got.Form.Hash() == req.Shape {
				if places, layoutErr := LayoutCopies(alt, req.Resources, copies); layoutErr == nil {
					plan.Placements = places
					plan.Source = "llm"
					if override.Comment != "" {
						plan.Comment = override.Comment
					} else {
						plan.Comment = "已采用外部模型给出的加工步骤"
					}
					plan.ID = planKey(req, fmt.Sprintf("%s-%d-%d", plan.Source, len(places), speedup))
					return plan, nil
				}
			}
		}
	}
	places, err := LayoutCopies(chosen, req.Resources, copies)
	if err != nil {
		plan.Action = "error"
		plan.Error = err.Error()
		plan.Comment = "加工步骤已经算出，但附近资源不够或摆不下"
		plan.ID = planKey(req, plan.Error)
		return plan, nil
	}
	plan.Placements = places
	if speedup > 0 {
		plan.Comment = fmt.Sprintf("推进偏慢，已并行 %d 条产线并提高加速", copies)
	}
	plan.ID = planKey(req, fmt.Sprintf("%s-%d-%d", plan.Source, len(places), speedup))
	return plan, nil
}

// CompileExternal 只摆外部模型给出的加工步骤。演算或布局失败时不改用内置求解。
func CompileExternal(req Request, warp int, program *Program, speedup int) Plan {
	if warp < 1 {
		warp = 8
	}
	if speedup < 0 {
		speedup = 0
	}
	if speedup > 3 {
		speedup = 3
	}
	if speedup > 0 {
		warp *= 1 + speedup
		if warp > 60 {
			warp = 60
		}
	}
	if req.ThroughputOnly && warp < 16 {
		warp = 16
	}
	base := Plan{
		Action: "error",
		Level:  req.Level,
		Shape:  req.Shape,
		Warp:   warp,
		Source: "llm",
	}
	if program == nil || len(program.Nodes) == 0 {
		base.Error = "外部模型没有给出加工步骤"
		base.ID = planKey(req, base.Error)
		return base
	}
	copies := 1 + speedup
	if program.Copies > copies {
		copies = program.Copies
	}
	if copies > 4 {
		copies = 4
	}
	alt, err := ParseProgram(program.Nodes)
	if err != nil {
		base.Error = err.Error()
		base.ID = planKey(req, base.Error)
		return base
	}
	got, err := Eval(alt)
	if err != nil {
		base.Error = err.Error()
		base.ID = planKey(req, base.Error)
		return base
	}
	if got.Form.Hash() != req.Shape {
		base.Error = fmt.Sprintf("外部模型演算结果 %s 与目标 %s 不一致", got.Form.Hash(), req.Shape)
		base.ID = planKey(req, base.Error)
		return base
	}
	places, err := LayoutCopies(alt, req.Resources, copies)
	if err != nil {
		base.Error = err.Error()
		base.ID = planKey(req, base.Error)
		return base
	}
	base.Action = "build"
	base.Error = ""
	base.Placements = places
	if program.Comment != "" {
		base.Comment = program.Comment
	} else if speedup > 0 {
		base.Comment = fmt.Sprintf("外部模型重新设计，并行 %d 条产线", copies)
	} else {
		base.Comment = "已采用外部模型给出的加工步骤"
	}
	base.ID = planKey(req, fmt.Sprintf("llm-%d-%d-%s", len(places), speedup, base.Comment))
	return base
}

func planKey(req Request, extra string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%d|%s|%s|%v", req.Level, req.Shape, extra, req.Rewards)))
	return hex.EncodeToString(sum[:8])
}

// ParseProgram 把模型节点编译成加工树。最后一个 deliver，或唯一没有被引用的节点作为输出。
func ParseProgram(nodes []Node) (*Op, error) {
	ops := map[string]*Op{}
	var order []string
	var deliver string
	for _, node := range nodes {
		if node.ID == "" || node.Op == "" {
			return nil, fmt.Errorf("节点缺少 id 或 op")
		}
		if _, ok := ops[node.ID]; ok {
			return nil, fmt.Errorf("重复节点 %s", node.ID)
		}
		op := &Op{N: node.N, Color: node.Color}
		if node.Shape != "" {
			op.Shape = node.Shape[0]
		}
		switch node.Op {
		case "mine-shape":
			op.Kind = "mineShape"
		case "mine-color":
			op.Kind = "mineColor"
		case "cut-left":
			op.Kind = "cutLeft"
		case "cut-right":
			op.Kind = "cutRight"
		case "rot-cw":
			op.Kind = "rotCW"
		case "rot-ccw":
			op.Kind = "rotCCW"
		case "rot-180":
			op.Kind = "rot180"
		case "paint":
			op.Kind = "paint"
		case "stack":
			op.Kind = "stack"
		case "mix":
			op.Kind = "mix"
		case "deliver":
			deliver = node.In
			continue
		default:
			return nil, fmt.Errorf("不认识的步骤 %s", node.Op)
		}
		ops[node.ID] = op
		order = append(order, node.ID)
	}
	byID := map[string]Node{}
	for _, node := range nodes {
		byID[node.ID] = node
	}
	var link func(id string, stack []string) (*Op, error)
	link = func(id string, stack []string) (*Op, error) {
		for _, seen := range stack {
			if seen == id {
				return nil, fmt.Errorf("加工步骤循环")
			}
		}
		op := ops[id]
		if op == nil {
			return nil, fmt.Errorf("缺少节点 %s", id)
		}
		node := byID[id]
		next := append(stack, id)
		switch op.Kind {
		case "cutLeft", "cutRight", "rotCW", "rotCCW", "rot180":
			child, err := link(node.In, next)
			if err != nil {
				return nil, err
			}
			op.A = child
		case "paint":
			child, err := link(node.In, next)
			if err != nil {
				return nil, err
			}
			op.A = child
			if node.Lower != "" {
				color, err := link(node.Lower, next)
				if err != nil {
					return nil, err
				}
				op.B = color
			}
		case "stack", "mix":
			a, err := link(firstNonEmpty(node.Lower, node.In), next)
			if err != nil {
				return nil, err
			}
			b, err := link(node.Upper, next)
			if err != nil {
				return nil, err
			}
			op.A = a
			op.B = b
		}
		return op, nil
	}
	rootID := deliver
	if rootID == "" && len(order) > 0 {
		rootID = order[len(order)-1]
	}
	if rootID == "" {
		return nil, fmt.Errorf("没有加工步骤")
	}
	if ops[rootID] == nil {
		return nil, fmt.Errorf("deliver 指向了不存在的节点")
	}
	return link(rootID, nil)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
