package gomukit

import (
	"slices"
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func TestPageSizeOptions(t *testing.T) {
	cases := []struct {
		name     string
		pageSize int
		sizes    []int
		want     []int
	}{
		{"none configured", 10, nil, nil},
		{"unpaginated", 0, []int{10, 25}, nil},
		{"current size added and sorted", 10, []int{50, 25}, []int{10, 25, 50}},
		{"duplicates collapsed", 25, []int{25, 10, 25}, []int{10, 25}},
	}
	for _, c := range cases {
		got := pageSizeOptions(c.pageSize, c.sizes)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: pageSizeOptions(%d, %v) = %v, want %v", c.name, c.pageSize, c.sizes, got, c.want)
		}
	}
}

func TestPaginationNodePageSizeChooser(t *testing.T) {
	plain := renderNode(t, paginationNode(nil, 0))
	if strings.Contains(plain, "data-gomu-page-size") {
		t.Error("pagination without page sizes must not render a chooser")
	}

	withSizes := renderNode(t, paginationNode([]int{10, 25}, 25))
	for _, want := range []string{
		`data-gomu-page-size`,
		`aria-label="Items per page"`,
		`<option value="10">10</option>`,
		`<option value="25" selected>25</option>`,
	} {
		if !strings.Contains(withSizes, want) {
			t.Errorf("pagination bar missing %q in:\n%s", want, withSizes)
		}
	}
}

func renderNode(t *testing.T, node g.Node) string {
	t.Helper()
	var b strings.Builder
	if err := node.Render(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
