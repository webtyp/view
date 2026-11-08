package tests

import (
	"testing"

	"webtyp.com/view"
)

func TestNotify_AutoSizesToWords(t *testing.T) {
	d := view.Auto()
	if !d.IsAuto() {
		t.Fatal("expected IsAuto() to be true")
	}
	if !d.HasTimer() {
		t.Fatal("expected HasTimer() to be true")
	}

	// Short message -> floor 4000ms
	// "ok": 1 word -> 2500 + 1*350 = 2850 -> floored at 4000
	if ms := d.Millis("ok"); ms != 4000 {
		t.Fatalf("expected 4000, got %d", ms)
	}

	// 40 words -> 2500 + 40*350 = 16500 -> capped at 10000
	msg40 := "one two three four five six seven eight nine ten one two three four five six seven eight nine ten one two three four five six seven eight nine ten one two three four five six seven eight nine ten"
	if ms := d.Millis(msg40); ms != 10000 {
		t.Fatalf("expected 10000, got %d", ms)
	}
}

func TestNotify_ForAndPersistent(t *testing.T) {
	dFor := view.For(1500)
	if dFor.IsAuto() {
		t.Fatal("expected For(1500).IsAuto() to be false")
	}
	if !dFor.HasTimer() {
		t.Fatal("expected For(1500).HasTimer() to be true")
	}
	if ms := dFor.Millis("x"); ms != 1500 {
		t.Fatalf("expected 1500, got %d", ms)
	}

	dPers := view.Persistent()
	if dPers.IsAuto() {
		t.Fatal("expected Persistent().IsAuto() to be false")
	}
	if dPers.HasTimer() {
		t.Fatal("expected Persistent().HasTimer() to be false")
	}
	if ms := dPers.Millis("x"); ms != 0 {
		t.Fatalf("expected 0, got %d", ms)
	}
}
