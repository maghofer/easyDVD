package engine

import (
	"fmt"
	"slices"
	"sort"

	"easydvd/internal/model"
)

// UpdateJobs validates the entire selection before changing any job. A patch
// changes only the control the user edited, preserving mixed settings.
func (e *Engine) UpdateJobs(ids []string, patch func(*model.Settings)) error {
	return e.updateJobs(ids, patch, false)
}
func (e *Engine) updateJobs(ids []string, patch func(*model.Settings), explicitPlayback bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clearing {
		return fmt.Errorf("queue is being cleared")
	}
	next := make(map[int]model.Settings)
	for _, id := range ids {
		i := e.index(id)
		if i < 0 {
			return fmt.Errorf("job no longer exists")
		}
		j := e.jobs[i]
		if j.Locked || j.State == model.Scanning || e.cancels[id] != nil || e.scans[id] != nil {
			return fmt.Errorf("selected jobs must be stopped and unrendered; clear render status to edit a finished job")
		}
		s := j.Settings
		patch(&s)
		s.Packing = "Strict order"
		if err := s.Validate(); err != nil {
			return err
		}
		next[i] = s
	}
	for i, s := range next {
		if explicitPlayback || s.NoMenu != e.jobs[i].Settings.NoMenu {
			mode := s.NoMenu
			e.jobs[i].NoMenuOverride = &mode
		}
		e.jobs[i].Settings = s
		e.refreshReadinessLocked(&e.jobs[i])
		e.estimateLocked(&e.jobs[i])
	}
	e.persistLocked()
	return nil
}

func (e *Engine) SetPlayback(ids []string, noMenu bool) error {
	return e.updateJobs(ids, func(s *model.Settings) { s.NoMenu = noMenu }, true)
}

// ClearRenderStatus starts a new artifact namespace. Previous outputs and
// checkpoints remain intact, including when a reset is interrupted by a crash.
func (e *Engine) ClearRenderStatus(ids []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clearing {
		return fmt.Errorf("queue is being cleared")
	}
	for _, id := range ids {
		i := e.index(id)
		if i < 0 {
			return fmt.Errorf("job no longer exists")
		}
		j := e.jobs[i]
		if (j.State != model.Complete && j.State != model.Blocked) || e.cancels[id] != nil || e.scans[id] != nil {
			return fmt.Errorf("clear render status is available for completed or failed jobs after processing stops")
		}
	}
	for _, id := range ids {
		j := &e.jobs[e.index(id)]
		j.RenderRevision++
		j.Locked = false
		j.Progress = 0
		j.ElapsedSeconds = 0
		j.Finished = nil
		j.OutputPath = ""
		j.Generation = ""
		j.Discs = nil
		j.Error = ""
		needsScan := !j.ScanComplete
		for k := range j.Videos {
			v := &j.Videos[k]
			v.Encoded = ""
			v.EncodedSize = 0
			v.EncodedHash = ""
			v.EncodingKey = ""
			v.Thumbnail = ""
			v.Bluray = model.BlurayPlan{}
			if v.Duration > 0 {
				v.Error = ""
			} else if !v.Excluded {
				needsScan = true
			}
		}
		j.State, j.Stage = model.Paused, "Render status cleared — start to regenerate"
		if needsScan {
			j.ScanComplete = false
		} else {
			j.State = model.Ready
			e.refreshReadinessLocked(j)
		}
		delete(e.pauses, id)
	}
	e.persistLocked()
	return nil
}

func applyVideoOrder(j *model.Job) {
	rank := map[string]int{}
	for i, id := range j.VideoOrder {
		rank[id] = i
	}
	sort.SliceStable(j.Videos, func(i, k int) bool {
		a, oka := rank[j.Videos[i].ID]
		b, okb := rank[j.Videos[k].ID]
		return oka && (!okb || a < b)
	})
}

// ReorderJobs moves a selected block to an insertion position in the original list.
func (e *Engine) ReorderJobs(ids []string, before int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clearing {
		return fmt.Errorf("queue is being cleared")
	}
	selected := map[string]bool{}
	for _, id := range ids {
		if e.index(id) < 0 {
			return fmt.Errorf("job no longer exists")
		}
		selected[id] = true
	}
	before = max(0, min(before, len(e.jobs)))
	var moved, rest []model.Job
	insert := 0
	for i, j := range e.jobs {
		if selected[j.ID] {
			moved = append(moved, j)
		} else {
			rest = append(rest, j)
			if i < before {
				insert++
			}
		}
	}
	e.jobs = slices.Insert(rest, insert, moved...)
	e.persistLocked()
	return nil
}

func (e *Engine) ReorderVideos(id string, ids []string, before int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	i := e.index(id)
	if i < 0 {
		return fmt.Errorf("job no longer exists")
	}
	j := &e.jobs[i]
	if e.clearing || j.State == model.Running || j.State == model.Scanning || j.State == model.Complete || e.cancels[id] != nil || e.scans[id] != nil {
		return fmt.Errorf("stop the job before reordering files")
	}
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	for _, v := range j.Videos {
		delete(selected, v.ID)
	}
	if len(selected) > 0 {
		return fmt.Errorf("file no longer exists")
	}
	for _, id := range ids {
		selected[id] = true
	}
	before = max(0, min(before, len(j.Videos)))
	var moved, rest []model.Video
	insert := 0
	for k, v := range j.Videos {
		if selected[v.ID] {
			moved = append(moved, v)
		} else {
			rest = append(rest, v)
			if k < before {
				insert++
			}
		}
	}
	j.Videos = slices.Insert(rest, insert, moved...)
	j.VideoOrder = nil
	for _, v := range j.Videos {
		j.VideoOrder = append(j.VideoOrder, v.ID)
	}
	j.Discs = nil
	j.Generation = ""
	e.estimateLocked(j)
	e.persistLocked()
	return nil
}
