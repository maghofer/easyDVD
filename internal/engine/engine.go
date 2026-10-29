package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"easydvd/internal/fsops"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/packing"
	"easydvd/internal/store"
)

var errPaused = errors.New("paused at stage boundary")

type Snapshot struct {
	Jobs     []model.Job
	Settings model.Settings
	Enabled  bool
	Clearing bool
	Error    string
	Busy     map[string]bool
}
type Engine struct {
	mu         sync.Mutex
	jobs       []model.Job
	settings   model.Settings
	store      *store.Store
	tools      media.Tools
	ctx        context.Context
	cancel     context.CancelFunc
	wake       chan struct{}
	Events     chan struct{}
	enabled    bool
	clearing   bool
	pauses     map[string]bool
	cancels    map[string]context.CancelFunc
	scans      map[string]context.CancelFunc
	scanSlots  chan struct{}
	probeSlots chan struct{}
	wg         sync.WaitGroup
	lastError  string
	dataDir    string
}

type Option func(*Engine)

// WithDataDir separates persistent logs/job records from media work and output.
// Without an override, isolated queues keep these records beside their database.
func WithDataDir(dir string) Option { return func(e *Engine) { e.dataDir = dir } }

func New(st *store.Store, defaults model.Settings, t media.Tools, options ...Option) (*Engine, error) {
	settings, err := st.Settings(defaults)
	if err != nil {
		return nil, err
	}
	jobs, err := st.Load()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{jobs: jobs, settings: settings, store: st, tools: t, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), Events: make(chan struct{}, 1), pauses: map[string]bool{}, cancels: map[string]context.CancelFunc{}, scans: map[string]context.CancelFunc{}, scanSlots: make(chan struct{}, 2), probeSlots: make(chan struct{}, 2)}
	e.dataDir = st.Directory()
	for _, option := range options {
		option(e)
	}
	for i := range e.jobs {
		j := &e.jobs[i]
		if len(j.ID) != 24 || strings.Trim(j.ID, "0123456789abcdef") != "" {
			cancel()
			return nil, fmt.Errorf("invalid saved job identifier")
		}
		if j.State == model.Running || j.State == model.Scanning {
			j.State = model.Paused
			j.Stage = "Interrupted — resume to continue"
		}
	}
	if err = st.Save(e.jobs); err != nil {
		cancel()
		return nil, err
	}
	e.wg.Add(1)
	go e.loop()
	return e, nil
}
func (e *Engine) Close() { e.cancel(); e.wg.Wait() }
func (e *Engine) signal() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
	select {
	case e.Events <- struct{}{}:
	default:
	}
}
func (e *Engine) persistLocked() {
	if err := e.store.Save(e.jobs); err != nil {
		e.lastError = "Cannot save queue: " + err.Error()
		e.enabled = false
	}
	e.signal()
}
func (e *Engine) index(id string) int {
	for i := range e.jobs {
		if e.jobs[i].ID == id {
			return i
		}
	}
	return -1
}
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := Snapshot{Settings: e.settings, Enabled: e.enabled, Clearing: e.clearing, Error: e.lastError, Busy: map[string]bool{}}
	for id := range e.cancels {
		s.Busy[id] = true
	}
	for id := range e.scans {
		s.Busy[id] = true
	}
	for _, j := range e.jobs {
		s.Jobs = append(s.Jobs, j.Clone())
	}
	return s
}
func (e *Engine) Tools() media.Tools { e.mu.Lock(); defer e.mu.Unlock(); return e.tools }
func (e *Engine) SetTools(t media.Tools) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.cancels) > 0 || len(e.scans) > 0 {
		return fmt.Errorf("wait for active scanning and processing to stop before changing media tools")
	}
	e.tools = t
	e.signal()
	return nil
}

func (e *Engine) Add(folder string) error {
	absolute, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	absolute = filepath.Clean(absolute)
	info, err := fsops.Stat(e.ctx, absolute)
	if err != nil {
		return err
	}
	if info.Exists && !info.Directory {
		return e.AddFiles([]string{absolute})
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, j := range e.jobs {
		if len(j.Sources) == 0 && strings.EqualFold(j.Folder, absolute) && j.State != model.Complete && j.State != model.Canceled {
			return fmt.Errorf("folder is already in the queue")
		}
	}
	j := model.Job{ID: model.ID(), Folder: absolute, Title: filepath.Base(absolute), State: model.Scanning, Stage: "Discovering videos", Settings: e.settings, Created: time.Now()}
	e.jobs = append(e.jobs, j)
	e.persistLocked()
	e.scanLocked(j.ID)
	return nil
}

// AddFiles records an explicit selection so rescans never pick up sibling files.
func (e *Engine) AddFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("select at least one video file")
	}
	var sources []string
	seen := map[string]bool{}
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		absolute = filepath.Clean(absolute)
		if !media.IsVideo(absolute) {
			return fmt.Errorf("unsupported video file: %s", path)
		}
		key := strings.ToLower(absolute)
		if !seen[key] {
			sources = append(sources, absolute)
			seen[key] = true
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, j := range e.jobs {
		if j.State == model.Complete || j.State == model.Canceled {
			continue
		}
		for _, path := range j.Sources {
			if seen[strings.ToLower(path)] {
				return fmt.Errorf("file is already in the queue: %s", path)
			}
		}
	}
	title := strings.TrimSuffix(filepath.Base(sources[0]), filepath.Ext(sources[0]))
	if len(sources) > 1 {
		title = filepath.Base(filepath.Dir(sources[0]))
	}
	j := model.Job{ID: model.ID(), Folder: filepath.Dir(sources[0]), Sources: sources, Title: title, State: model.Scanning, Stage: "Inspecting selected videos", Settings: e.settings, Created: time.Now()}
	e.jobs = append(e.jobs, j)
	e.persistLocked()
	e.scanLocked(j.ID)
	return nil
}

func (e *Engine) scanLocked(id string) {
	if _, ok := e.scans[id]; ok {
		return
	}
	// Publish the pending scan before waking the queue loop. Otherwise a
	// resumed scan can look idle and cause Start to disable processing.
	i := e.index(id)
	if i < 0 {
		return
	}
	e.jobs[i].State = model.Scanning
	e.jobs[i].ScanComplete = false
	e.jobs[i].Stage = "Discovering videos"
	ctx, cancel := context.WithCancel(e.ctx)
	e.scans[id] = cancel
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer cancel()
		defer func() { e.mu.Lock(); delete(e.scans, id); e.signal(); e.mu.Unlock() }()
		select {
		case e.scanSlots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		defer func() { <-e.scanSlots }()
		e.mu.Lock()
		idx := e.index(id)
		if idx < 0 {
			e.mu.Unlock()
			return
		}
		job := e.jobs[idx].Clone()
		old := map[string]model.Video{}
		for _, v := range job.Videos {
			old[v.Relative] = v
		}
		e.jobs[idx].Videos = nil
		e.jobs[idx].Discovered = 0
		e.jobs[idx].State = model.Scanning
		e.jobs[idx].Error = ""
		e.mu.Unlock()
		paths := make(chan string, 4)
		var workers sync.WaitGroup
		var probeCount atomic.Int64
		for k := 0; k < 2; k++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for relative := range paths {
					select {
					case e.probeSlots <- struct{}{}:
					case <-ctx.Done():
						return
					}
					path := relative
					if !filepath.IsAbs(path) {
						path = filepath.Join(job.Folder, relative)
					}
					v, err := e.tools.Probe(ctx, path, relative)
					<-e.probeSlots
					if ctx.Err() != nil {
						return
					}
					if err != nil {
						v.Error = err.Error()
					}
					if previous, ok := old[relative]; ok {
						v.Label = previous.Label
						v.Excluded = previous.Excluded
						if previous.Fingerprint() == v.Fingerprint() {
							v.CropLeft, v.CropRight, v.CropChecked = previous.CropLeft, previous.CropRight, previous.CropChecked
							v.CropVersion = previous.CropVersion
							v.Encoded = previous.Encoded
							v.EncodedSize = previous.EncodedSize
							v.EncodedHash = previous.EncodedHash
							v.EncodingKey = previous.EncodingKey
							v.Thumbnail = previous.Thumbnail
						}
					}
					n := probeCount.Add(1)
					e.mu.Lock()
					i := e.index(id)
					if i >= 0 {
						e.jobs[i].Videos = append(e.jobs[i].Videos, v)
						e.jobs[i].Stage = fmt.Sprintf("Found %d · inspected %d", e.jobs[i].Discovered, n)
						if n%20 == 0 {
							e.persistLocked()
						} else {
							e.signal()
						}
					}
					e.mu.Unlock()
				}
			}()
		}
		found := func(path string) {
			e.mu.Lock()
			if i := e.index(id); i >= 0 {
				e.jobs[i].Discovered++
				e.signal()
			}
			e.mu.Unlock()
			select {
			case paths <- path:
			case <-ctx.Done():
			}
		}
		var err error
		if len(job.Sources) > 0 {
			for _, path := range job.Sources {
				relative, relErr := filepath.Rel(job.Folder, path)
				if relErr != nil {
					relative = path
				}
				found(relative)
			}
		} else {
			err = media.Discover(ctx, job.Folder, []string{job.Settings.WorkDir, job.Settings.OutputDir}, found)
		}
		close(paths)
		workers.Wait()
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.scans, id)
		i := e.index(id)
		if i < 0 {
			return
		}
		j := &e.jobs[i]
		packing.SortVideos(j.Videos)
		if ctx.Err() != nil {
			if j.State != model.Canceled {
				j.State = model.Paused
			}
			j.Stage = "Scan interrupted — resume to rescan"
			e.persistLocked()
			return
		}
		j.ScanComplete = err == nil
		j.State = model.Ready
		j.Stage = "Ready"
		j.Progress = 0
		if err != nil {
			j.State = model.Blocked
			j.Error = err.Error()
			j.Stage = "Folder scan failed"
		} else if len(j.ActiveVideos()) == 0 {
			j.State = model.Blocked
			j.Error = "No supported video files found"
		} else {
			for _, v := range j.Videos {
				if !v.Excluded && v.Error != "" {
					j.State = model.Blocked
					j.Error = "One or more videos could not be inspected; retry or exclude them"
					break
				}
			}
		}
		e.estimateLocked(j)
		e.persistLocked()
	}()
}

func (e *Engine) estimateLocked(j *model.Job) {
	var items []packing.Item
	for i, v := range j.Videos {
		if !v.Excluded && v.Error == "" {
			items = append(items, packing.Item{ID: v.ID, Bytes: v.EstimatedBytes(j.Settings), Order: i})
		}
	}
	r, err := packing.Plan(context.Background(), items, j.Settings.Capacity(), j.Settings.Packing == "Space efficiency", 0)
	if err == nil {
		j.EstimatedDiscs = len(r.Discs)
	} else {
		j.EstimatedDiscs = 0
	}
}
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clearing {
		return
	}
	e.enabled = true
	for i := range e.jobs {
		j := &e.jobs[i]
		delete(e.pauses, j.ID)
		if j.State == model.Paused {
			delete(e.pauses, j.ID)
			if j.ScanComplete {
				j.State = model.Ready
			} else {
				e.scanLocked(j.ID)
			}
		}
	}
	e.persistLocked()
}
func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = false
	for _, j := range e.jobs {
		if j.State == model.Running {
			e.pauses[j.ID] = true
		}
	}
	e.persistLocked()
}
func (e *Engine) Cancel(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	i := e.index(id)
	if i < 0 || e.jobs[i].State == model.Complete {
		return
	}
	if c := e.cancels[id]; c != nil {
		c()
	}
	if c := e.scans[id]; c != nil {
		c()
	}
	e.jobs[i].State = model.Canceled
	e.jobs[i].Stage = "Canceled — completed encodes retained"
	e.persistLocked()
}
func (e *Engine) Retry(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.clearing {
		return
	}
	i := e.index(id)
	if i < 0 {
		return
	}
	j := &e.jobs[i]
	if j.State == model.Running || j.State == model.Scanning || j.State == model.Complete {
		return
	}
	delete(e.pauses, id)
	j.Error = ""
	e.enabled = true
	if !j.ScanComplete {
		e.scanLocked(id)
	} else {
		for _, v := range j.Videos {
			if !v.Excluded && v.Duration <= 0 {
				e.scanLocked(id)
				return
			}
		}
		j.State = model.Ready
		j.Stage = "Ready to resume"
	}
	e.persistLocked()
}
func (e *Engine) Move(id string, delta int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	i := e.index(id)
	n := i + delta
	if i < 0 || n < 0 || n >= len(e.jobs) {
		return
	}
	e.jobs[i], e.jobs[n] = e.jobs[n], e.jobs[i]
	e.persistLocked()
}
func (e *Engine) Remove(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	i := e.index(id)
	if i < 0 {
		return nil
	}
	if e.jobs[i].State == model.Running || e.jobs[i].State == model.Scanning {
		return fmt.Errorf("cancel the job before removing it")
	}
	if e.cancels[id] != nil || e.scans[id] != nil {
		return fmt.Errorf("job is still stopping")
	}
	e.jobs = append(e.jobs[:i], e.jobs[i+1:]...)
	e.persistLocked()
	return nil
}

// ClearQueue cancels existing jobs and waits for their workers before removing
// them. Source files, saved encodes and published output are left intact. Jobs
// added while cancellation is underway are not part of this clear operation.
func (e *Engine) ClearQueue() error {
	e.mu.Lock()
	if e.clearing {
		e.mu.Unlock()
		return nil
	}
	e.clearing = true
	e.enabled = false
	ids := make(map[string]bool, len(e.jobs))
	for k := range e.jobs {
		j := &e.jobs[k]
		ids[j.ID] = true
		if cancel := e.cancels[j.ID]; cancel != nil {
			cancel()
		}
		if cancel := e.scans[j.ID]; cancel != nil {
			cancel()
		}
		if j.State != model.Complete {
			j.State = model.Canceled
			j.Stage = "Stopping — clearing queue"
		}
	}
	e.persistLocked()
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.clearing = false
		e.signal()
		e.mu.Unlock()
	}()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		e.mu.Lock()
		busy := false
		for id := range ids {
			if e.cancels[id] != nil || e.scans[id] != nil {
				busy = true
				break
			}
		}
		if !busy {
			remaining := make([]model.Job, 0, len(e.jobs))
			for _, j := range e.jobs {
				if !ids[j.ID] {
					remaining = append(remaining, j)
				} else {
					delete(e.pauses, j.ID)
				}
			}
			if err := e.store.Save(remaining); err != nil {
				e.lastError = "Cannot clear queue: " + err.Error()
				e.signal()
				e.mu.Unlock()
				return err
			}
			e.jobs = remaining
			e.signal()
			e.mu.Unlock()
			return nil
		}
		e.mu.Unlock()
		select {
		case <-e.ctx.Done():
			return e.ctx.Err()
		case <-tick.C:
		}
	}
}

func (e *Engine) Edit(id, title, videoID, label string, excluded *bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	i := e.index(id)
	if i < 0 {
		return fmt.Errorf("job not found")
	}
	j := &e.jobs[i]
	if j.State == model.Running || j.State == model.Scanning || j.State == model.Complete {
		return fmt.Errorf("pause the job before editing it")
	}
	if title != "" {
		j.Title = title
	}
	for k := range j.Videos {
		v := &j.Videos[k]
		if v.ID == videoID {
			if label != "" {
				v.Label = label
			}
			if excluded != nil {
				v.Excluded = *excluded
			}
		}
	}
	if excluded != nil {
		e.refreshReadinessLocked(j)
	}
	e.estimateLocked(j)
	e.persistLocked()
	return nil
}

// SetExcluded updates a selection atomically, with one estimate and queue save.
func (e *Engine) SetExcluded(id string, videoIDs []string, excluded bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	i := e.index(id)
	if i < 0 {
		return fmt.Errorf("job not found")
	}
	j := &e.jobs[i]
	if j.State == model.Running || j.State == model.Scanning || j.State == model.Complete {
		return fmt.Errorf("pause the job before editing it")
	}
	ids := make(map[string]bool, len(videoIDs))
	for _, id := range videoIDs {
		ids[id] = true
	}
	for _, v := range j.Videos {
		delete(ids, v.ID)
	}
	if len(ids) != 0 {
		return fmt.Errorf("one or more selected files no longer exist")
	}
	if len(videoIDs) == 0 {
		return nil
	}
	for _, id := range videoIDs {
		ids[id] = true
	}
	for k := range j.Videos {
		if ids[j.Videos[k].ID] {
			j.Videos[k].Excluded = excluded
		}
	}
	e.refreshReadinessLocked(j)
	e.estimateLocked(j)
	e.persistLocked()
	return nil
}

func (e *Engine) refreshReadinessLocked(j *model.Job) {
	if !j.ScanComplete || (j.State != model.Ready && j.State != model.Blocked) {
		return
	}
	active := j.ActiveVideos()
	if len(active) == 0 {
		j.State, j.Error = model.Blocked, "No included video files"
		return
	}
	for _, v := range active {
		if v.Error != "" {
			j.State, j.Error = model.Blocked, "One or more videos need attention; retry or exclude them"
			return
		}
	}
	if j.Settings.NoMenu && len(active) != 1 {
		j.State, j.Error = model.Blocked, "No menu requires exactly one included video; enable menus or exclude the other files"
		return
	}
	j.State, j.Stage, j.Error = model.Ready, "Ready", ""
}

func (e *Engine) SetSettings(id string, s model.Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if id == "" {
		if err := e.store.SaveSettings(s); err != nil {
			return err
		}
		e.settings = s
		for i := range e.jobs {
			if !e.jobs[i].Locked && e.jobs[i].State != model.Complete {
				e.jobs[i].Settings = s
				e.refreshReadinessLocked(&e.jobs[i])
				e.estimateLocked(&e.jobs[i])
			}
		}
	} else {
		i := e.index(id)
		if i < 0 {
			return fmt.Errorf("job not found")
		}
		if e.jobs[i].Locked {
			return fmt.Errorf("settings were locked when this job started; add the folder again for different encoding settings")
		}
		e.jobs[i].Settings = s
		e.refreshReadinessLocked(&e.jobs[i])
		e.estimateLocked(&e.jobs[i])
	}
	e.persistLocked()
	return nil
}
func (e *Engine) update(id string, save bool, fn func(*model.Job)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if i := e.index(id); i >= 0 {
		fn(&e.jobs[i])
		if save {
			e.persistLocked()
		} else {
			e.signal()
		}
	}
}
func (e *Engine) boundary(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lastError != "" {
		return errors.New(e.lastError)
	}
	if e.pauses[id] {
		return errPaused
	}
	return nil
}

func (e *Engine) loop() {
	defer e.wg.Done()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-e.wake:
		}
		for {
			e.mu.Lock()
			if !e.enabled {
				e.mu.Unlock()
				break
			}
			i := -1
			for k, j := range e.jobs {
				if j.State == model.Ready {
					i = k
					break
				}
			}
			if i < 0 {
				waiting := false
				for _, job := range e.jobs {
					if job.State == model.Scanning {
						waiting = true
						break
					}
				}
				if !waiting {
					e.enabled = false
					e.signal()
				}
				e.mu.Unlock()
				break
			}
			j := e.jobs[i]
			if j.Settings.NoMenu && len(j.ActiveVideos()) != 1 {
				e.jobs[i].State = model.Blocked
				e.jobs[i].Error = "No menu requires exactly one included video; enable menus or exclude the other files"
				e.persistLocked()
				e.mu.Unlock()
				continue
			}
			ctx, cancel := context.WithCancel(e.ctx)
			e.cancels[j.ID] = cancel
			delete(e.pauses, j.ID)
			e.jobs[i].State = model.Running
			e.jobs[i].Locked = true
			e.jobs[i].Error = ""
			j = e.jobs[i].Clone()
			e.persistLocked()
			e.mu.Unlock()
			err := e.run(ctx, j)
			cancel()
			e.mu.Lock()
			delete(e.cancels, j.ID)
			i = e.index(j.ID)
			if i >= 0 && err != nil {
				job := &e.jobs[i]
				switch {
				case errors.Is(err, errPaused):
					job.State = model.Paused
					job.Stage = "Paused — completed stages saved"
				case ctx.Err() != nil && errors.Is(err, context.Canceled):
					if job.State != model.Canceled {
						job.State = model.Paused
						job.Stage = "Interrupted — resume to continue"
					}
				default:
					job.State = model.Blocked
					job.Stage = "Needs attention"
					job.Error = err.Error()
				}
				e.persistLocked()
			}
			e.mu.Unlock()
			if e.ctx.Err() != nil {
				return
			}
		}
	}
}

func CopyFile(ctx context.Context, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024*1024)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		var n int
		n, err = in.Read(buf)
		if n > 0 {
			var w int
			w, err = out.Write(buf[:n])
			if err != nil {
				break
			}
			if w != n {
				err = io.ErrShortWrite
				break
			}
		}
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
	}
	if err == nil {
		err = out.Sync()
	}
	ce := out.Close()
	if err == nil {
		err = ce
	}
	return err
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(b)
}

func checkSpace(ctx context.Context, s model.Settings, videos []model.Video) error {
	for _, p := range []string{s.WorkDir, s.OutputDir} {
		if err := fsops.Mkdir(ctx, p); err != nil {
			return err
		}
	}
	var remaining, stage int64
	for _, v := range videos {
		if v.Encoded == "" {
			remaining += v.EstimatedBytes(s)
		}
		if v.Size > stage {
			stage = v.Size
		}
	}
	needWork := remaining + 2*s.Capacity() + (1 << 30)
	if s.StageLocally {
		needWork += stage * int64(s.Encoders+1)
	}
	output := remaining
	for _, v := range videos {
		output += v.EncodedSize
	}
	output += int64(len(videos))*packing.TitleReserve + packing.BaseReserve
	if strings.EqualFold(filepath.VolumeName(s.WorkDir), filepath.VolumeName(s.OutputDir)) {
		needWork += output
	}
	free, err := fsops.Free(ctx, s.WorkDir)
	if err != nil {
		return err
	}
	if free < uint64(needWork) {
		return fmt.Errorf("work volume needs about %.1f GB free; %.1f GB available", float64(needWork)/1e9, float64(free)/1e9)
	}
	free, err = fsops.Free(ctx, s.OutputDir)
	if err != nil {
		return err
	}
	if free < uint64(output) {
		return fmt.Errorf("output volume needs about %.1f GB free", float64(output)/1e9)
	}
	return nil
}

func (e *Engine) run(ctx context.Context, j model.Job) error {
	if err := e.tools.Require(); err != nil {
		return err
	}
	if err := j.Settings.Validate(); err != nil {
		return err
	}
	if err := fsops.RequireLocal(ctx, j.Settings.WorkDir); err != nil {
		return err
	}
	videos := j.ActiveVideos()
	if len(videos) == 0 {
		return fmt.Errorf("no included videos")
	}
	if j.Settings.NoMenu && len(videos) != 1 {
		return fmt.Errorf("no menu requires exactly one included video; enable menus or exclude the other files")
	}
	for _, v := range videos {
		if v.Error != "" && v.Duration <= 0 {
			return fmt.Errorf("%s: %s", v.Relative, v.Error)
		}
	}
	root := filepath.Join(j.Settings.WorkDir, "jobs", j.ID)
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if err := e.prepareLog(ctx, j); err != nil {
		return err
	}
	logFile, err := os.OpenFile(e.JobLogPath(j.ID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	log := &lockedWriter{w: logFile}
	e.update(j.ID, false, func(j *model.Job) { j.Stage = "Checking sources and saved encodes" })
	// Refresh every fingerprint before deciding whether a saved encode is reusable.
	for i, v := range videos {
		if err = e.boundary(ctx, j.ID); err != nil {
			return err
		}
		current, probeErr := e.tools.Probe(ctx, v.Path, v.Relative)
		if probeErr != nil {
			return fmt.Errorf("%s: %w", v.Relative, probeErr)
		}
		if current.Fingerprint() != v.Fingerprint() {
			current.Label = v.Label
			current.Excluded = v.Excluded
			v = current
		}
		if !media.ValidArtifact(ctx, v, j.Settings) {
			v.Encoded = ""
			v.EncodedHash = ""
			v.EncodedSize = 0
			v.Thumbnail = ""
		}
		v.Error = ""
		videos[i] = v
		e.updateVideo(j.ID, v, true)
	}
	if err = checkSpace(ctx, j.Settings, videos); err != nil {
		return err
	}
	if err = e.encode(ctx, j, videos, root, log); err != nil {
		return err
	}
	e.mu.Lock()
	if i := e.index(j.ID); i >= 0 {
		j = e.jobs[i].Clone()
	}
	e.mu.Unlock()
	videos = j.ActiveVideos()
	if err = e.boundary(ctx, j.ID); err != nil {
		return err
	}
	e.update(j.ID, false, func(j *model.Job) { j.Stage = "Finalizing disc layout"; j.Progress = .8 })
	var items []packing.Item
	for i, v := range videos {
		items = append(items, packing.Item{ID: v.ID, Bytes: v.EncodedSize, Order: i})
	}
	plan, err := packing.Plan(ctx, items, j.Settings.Capacity(), j.Settings.Packing == "Space efficiency", 10*time.Second)
	if err != nil {
		return err
	}
	// Re-authoring uses only encoded assets. Even overflow corrections never change video quality.
	for attempt := 0; attempt <= len(videos); attempt++ {
		if err = e.boundary(ctx, j.ID); err != nil {
			return err
		}
		keyBytes, _ := json.Marshal(struct {
			Template string
			Title    string
			S        model.Settings
			V        []model.Video
			D        []model.Disc
		}{"dvd-menu-v4", j.Title, j.Settings, videos, plan.Discs})
		generation := model.Hash(string(keyBytes))[:16]
		genDir := filepath.Join(root, "generation-"+generation)
		old := j.Discs
		if generation == j.Generation && len(old) == len(plan.Discs) {
			for k := range plan.Discs {
				if reflect.DeepEqual(old[k].VideoIDs, plan.Discs[k].VideoIDs) {
					plan.Discs[k].ISO = old[k].ISO
					plan.Discs[k].SHA256 = old[k].SHA256
				}
			}
		}
		j.Generation = generation
		j.MinimumProven = plan.Proven
		e.update(j.ID, true, func(job *model.Job) {
			job.Generation = generation
			job.Discs = append([]model.Disc(nil), plan.Discs...)
			job.MinimumProven = plan.Proven
		})
		overflow := -1
		for k := range plan.Discs {
			if err = e.boundary(ctx, j.ID); err != nil {
				return err
			}
			d := &plan.Discs[k]
			valid := false
			if d.ISO != "" {
				h, he := media.HashFile(ctx, d.ISO)
				st, se := os.Stat(d.ISO)
				valid = he == nil && se == nil && h == d.SHA256 && st.Size() <= j.Settings.Capacity()
				if valid {
					d.ISOBytes = st.Size()
				}
			}
			if !valid {
				iso, ae := e.tools.Author(ctx, j, *d, len(plan.Discs), filepath.Join(genDir, fmt.Sprintf("disc-%03d", k+1)), log, func(stage string) error {
					if err := e.boundary(ctx, j.ID); err != nil {
						return err
					}
					e.update(j.ID, false, func(job *model.Job) {
						job.Stage = fmt.Sprintf("%s · DVD %d of %d", stage, k+1, len(plan.Discs))
						job.Progress = .8 + .15*float64(k)/float64(len(plan.Discs))
					})
					return nil
				})
				if ae != nil {
					return ae
				}
				st, se := os.Stat(iso)
				if se != nil {
					return se
				}
				if st.Size() > j.Settings.Capacity() {
					overflow = k
					break
				}
				d.ISO = iso
				d.ISOBytes = st.Size()
				d.SHA256, err = media.HashFile(ctx, iso)
				if err != nil {
					return err
				}
			}
			saved := append([]model.Disc(nil), plan.Discs...)
			e.update(j.ID, true, func(job *model.Job) { job.Discs = saved })
		}
		if overflow >= 0 {
			if len(plan.Discs[overflow].VideoIDs) <= 1 {
				return fmt.Errorf("video plus DVD menus exceeds disc capacity; choose DVD-9 or exclude that video")
			}
			ids := plan.Discs[overflow].VideoIDs
			move := ids[len(ids)-1]
			plan.Discs[overflow].VideoIDs = ids[:len(ids)-1]
			if j.Settings.Packing == "Strict order" {
				plan.Discs = append(plan.Discs[:overflow+1], append([]model.Disc{{VideoIDs: []string{move}}}, plan.Discs[overflow+1:]...)...)
			} else {
				plan.Discs = append(plan.Discs, model.Disc{VideoIDs: []string{move}})
			}
			for k := range plan.Discs {
				d := &plan.Discs[k]
				d.Number = k + 1
				d.ISO = ""
				d.SHA256 = ""
				d.ISOBytes = 0
			}
			plan.Proven = false
			j.Generation = ""
			continue
		}
		if err = e.boundary(ctx, j.ID); err != nil {
			return err
		}
		return e.publish(ctx, j, plan.Discs, root, genDir, log)
	}
	return fmt.Errorf("could not produce a capacity-safe disc layout")
}

func (e *Engine) updateVideo(id string, v model.Video, save bool) {
	e.update(id, save, func(j *model.Job) {
		for i := range j.Videos {
			if j.Videos[i].ID == v.ID {
				j.Videos[i] = v
				break
			}
		}
	})
}

func (e *Engine) encode(ctx context.Context, j model.Job, videos []model.Video, root string, log io.Writer) error {
	type task struct {
		v      model.Video
		input  string
		staged bool
	}
	tasks := make(chan task, 1)
	stageSlots := make(chan struct{}, j.Settings.Encoders+1)
	var wg sync.WaitGroup
	var failure atomic.Bool
	var firstErr error
	var errMu sync.Mutex
	fail := func(v model.Video, err error) {
		failure.Store(true)
		errMu.Lock()
		if firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", v.Relative, err)
		}
		errMu.Unlock()
		if !errors.Is(err, context.Canceled) && !errors.Is(err, errPaused) {
			v.Error = err.Error()
			e.updateVideo(j.ID, v, true)
		}
	}
	var done atomic.Int64
	progresses := map[string]float64{}
	var progressMu sync.Mutex
	for n := 0; n < j.Settings.Encoders; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				release := func() {
					if task.staged {
						<-stageSlots
					}
				}
				v := task.v
				if failure.Load() {
					release()
					continue
				}
				if err := e.boundary(ctx, j.ID); err != nil {
					fail(v, err)
					release()
					continue
				}
				dir := filepath.Join(root, "assets", v.ID)
				if err := os.MkdirAll(dir, 0755); err != nil {
					fail(v, err)
					release()
					continue
				}
				if v.Encoded == "" && (!v.CropChecked || v.CropVersion != media.PillarboxVersion) {
					e.update(j.ID, false, func(job *model.Job) { job.Stage = "Checking black borders · " + v.Label })
					var err error
					v, err = e.tools.DetectPillarbox(ctx, v, task.input, log)
					if err != nil {
						fail(v, err)
						release()
						continue
					}
				}
				v.Thumbnail = filepath.Join(dir, "thumbnail.jpg")
				if _, err := os.Stat(v.Thumbnail); err != nil {
					if err = e.tools.Thumbnail(ctx, v, task.input, v.Thumbnail, log); err != nil {
						fail(v, err)
						release()
						continue
					}
				}
				if v.Encoded == "" {
					var err error
					v, err = e.tools.Encode(ctx, v, j.Settings, task.input, dir, log, func(p float64) {
						progressMu.Lock()
						progresses[v.ID] = p
						sum := float64(done.Load())
						for _, f := range progresses {
							sum += f
						}
						progressMu.Unlock()
						e.update(j.ID, false, func(job *model.Job) {
							job.Stage = fmt.Sprintf("Encoding · %d of %d complete", done.Load(), len(videos))
							job.Progress = .8 * sum / float64(len(videos))
						})
					})
					if err != nil {
						fail(v, err)
						release()
						continue
					}
				}
				current, sourceErr := fsops.Stat(ctx, v.Path)
				if sourceErr != nil {
					fail(v, sourceErr)
					release()
					continue
				}
				if !current.Exists || current.Size != v.Size || current.ModTime != v.ModTime {
					fail(v, fmt.Errorf("source changed during processing; retry to encode the current version"))
					release()
					continue
				}
				v.Error = ""
				e.updateVideo(j.ID, v, true)
				progressMu.Lock()
				delete(progresses, v.ID)
				done.Add(1)
				progressMu.Unlock()
				if task.staged {
					_ = os.Remove(task.input)
				}
				release()
			}
		}()
	}
	// One producer and a one-item buffer give bounded per-file staging with one-file prefetch.
	for _, v := range videos {
		if failure.Load() {
			break
		}
		if err := e.boundary(ctx, j.ID); err != nil {
			fail(v, err)
			break
		}
		input := v.Path
		staged := false
		if v.Encoded == "" && j.Settings.StageLocally {
			dir := filepath.Join(root, "staged")
			if err := os.MkdirAll(dir, 0755); err != nil {
				fail(v, err)
				break
			}
			part := filepath.Join(dir, v.ID+".part")
			input = filepath.Join(dir, v.ID+filepath.Ext(v.Path))
			e.update(j.ID, false, func(job *model.Job) { job.Stage = "Staging " + v.Relative })
			select {
			case stageSlots <- struct{}{}:
			case <-ctx.Done():
				fail(v, ctx.Err())
				break
			}
			if failure.Load() {
				break
			}
			if err := e.boundary(ctx, j.ID); err != nil {
				<-stageSlots
				fail(v, err)
				break
			}
			if err := fsops.Copy(ctx, v.Path, part); err != nil {
				fail(v, err)
				break
			}
			st, err := os.Stat(part)
			if err != nil || st.Size() != v.Size {
				fail(v, fmt.Errorf("source changed or staging copy is incomplete"))
				break
			}
			original, err := fsops.Stat(ctx, v.Path)
			if err != nil || !original.Exists || original.Size != v.Size || original.ModTime != v.ModTime {
				fail(v, fmt.Errorf("source changed during staging"))
				break
			}
			if err = os.Rename(part, input); err != nil {
				fail(v, err)
				break
			}
			staged = true
		}
		select {
		case tasks <- task{v, input, staged}:
		case <-ctx.Done():
			fail(v, ctx.Err())
		}
	}
	close(tasks)
	wg.Wait()
	return firstErr
}

func (e *Engine) publish(ctx context.Context, j model.Job, discs []model.Disc, root, genDir string, log io.Writer) error {
	e.update(j.ID, false, func(j *model.Job) { j.Stage = "Validating and publishing ISO set"; j.Progress = .96 })
	name := "Set-" + model.SafeName(j.Title) + "-" + j.ID[:8]
	dest := filepath.Join(j.Settings.OutputDir, name)
	partial := filepath.Join(j.Settings.OutputDir, ".easydvd-"+j.ID+".partial")
	destInfo, statErr := fsops.Stat(ctx, dest)
	if statErr != nil {
		return statErr
	}
	if destInfo.Exists {
		// The local manifest is checkpointed before the atomic output rename.
		// Accept legacy output manifests for jobs interrupted by an older build.
		b, err := os.ReadFile(e.ManifestPath(j.ID))
		if os.IsNotExist(err) {
			cached := filepath.Join(root, "published-manifest.json")
			if err = fsops.Copy(ctx, filepath.Join(dest, "manifest.json"), cached); err == nil {
				b, err = os.ReadFile(cached)
			}
		}
		var saved model.Job
		if err != nil || json.Unmarshal(b, &saved) != nil || saved.ID != j.ID || saved.Generation != j.Generation {
			return fmt.Errorf("output folder already exists: %s", dest)
		}
		for _, d := range saved.Discs {
			h, err := fsops.Hash(ctx, d.ISO)
			if err != nil || h != d.SHA256 {
				return fmt.Errorf("existing published ISO failed verification: %s", d.ISO)
			}
		}
		if err := e.saveManifest(saved); err != nil {
			return err
		}
		e.update(j.ID, true, func(job *model.Job) { *job = saved })
		return nil
	}
	if err := fsops.ResetPartial(ctx, partial, j.ID); err != nil {
		return err
	}
	for i := range discs {
		filename := fmt.Sprintf("DVD-%03d-of-%03d.iso", i+1, len(discs))
		path := filepath.Join(partial, filename)
		if err := fsops.Copy(ctx, discs[i].ISO, path); err != nil {
			return err
		}
		h, err := fsops.Hash(ctx, path)
		if err != nil {
			return err
		}
		if h != discs[i].SHA256 {
			return fmt.Errorf("output copy checksum mismatch")
		}
		discs[i].ISO = filepath.Join(dest, filename)
	}
	// Previews are disposable work files, never part of the published ISO set.
	previews := PreviewDir(j)
	err := os.MkdirAll(previews, 0755)
	if err == nil {
		err = filepath.WalkDir(genDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, "-preview.png") {
				relative, _ := filepath.Rel(genDir, path)
				return CopyFile(ctx, path, filepath.Join(previews, relative))
			}
			return nil
		})
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintf(log, "Temporary menu preview cache unavailable (can be regenerated): %v\n", err)
	}
	j.Discs = discs
	j.OutputPath = dest
	j.State = model.Complete
	j.Stage = "Complete"
	j.Progress = 1
	j.Error = ""
	now := time.Now()
	j.Finished = &now
	if err = e.saveManifest(j); err != nil {
		return err
	}
	if err = fsops.Rename(ctx, partial, dest); err != nil {
		return err
	}
	e.update(j.ID, true, func(job *model.Job) { *job = j })
	if !j.Settings.KeepWork {
		// Remove only generated media of this exact job; persistent records live elsewhere.
		for _, sub := range []string{"assets", "staged"} {
			if err = os.RemoveAll(filepath.Join(root, sub)); err != nil {
				fmt.Fprintf(log, "Cleanup: %v\n", err)
			}
		}
		entries, _ := os.ReadDir(root)
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "generation-") {
				if err = os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
					fmt.Fprintf(log, "Cleanup: %v\n", err)
				}
			}
		}
	}
	return nil
}
