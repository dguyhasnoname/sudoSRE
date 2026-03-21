package pdf

import (
	"reflect"
	"testing"
)

func TestParsePageSpec(t *testing.T) {
	segs, err := ParsePageSpec("1-3, 5 ,12-15")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1-3", "5", "12-15"}
	if !reflect.DeepEqual(segs, want) {
		t.Fatalf("got %v want %v", segs, want)
	}
}

func TestExpandSpecCombine(t *testing.T) {
	segs := []string{"1-2", "5", "7-8"}
	got, err := expandSpecCombine(segs, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 5, 7, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSegmentOverlapWarnings(t *testing.T) {
	w, err := segmentOverlapWarnings([]string{"1-3", "3-5"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(w) == 0 {
		t.Fatal("expected overlap warning")
	}
}
