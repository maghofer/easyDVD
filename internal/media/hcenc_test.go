package media

import (
	"context"
	"debug/pe"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"easydvd/internal/model"
)

func TestHCencAVStoDVDProfile(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.EncodingProfile = model.ProfileQuality
	v := model.Video{Width: 1920, Height: 1080, DAR: 16.0 / 9, FPS: 25, FieldOrder: "progressive"}
	for _, standard := range []string{"PAL", "NTSC"} {
		s.Standard = standard
		ini := HCencINI(v, s, false)
		for _, line := range []string{"*AVSRELOAD", "*BITRATE 8500", "*MAXBITRATE 9000", "*PROFILE BEST", "*ASPECT 16:9", "*PROGRESSIVE", "*TFF", "*BIAS 80", "*DC_PREC 10", "*MATRIX MPEG", "*AQ 1", "*LUMGAIN 1", "*SMP", "*PRIORITY IDLE"} {
			if !strings.Contains("\n"+ini, "\n"+line+"\n") {
				t.Fatal("AVStoDVD setting missing", line, ini)
			}
		}
		if strings.Contains(ini, "*1PASS") || strings.Contains(ini, "*CQ") || strings.Contains(ini, "*GOP") {
			t.Fatal("two-pass VBR / automatic GOP settings changed", ini)
		}
	}
	v.DAR, v.FieldOrder = 4.0/3, "bb"
	ini := HCencINI(v, s, true)
	if !strings.Contains(ini, "*INTERLACED\n*BFF") || !strings.Contains(ini, "*ASPECT 4:3") || strings.Contains(ini, "*PROGRESSIVE") {
		t.Fatal("source field order/aspect lost", ini)
	}
}

func TestHCencSourceAndAudioSelection(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.EncodingProfile = model.ProfileQuality
	v := model.Video{Width: 720, Height: 576, FPS: 25, DAR: 4.0 / 3, Duration: 10, VideoStream: 2, AudioStream: 4, FieldOrder: "bb"}
	prep := strings.Join(HCencPrepareArgs(v, s, "source", "input.mkv"), " ")
	for _, want := range []string{"-map 0:2", "interl=1", "-c:v ffv1", "-pix_fmt yuv420p", "-an", "-t 10.000000"} {
		if !strings.Contains(prep, want) {
			t.Fatal(want, prep)
		}
	}
	for _, audio := range []int{4, -1} {
		v.AudioStream = audio
		mux := strings.Join(HCencMuxArgs(v, s, "source", "video.m2v", "output.mpg"), " ")
		for _, want := range []string{"-map 0:v:0", "-c:v copy", "-b:a 448k", "-ar 48000", "-ac 2", "-af apad", "-shortest", "-t 10.000000", "-f dvd", "-muxrate 10080k", "-packetsize 2048"} {
			if !strings.Contains(mux, want) {
				t.Fatal(want, mux)
			}
		}
		if audio == 4 && (!strings.Contains(mux, "-map 1:4") || strings.Contains(mux, "anullsrc")) || audio == -1 && (!strings.Contains(mux, "anullsrc") || !strings.Contains(mux, "-map 1:a:0")) {
			t.Fatal("selected audio/silence mapping changed", mux)
		}
	}
}

func TestMissingHCencDoesNotFallBackToFFmpeg(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	if err := (Tools{}).RequireProfile(s); err != nil {
		t.Fatal("Balanced acquired an HCEnc dependency", err)
	}
	s.EncodingProfile = model.ProfileQuality
	_, err := (Tools{}).Encode(context.Background(), model.Video{}, s, "input", filepath.Join(t.TempDir(), "encode"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "HCEnc") {
		t.Fatal("Quality must require HCEnc before touching source media", err)
	}
}

func TestExplicitHCencRootDoesNotFallBack(t *testing.T) {
	t.Setenv("EASYDVD_HCENC", filepath.Join(t.TempDir(), "missing-runtime"))
	encoder, plugin, avs := findHCenc()
	if encoder != "" || plugin != "" || avs != "" {
		t.Fatal("explicit missing runtime silently selected another installation", encoder, plugin, avs)
	}
}

func TestPortableHCencRuntimeIsAtomic(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows runtime discovery")
	}
	root := t.TempDir()
	quality := filepath.Join(root, "quality")
	if err := os.MkdirAll(quality, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EASYDVD_HCENC", root)
	for _, name := range []string{"HCenc.exe", "ffms2.dll"} {
		if err := os.WriteFile(filepath.Join(quality, name), []byte("discovery fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	enc, plugin, avs := findHCenc()
	if enc != filepath.Join(quality, "HCenc.exe") || plugin != filepath.Join(quality, "ffms2.dll") || avs != "" {
		t.Fatal("partial portable runtime mixed in an installed AviSynth", enc, plugin, avs)
	}
	if err := os.WriteFile(filepath.Join(quality, "avisynth.dll"), []byte("discovery fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, avs = findHCenc()
	if avs != filepath.Join(quality, "avisynth.dll") {
		t.Fatal("portable AviSynth was not selected", avs)
	}
	if err := os.Remove(filepath.Join(quality, "HCenc.exe")); err != nil {
		t.Fatal(err)
	}
	enc, _, _ = findHCenc()
	if enc != "" {
		t.Fatal("empty portable folder silently fell back to an installed encoder", enc)
	}
}

func TestPortableAviSynthImportWithoutOriginalThunk(t *testing.T) {
	// Minimal PE32 image with a name/first thunk but OriginalFirstThunk=0,
	// matching the import descriptor layout used by the shipped AviSynth DLL.
	data := make([]byte, 1024)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[0x3c:], 0x80)
	copy(data[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[0x84:], pe.IMAGE_FILE_MACHINE_I386)
	binary.LittleEndian.PutUint16(data[0x86:], 1)
	binary.LittleEndian.PutUint16(data[0x94:], 224)
	binary.LittleEndian.PutUint16(data[0x98:], 0x10b)
	binary.LittleEndian.PutUint32(data[0x98+92:], 16)
	binary.LittleEndian.PutUint32(data[0x98+96+8:], 0x1000)
	binary.LittleEndian.PutUint32(data[0x98+96+12:], 40)
	section := 0x98 + 224
	copy(data[section:], ".idata")
	for offset, value := range map[int]uint32{8: 512, 12: 0x1000, 16: 512, 20: 512} {
		binary.LittleEndian.PutUint32(data[section+offset:], value)
	}
	binary.LittleEndian.PutUint32(data[512+12:], 0x1040)
	binary.LittleEndian.PutUint32(data[512+16:], 0x1080)
	copy(data[512+64:], "DevIL.dll\x00")
	dir := t.TempDir()
	path := filepath.Join(dir, "avisynth.dll")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := avisynthSupport(path); err == nil || !strings.Contains(err.Error(), "DevIL.dll is missing") {
		t.Fatal("packed runtime's required import was missed", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "DevIL.dll"), data, 0600); err != nil {
		t.Fatal(err)
	}
	support, err := avisynthSupport(path)
	if err != nil || len(support) != 1 || support[0] != filepath.Join(dir, "DevIL.dll") {
		t.Fatal("bundled DevIL was not resolved beside AviSynth", support, err)
	}
}
