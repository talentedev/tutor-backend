package utils

import (
	"testing"
	"time"
)

func TestDateIsBefore(t *testing.T) {
	now := time.Now()
	type args struct {
		to   time.Time
		from time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now.Add(-1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now.Add(1 * time.Hour), now.Add(1 * time.Hour)}, false},
		{"ok", args{now.Add(1 * time.Hour), now.Add(2 * time.Hour)}, true},
		{"ok", args{now.Add(3 * time.Hour), now.Add(2 * time.Hour)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsBefore(tt.args.to, tt.args.from); got != tt.want {
				t.Errorf("DateIsBefore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsBeforeOrEqual(t *testing.T) {
	now := time.Now()
	type args struct {
		to   time.Time
		from time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now.Add(-1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now.Add(1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now.Add(1 * time.Hour), now.Add(2 * time.Hour)}, true},
		{"ok", args{now.Add(3 * time.Hour), now.Add(2 * time.Hour)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsBeforeOrEqual(tt.args.to, tt.args.from); got != tt.want {
				t.Errorf("DateIsBeforeOrEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsAfter(t *testing.T) {
	now := time.Now()
	type args struct {
		to   time.Time
		from time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now.Add(-1 * time.Hour), now.Add(1 * time.Hour)}, false},
		{"ok", args{now.Add(1 * time.Hour), now.Add(1 * time.Hour)}, false},
		{"ok", args{now.Add(1 * time.Hour), now.Add(2 * time.Hour)}, false},
		{"ok", args{now.Add(3 * time.Hour), now.Add(2 * time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsAfter(tt.args.to, tt.args.from); got != tt.want {
				t.Errorf("DateIsAfter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsAfterOrEqual(t *testing.T) {
	now := time.Now()
	type args struct {
		to   time.Time
		from time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now.Add(-1 * time.Hour), now.Add(1 * time.Hour)}, false},
		{"ok", args{now.Add(1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now.Add(1 * time.Hour), now.Add(2 * time.Hour)}, false},
		{"ok", args{now.Add(3 * time.Hour), now.Add(2 * time.Hour)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsAfterOrEqual(tt.args.to, tt.args.from); got != tt.want {
				t.Errorf("DateIsAfterOrEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsBetween(t *testing.T) {
	now := time.Now()
	type args struct {
		t      time.Time
		oldRef time.Time
		newRef time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now, now, now}, false},
		{"ok", args{now, now.Add(-1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now, now.Add(1 * time.Hour), now.Add(1 * time.Hour)}, false},
		{"ok", args{now, now.Add(1 * time.Hour), now.Add(2 * time.Hour)}, false},
		{"ok", args{now, now.Add(3 * time.Hour), now.Add(2 * time.Hour)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsBetween(tt.args.t, tt.args.oldRef, tt.args.newRef); got != tt.want {
				t.Errorf("DateIsBetween() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsBetweenOrEqual(t *testing.T) {
	now := time.Now()
	type args struct {
		t      time.Time
		oldRef time.Time
		newRef time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now, now, now}, true},
		{"ok", args{now, now.Add(-1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now.Add(1 * time.Hour), now.Add(1 * time.Hour), now.Add(1 * time.Hour)}, true},
		{"ok", args{now, now.Add(1 * time.Hour), now.Add(2 * time.Hour)}, false},
		{"ok", args{now, now.Add(3 * time.Hour), now.Add(2 * time.Hour)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsBetweenOrEqual(tt.args.t, tt.args.oldRef, tt.args.newRef); got != tt.want {
				t.Errorf("DateIsBetweenOrEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsSame(t *testing.T) {
	date := time.Time{}
	type args struct {
		a time.Time
		b time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"same day", args{date, date.Add(1 * time.Hour)}, true},
		{"different day", args{date, date.Add(24 * time.Hour)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsSame(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("DateIsSame() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDateIsEqual(t *testing.T) {
	now := time.Now()
	type args struct {
		a time.Time
		b time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"ok", args{now, now.Add(1 * time.Second)}, true},
		{"ok", args{now.Add(1 * time.Second), now}, true},
		{"ok", args{now, now.Add(1 * time.Minute)}, false},
		{"ok", args{now.Add(-24 * time.Hour), now}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DateIsEqual(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("DateIsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
