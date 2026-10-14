package shape

import "testing"

func TestCutRotatePaintStack(t *testing.T) {
	full, err := Parse("CuCuCuCu")
	if err != nil {
		t.Fatal(err)
	}
	left, right := Cut(full)
	if left.Hash() != "----CuCu" || right.Hash() != "CuCu----" {
		t.Fatalf("cut got %s | %s", left.Hash(), right.Hash())
	}
	if RotateCW(left).Hash() != "Cu----Cu" {
		t.Fatalf("rot %s", RotateCW(left).Hash())
	}
	painted := Paint(Full('R', 'u'), 'b')
	if painted.Hash() != "RbRbRbRb" {
		t.Fatal(painted.Hash())
	}
	q := singleQuadrant(t, 'C', 'u', 0)
	if q.Hash() != "Cu------" {
		t.Fatal(q.Hash())
	}
	br := singleQuadrant(t, 'C', 'u', 1)
	if br.Hash() != "--Cu----" {
		t.Fatal(br.Hash())
	}
	stacked, err := Stack(q, br)
	if err != nil {
		t.Fatal(err)
	}
	if stacked.Hash() != "CuCu----" {
		t.Fatal(stacked.Hash())
	}
	lower, _ := Parse("--Cg----")
	upper, _ := Parse("--Cr----")
	two, err := Stack(lower, upper)
	if err != nil {
		t.Fatal(err)
	}
	if two.Hash() != "--Cg----:--Cr----" {
		t.Fatal(two.Hash())
	}
	white, err := Mix("yellow", "blue")
	if err != nil || white != "white" {
		t.Fatal(white, err)
	}
	purple, err := Mix("red", "blue")
	if err != nil || purple != "purple" {
		t.Fatal(purple, err)
	}
}

func singleQuadrant(t *testing.T, shape, color byte, index int) Form {
	t.Helper()
	f := Full(shape, color)
	_, right := Cut(f)
	right = RotateCW(right)
	_, right = Cut(right)
	k := (index - 1 + 4) % 4
	for i := 0; i < k; i++ {
		right = RotateCW(right)
	}
	return right
}
