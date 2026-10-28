package store

import (
	"easydvd/internal/model"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTripAndReorder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(path); err == nil {
		other.Close()
		t.Fatal("concurrent queue open accepted")
	}
	settings := model.DefaultSettings(t.TempDir())
	jobs := []model.Job{{ID: "one", Title: "Film & family", State: model.Paused, Settings: settings, Videos: []model.Video{{ID: "v", Path: `\\server\share\ü & films\5694-001.mp4`, EncodedHash: "abc"}}}, {ID: "two", State: model.Ready}}
	if err = s.Save(jobs); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || !reflect.DeepEqual(got, jobs) {
		t.Fatalf("%+v %v", got, err)
	}
	v, err := s.Settings(model.Settings{})
	if err != nil || !reflect.DeepEqual(v, settings) {
		t.Fatal(v, err)
	}
	jobs[0], jobs[1] = jobs[1], jobs[0]
	if err = s.Save(jobs); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil || got[0].ID != "two" {
		t.Fatal(got, err)
	}
}
