package media

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"easydvd/internal/model"
	"easydvd/internal/process"
)

// Version 1 intentionally accepts a narrow, independently checkable subset:
// progressive I/P AVC, one short-term reference, closed IDR GOPs, four slices,
// AUDs, repeated parameter sets and single-CPB HRD timing. B pictures, fields,
// missing signaling and unfamiliar parameter changes are encoded instead.
// FFmpeg parses every header; no encoder-name or profile-label whitelist is used.
func (t Tools) verifyBlurayAVC(ctx context.Context, v model.Video, input string) (bool, string, error) {
	p := v.Bluray
	reject := func(reason string) (bool, string, error) { return false, "Encode video: " + reason, nil }
	if v.Source.Codec != "h264" {
		return reject("source is not H.264")
	}
	if v.Rotation != 0 || v.CropLeft+v.CropRight != 0 || v.Width != p.Width || v.Height != p.Height {
		return reject("picture transformation required")
	}
	if v.Source.PixelFormat != "yuv420p" {
		return reject("requires 8-bit 4:2:0")
	}
	if p.Mode != "720" && !(p.Mode == "1080" && (p.FPSNum == 24 || p.FPSNum == 24000)) {
		return reject("interlaced signaling is outside verifier v1's copy subset")
	}
	if math.Abs(v.FPS-float64(p.FPSNum)/float64(p.FPSDen)) > .0001 {
		return reject("frame-rate conversion required")
	}
	if v.Source.ColorPrimaries != "bt709" || v.Source.ColorSpace != "bt709" || v.Source.ColorTransfer != "bt709" || v.Source.ColorRange != "tv" {
		return reject("color signaling requires normalization")
	}
	if Ratio(v.Source.TimeBase) <= 0 {
		return reject("packet time base is unavailable")
	}
	inspection, cancel := context.WithCancel(ctx)
	defer cancel()
	scan := &avcVerifier{target: p, timeBase: Ratio(v.Source.TimeBase), abort: cancel}
	err := process.Run(inspection, t.FFmpeg, []string{"-hide_banner", "-nostdin", "-v", "info", "-xerror", "-i", input, "-map", fmt.Sprintf("0:%d", v.VideoStream), "-an", "-c:v", "copy", "-bsf:v", "h264_mp4toannexb,trace_headers", "-f", "null", "-"}, process.Options{Stderr: scan})
	scan.finish()
	if ctx.Err() != nil {
		return false, "", ctx.Err()
	}
	if scan.reason != "" {
		return reject(scan.reason)
	}
	if err != nil {
		return reject("full-stream inspection could not establish compatibility")
	}
	return true, "Copy verified H.264 (conservative verifier v1)", nil
}

var packetHeader = regexp.MustCompile(`Packet: (\d+) bytes,.*pts (-?\d+), dts (-?\d+), duration (\d+)\.`)

type avcVerifier struct {
	abort                                       context.CancelFunc
	packetBuffer                                bool
	initialDelay                                int64
	target                                      model.BlurayPlan
	timeBase                                    float64
	pending                                     string
	reason                                      string
	sps                                         map[string]int64
	spsSignature                                string
	nal                                         int64
	inSPS                                       bool
	pps                                         bool
	packets, sinceIDR, aud, slices, timing, idr int
	packetSPS, packetPPS                        bool
	lastDTS                                     float64
	rate, buffer, bucket                        float64
	packetDelay, lastDelay, delayLength         int64
}

func (v *avcVerifier) fail(reason string) {
	if v.reason == "" {
		v.reason = reason
		if v.abort != nil {
			v.abort()
		}
	}
}
func (v *avcVerifier) Write(data []byte) (int, error) {
	if v.reason != "" {
		return len(data), nil
	}
	v.pending += string(data)
	for {
		i := strings.IndexByte(v.pending, '\n')
		if i < 0 {
			break
		}
		v.line(v.pending[:i])
		v.pending = v.pending[i+1:]
	}
	if len(v.pending) > 16384 {
		v.fail("unrecognized inspection output")
		v.pending = ""
	}
	return len(data), nil
}

func (v *avcVerifier) parameterSet() {
	if !v.inSPS {
		return
	}
	v.inSPS = false
	s := v.sps
	need := []string{"profile_idc", "level_idc", "max_num_ref_frames", "frame_mbs_only_flag", "pic_width_in_mbs_minus1", "pic_height_in_map_units_minus1", "num_units_in_tick", "time_scale", "fixed_frame_rate_flag", "nal_hrd_parameters_present_flag", "bit_rate_value_minus1[0]", "cpb_size_value_minus1[0]", "bit_rate_scale", "cpb_size_scale", "max_num_reorder_frames", "max_dec_frame_buffering", "pic_order_cnt_type", "cpb_removal_delay_length_minus1", "colour_primaries", "transfer_characteristics", "matrix_coefficients", "aspect_ratio_idc"}
	for _, k := range need {
		if _, ok := s[k]; !ok {
			v.fail("incomplete AVC timing/buffer signaling")
			return
		}
	}
	if s["profile_idc"] != 100 && s["profile_idc"] != 77 || s["level_idc"] != 41 && s["level_idc"] != 40 || s["max_num_ref_frames"] > 1 || s["max_dec_frame_buffering"] > 1 || s["max_num_reorder_frames"] != 0 || s["frame_mbs_only_flag"] != 1 || s["seq_parameter_set_id"] != 0 || s["cpb_cnt_minus1"] != 0 || s["fixed_frame_rate_flag"] != 1 || s["nal_hrd_parameters_present_flag"] != 1 || s["vcl_hrd_parameters_present_flag"] != 0 {
		v.fail("AVC parameters are outside verifier v1's supported subset")
		return
	}
	if s["profile_idc"] == 100 && (s["chroma_format_idc"] != 1 || s["bit_depth_luma_minus8"] != 0 || s["bit_depth_chroma_minus8"] != 0) {
		v.fail("AVC sample format is unsupported")
		return
	}
	if s["pic_order_cnt_type"] != 2 || s["colour_primaries"] != 1 || s["transfer_characteristics"] != 1 || s["matrix_coefficients"] != 1 || s["video_full_range_flag"] != 0 || s["aspect_ratio_idc"] != 1 {
		v.fail("AVC picture-order or color/aspect signaling requires normalization")
		return
	}
	v.delayLength = s["cpb_removal_delay_length_minus1"] + 1
	if v.delayLength < 1 || v.delayLength > 32 {
		v.fail("unsupported HRD delay width")
		return
	}
	w := 16*(s["pic_width_in_mbs_minus1"]+1) - 2*(s["frame_crop_left_offset"]+s["frame_crop_right_offset"])
	h := 16*(s["pic_height_in_map_units_minus1"]+1) - 2*(s["frame_crop_top_offset"]+s["frame_crop_bottom_offset"])
	if int(w) != v.target.Width || int(h) != v.target.Height || s["num_units_in_tick"] <= 0 || math.Abs(float64(s["time_scale"])/(2*float64(s["num_units_in_tick"]))-float64(v.target.FPSNum)/float64(v.target.FPSDen)) > .00001 {
		v.fail("AVC dimensions or timing do not match the target")
		return
	}
	if s["bit_rate_scale"] > 15 || s["cpb_size_scale"] > 15 || s["bit_rate_value_minus1[0]"] < 0 || s["cpb_size_value_minus1[0]"] < 0 {
		v.fail("invalid AVC buffer parameters")
		return
	}
	v.rate = float64(s["bit_rate_value_minus1[0]"]+1) * math.Exp2(float64(6+s["bit_rate_scale"]))
	v.buffer = float64(s["cpb_size_value_minus1[0]"]+1) * math.Exp2(float64(4+s["cpb_size_scale"]))
	if v.rate <= 0 || v.rate > 40000000 || v.buffer <= 0 || v.buffer > 30000000 {
		v.fail("AVC bitrate or buffer exceeds Blu-ray limits")
		return
	}
	sig := fmt.Sprint(s)
	if v.spsSignature != "" && sig != v.spsSignature {
		v.fail("changing AVC parameter sets require encoding")
	}
	v.spsSignature = sig
}

func (v *avcVerifier) packetEnd() {
	if v.packets == 0 {
		return
	}
	if v.aud != 1 || v.slices < 4 || v.timing != 1 || !v.pps || v.spsSignature == "" {
		v.fail("missing access-unit, slice or picture-timing evidence")
	}
	if v.packets == 1 && v.idr == 0 {
		v.fail("video does not start at an IDR picture")
	}
	if v.packets > 1 && v.packetDelay != (v.lastDelay+2)%(int64(1)<<uint(v.delayLength)) {
		v.fail("inconsistent HRD removal timing")
	}
	v.lastDelay = v.packetDelay
	if v.idr > 0 {
		if !v.packetBuffer {
			v.fail("IDR lacks an HRD buffering period")
		}
		if !v.packetSPS || !v.packetPPS {
			v.fail("IDR lacks repeated parameter sets")
		}
		v.sinceIDR = 0
		// A buffering-period SEI establishes the new clock origin after this AU.
		v.lastDelay = 0
	} else if v.packetBuffer {
		v.fail("unexpected buffering-period change")
	}
	v.sinceIDR++
	if v.sinceIDR > int(math.Ceil(float64(v.target.FPSNum)/float64(v.target.FPSDen))) {
		v.fail("GOP exceeds one second")
	}
}

func (v *avcVerifier) line(line string) {
	if v.reason != "" {
		return
	}
	if strings.Contains(line, "Packet:") {
		v.parameterSet()
		v.packetEnd()
		m := packetHeader.FindStringSubmatch(line)
		if len(m) != 5 {
			v.fail("packet timing is unavailable")
			return
		}
		get := func(i int) float64 { f, _ := strconv.ParseFloat(m[i], 64); return f }
		dts, duration := get(3)*v.timeBase, get(4)*v.timeBase
		step := float64(v.target.FPSDen) / float64(v.target.FPSNum)
		if math.Abs((get(2)-get(3))*v.timeBase) > 2*v.timeBase || math.Abs(duration-step) > 2*v.timeBase+.000001 || v.packets > 0 && math.Abs(dts-v.lastDTS-step) > 2*v.timeBase+.000001 {
			v.fail("variable or discontinuous frame timing")
			return
		}
		if v.packets > 0 {
			v.bucket = math.Max(0, v.bucket-v.rate*(dts-v.lastDTS))
		}
		v.bucket += get(1) * 8
		if v.buffer <= 0 || v.bucket > v.buffer {
			v.fail("packet bitrate exceeds the signaled buffer envelope")
			return
		}
		v.lastDTS = dts
		v.packets++
		v.aud = 0
		v.slices = 0
		v.timing = 0
		v.idr = 0
		v.packetSPS = false
		v.packetPPS = false
		v.packetBuffer = false
		return
	}
	i := strings.LastIndex(line, " = ")
	if i < 0 {
		return
	}
	fields := strings.Fields(line[:i])
	if len(fields) < 3 {
		return
	}
	name := fields[len(fields)-2]
	n, err := strconv.ParseInt(strings.TrimSpace(line[i+3:]), 10, 64)
	if err != nil {
		return
	}
	if name == "nal_unit_type" {
		v.parameterSet()
		v.nal = n
		if n == 7 {
			v.inSPS = true
			v.sps = map[string]int64{}
			v.packetSPS = true
		}
		if n == 8 {
			v.pps = true
			v.packetPPS = true
		}
		if n == 9 {
			v.aud++
		}
		if n == 5 {
			v.idr++
		}
		if n < 1 || n > 12 || n == 2 || n == 3 || n == 4 {
			v.fail("unsupported AVC NAL unit")
		}
	}
	if v.inSPS {
		v.sps[name] = n
	}
	switch name {
	case "slice_type":
		v.slices++
		if n%5 != 0 && n%5 != 2 || n%5 == 2 && v.nal != 5 {
			v.fail("B pictures or non-IDR intra pictures require encoding in verifier v1")
		}
	case "num_slice_groups_minus1", "weighted_pred_flag", "weighted_bipred_idc", "long_term_reference_flag", "adaptive_ref_pic_marking_mode_flag", "pic_parameter_set_id", "num_ref_idx_l0_default_active_minus1", "num_ref_idx_l0_active_minus1":
		if n != 0 {
			v.fail("unsupported prediction or reference-picture signaling")
		}
	case "initial_cpb_removal_delay[0]":
		v.packetBuffer = true
		v.initialDelay = n
		if n <= 0 || v.rate <= 0 || float64(n) > 90000*v.buffer/v.rate+1 {
			v.fail("invalid initial HRD buffering delay")
		}
	case "dpb_output_delay":
		if n != 0 {
			v.fail("picture reordering is outside verifier v1's copy subset")
		}
	case "initial_cpb_removal_delay_offset[0]":
		if n < 0 || v.rate <= 0 || float64(n+v.initialDelay) > 90000*v.buffer/v.rate+1 {
			v.fail("invalid HRD delay offset")
		}
	case "cpb_removal_delay":
		v.timing++
		v.packetDelay = n
	}
}

func (v *avcVerifier) finish() {
	if v.pending != "" {
		v.line(v.pending)
		v.pending = ""
	}
	v.parameterSet()
	v.packetEnd()
	if v.packets < 2 {
		v.fail("insufficient full-stream timing evidence")
	}
}
