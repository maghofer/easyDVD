package media

import (
	"context"
	"easydvd/internal/model"
	"easydvd/internal/process"
	"fmt"
	"io"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var cropPattern = regexp.MustCompile(`crop=(-?\d+):(-?\d+):(-?\d+):(-?\d+)`)

const PillarboxVersion = 2

// pillarboxEvidence keeps the union of visible content over the entire video.
// A single wider frame prevents cropping. Letterbox bars are never removed.
type pillarboxEvidence struct {
	width, height, border  int
	tolerance, left, right int
	frames, visible        int
	safe                   bool
	pending                string
}

func (p *pillarboxEvidence) crops() (int, int) {
	return min(p.border, p.left) / 2 * 2, min(p.border, p.width-p.right) / 2 * 2
}

func (p *pillarboxEvidence) Write(data []byte) (int, error) {
	p.pending += string(data)
	end := strings.LastIndexByte(p.pending, '\n')
	if end < 0 {
		return len(data), nil
	}
	complete := p.pending[:end+1]
	p.pending = p.pending[end+1:]
	for _, match := range cropPattern.FindAllStringSubmatch(complete, -1) {
		p.frames++
		values := [4]int{}
		for i := range values {
			values[i], _ = strconv.Atoi(match[i+1])
		}
		w, h, x, y := values[0], values[1], values[2], values[3]
		if w <= 0 || h <= 0 {
			continue
		} // all-black frame
		if x < 0 || y < 0 || x+w > p.width || y+h > p.height {
			p.safe = false
			continue
		}
		p.visible++
		if p.visible == 1 {
			p.left, p.right = x, x+w
		} else {
			p.left, p.right = min(p.left, x), max(p.right, x+w)
		}
		if x < p.border-p.tolerance || x+w > p.width-p.border+p.tolerance {
			p.safe = false
		}
	}
	return len(data), nil
}

func (t Tools) DetectPillarbox(ctx context.Context, v model.Video, input string, log io.Writer) (model.Video, error) {
	if v.CropChecked && v.CropVersion == PillarboxVersion {
		return v, nil
	}
	v.CropLeft, v.CropRight = 0, 0
	v.CropVersion = PillarboxVersion
	// Narrow scope: only a 16:9 container containing a centered 4:3 image.
	if math.Abs(v.DAR-16.0/9) > .025 || v.Width <= 0 || v.Height <= 0 {
		v.CropChecked = true
		return v, nil
	}
	border := int(math.Floor(float64(v.Width)*(1-(4.0/3)/v.DAR)/4)) * 2
	if border < 2 {
		v.CropChecked = true
		return v, nil
	}
	evidence := &pillarboxEvidence{width: v.Width, height: v.Height, border: border, tolerance: max(2, v.Width/200), safe: true}
	null := "/dev/null"
	if runtime.GOOS == "windows" {
		null = "NUL"
	}
	// Inspect every decoded frame, before any resize. The low threshold is
	// deliberate: noisy or uncertain borders retain the full source image.
	// bbox checks individual pixels. cropdetect's line averages could miss a
	// small logo or a faint detail in an otherwise black border. Keep 16-bit
	// precision while normalizing full/limited range to a common black level.
	// Allow two 8-bit luma levels above nominal black for compression ringing;
	// ProRes side borders can contain this invisible residue across a block.
	err := process.Run(ctx, t.FFmpeg, []string{"-hide_banner", "-nostdin", "-v", "info", "-xerror", "-i", input, "-map", fmt.Sprintf("0:%d", v.VideoStream), "-vf", "scale=iw:ih:in_range=auto:out_range=tv,format=yuv444p16le,bbox=min_val=4608", "-an", "-sn", "-dn", "-f", "null", null}, process.Options{Log: evidence})
	if err != nil {
		return v, fmt.Errorf("checking pillarbox borders: %w", err)
	}
	v.CropChecked = true
	if evidence.safe && evidence.visible >= 3 {
		// Include the union of every frame's visible pixels, rounded outward
		// for chroma alignment. A few pixels of codec residue at the 4:3 edge
		// must expand the kept image instead of vetoing all border removal.
		v.CropLeft, v.CropRight = evidence.crops()
		if log != nil {
			fmt.Fprintf(log, "\nConfirmed 4:3 pillarbox: %s, cropped %d left / %d right black pixels after checking %d frames; kept all content bounds [%d,%d)\n", v.Relative, v.CropLeft, v.CropRight, evidence.frames, evidence.left, evidence.right)
		}
	} else if log != nil {
		fmt.Fprintf(log, "\nRetained 16:9 source: %s; %d visible frames, content bounds [%d,%d), expected 4:3 bounds [%d,%d), edge tolerance %d pixels\n", v.Relative, evidence.visible, evidence.left, evidence.right, border, v.Width-border, evidence.tolerance)
	}
	return v, nil
}
