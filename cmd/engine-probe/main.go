// engine-probe is a Stage 01 executable, not the production tool adapter.
// It operates only on owned fixtures and local artifacts, without network or DB.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

type fixture struct {
	File   string `json:"file"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	DPI    int    `json:"dpi"`
	Filter string `json:"filter"`
	Bits   int    `json:"bits"`
	SHA256 string `json:"sha256"`
}
type imageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}
type observation struct {
	DecodedImageEqual *bool       `json:"decoded_image_equal,omitempty"`
	Fixture           string      `json:"fixture"`
	Candidate         string      `json:"candidate"`
	Status            string      `json:"status"`
	InputBytes        int64       `json:"input_bytes"`
	OutputBytes       int64       `json:"output_bytes"`
	Images            []imageSize `json:"images"`
	ExpectedPixels    int         `json:"expected_pixels"`
	TextEqual         bool        `json:"gs_text_equal"`
	GeometryEqual     bool        `json:"geometry_equal"`
	FeaturesEqual     bool        `json:"links_bookmarks_equal"`
	PopplerTextEqual  *bool       `json:"poppler_text_equal,omitempty"`
	RenderEqual       *bool       `json:"independent_render_equal,omitempty"`
	Problem           string      `json:"problem,omitempty"`
}
type invocation struct {
	Tool   string   `json:"tool"`
	Args   []string `json:"args"`
	Exit   int      `json:"exit"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
}
type report struct {
	Status           string            `json:"status"`
	Versions         map[string]string `json:"versions"`
	QA               bool              `json:"qa"`
	SAFERDenied      bool              `json:"safer_denied"`
	JBIG2MatchesMono *bool             `json:"jbig2_matches_mono,omitempty"`
	Observations     []observation     `json:"observations"`
	Commands         []invocation      `json:"commands"`
	Problems         []string          `json:"problems,omitempty"`
}

var result = report{Status: "Pass", Versions: map[string]string{}}
var outDir string

func main() {
	fixturesDir := flag.String("fixtures", "/fixtures", "owned synthetic fixtures")
	output := flag.String("out", "/work/jobs/stage-01", "new artifact directory (must not exist)")
	qa := flag.Bool("qa", true, "require independent Poppler checks")
	flag.Parse()
	var err error
	outDir, err = filepath.Abs(*output)
	if err != nil {
		fatal(err)
	}
	if _, err = os.Stat(outDir); !os.IsNotExist(err) {
		fatal(fmt.Errorf("output directory must not exist: %s", outDir))
	}
	if err = os.MkdirAll(outDir, 0700); err != nil {
		fatal(err)
	}
	result.QA = *qa
	// Record a report even if the version gate or a later check fails.
	runErr := probe(*fixturesDir, *qa)
	if runErr != nil {
		result.Problems = append(result.Problems, runErr.Error())
		result.Status = "Fail"
	}
	for _, row := range result.Observations {
		if row.Status == "Fail" {
			result.Status = "Fail"
		}
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	if err = os.WriteFile(filepath.Join(outDir, "report.json"), append(data, '\n'), 0600); err != nil {
		fatal(err)
	}
	fmt.Printf("Stage 01: %s; %d observations; report %s/report.json\n", result.Status, len(result.Observations), outDir)
	if result.Status != "Pass" {
		os.Exit(1)
	}
}
func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

type probeRunner struct {
	underlying worker.Runner
}

func (p *probeRunner) Run(ctx context.Context, tool string, args ...string) (*worker.Result, error) {
	return p.RunWithOptions(ctx, worker.CommandOptions{}, tool, args...)
}

func (p *probeRunner) RunWithOptions(
	ctx context.Context,
	opts worker.CommandOptions,
	tool string,
	args ...string,
) (*worker.Result, error) {
	res, err := p.underlying.RunWithOptions(ctx, opts, tool, args...)
	code := 0
	var stdout_str, stderr_str string
	if res != nil {
		code = res.ExitCode
		stdout_str = string(res.Stdout)
		stderr_str = string(res.Stderr)
	} else if err != nil {
		code = -1
	}

	result.Commands = append(result.Commands, invocation{
		Tool:   tool,
		Args:   args,
		Exit:   code,
		Stdout: stdout_str,
		Stderr: stderr_str,
	})

	return res, err
}

var pRunner = &probeRunner{underlying: worker.NewLinuxRunner(true)}
var qpdfClient = qpdf.NewClient("qpdf", pRunner)
var gsClient = ghostscript.NewClient("gs", pRunner)

func command(tool string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := worker.CommandOptions{
		Dir: "/",
	}

	res, err := pRunner.RunWithOptions(ctx, opts, tool, args...)
	if res == nil {
		return nil, err
	}
	if err != nil {
		return res.Stdout, fmt.Errorf("%s exit %d: %s", tool, res.ExitCode, string(res.Stderr))
	}
	if len(res.Stderr) > 0 {
		return res.Stdout, fmt.Errorf("%s diagnostic: %s", tool, string(res.Stderr))
	}
	return res.Stdout, nil
}
func probe(dir string, qa bool) error {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.GoVersion != "go1.27.2" {
		return fmt.Errorf("probe requires Go 1.27.2 build")
	}
	result.Versions["go"] = info.GoVersion
	for _, tool := range []struct{ name, want string }{{"gs", "10.08.0"}, {"qpdf", "12.4.2"}} {
		data, err := command(tool.name, "--version")
		if err != nil {
			return err
		}
		result.Versions[tool.name] = strings.TrimSpace(string(data))
		if !strings.Contains(string(data), tool.want) {
			return fmt.Errorf("%s version mismatch", tool.name)
		}
	}
	if qa {
		// Poppler prints its version/copyright to stderr as normal version output.
		cmd := exec.Command("pdfinfo", "-v")
		data, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		result.Versions["poppler"] = strings.TrimSpace(string(data))
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return err
	}
	var fixtures []fixture
	if err = json.Unmarshal(manifest, &fixtures); err != nil {
		return err
	}
	if len(fixtures) != 8 {
		return fmt.Errorf("expected eight positive fixtures")
	}
	for _, f := range fixtures {
		if filepath.Base(f.File) != f.File {
			return fmt.Errorf("invalid fixture name")
		}
		data, err := os.ReadFile(filepath.Join(dir, f.File))
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != f.SHA256 {
			return fmt.Errorf("fixture hash mismatch: %s", f.File)
		}
		job := filepath.Join(outDir, strings.TrimSuffix(f.File, ".pdf"))
		if err = os.Mkdir(job, 0700); err != nil {
			return err
		}
		input := filepath.Join(job, "input.pdf")
		if err = os.WriteFile(input, data, 0600); err != nil {
			return err
		}
		ctx_input_check, cancel_input_check := context.WithTimeout(context.Background(), 60*time.Second)
		err = qpdfClient.Check(ctx_input_check, input)
		cancel_input_check()
		if err != nil {
			return err
		}
		original, err := snapshot(input)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(original.Images, []imageSize{{f.Width, f.Height}}) || original.Links != 1 || original.Bookmarks != 1 {
			return fmt.Errorf("fixture inventory invalid: %s", f.File)
		}
		originalText, err := text(input, filepath.Join(job, "input.txt"))
		if err != nil {
			return err
		}
		if !strings.Contains(originalText, "Stage 01 probe") {
			return fmt.Errorf("fixture text unavailable: %s", f.File)
		}
		for _, candidate := range []string{"lossless", "balanced", "compact"} {
			row := observation{Fixture: f.File, Candidate: candidate, Status: "Pass", InputBytes: int64(len(data)), ExpectedPixels: f.Width}
			if candidate == "balanced" && f.Bits != 1 && f.DPI > 300 {
				row.ExpectedPixels = 600
			} else if candidate == "compact" && f.Bits != 1 && f.DPI > 150 {
				row.ExpectedPixels = 300
			}
			output := filepath.Join(job, candidate+".pdf")
			err = process(candidate, input, output)
			if err == nil {
				ctx_out_check, cancel_out_check := context.WithTimeout(context.Background(), 60*time.Second)
				err = qpdfClient.Check(ctx_out_check, output)
				cancel_out_check()
			}
			if err == nil {
				stat, e := os.Stat(output)
				err = e
				if e == nil {
					row.OutputBytes = stat.Size()
				}
			}
			if err == nil {
				current, e := snapshot(output)
				err = e
				if e == nil {
					row.Images = current.Images
					row.GeometryEqual = reflect.DeepEqual(original.Geometry, current.Geometry)
					row.FeaturesEqual = original.Links == current.Links && original.Bookmarks == current.Bookmarks && reflect.DeepEqual(original.Features, current.Features)
					if !reflect.DeepEqual(current.Images, []imageSize{{row.ExpectedPixels, row.ExpectedPixels}}) {
						err = fmt.Errorf("unexpected image dimensions: %+v", current.Images)
					}
					if !row.GeometryEqual {
						err = fmt.Errorf("geometry changed")
					}
				}
			}
			if err == nil {
				extracted, e := text(output, filepath.Join(job, candidate+".txt"))
				err = e
				row.TextEqual = extracted == originalText
				if e == nil && !row.TextEqual {
					err = fmt.Errorf("txtwrite text changed")
				}
			}
			if err == nil && qa {
				err = independent(input, output, job, candidate, f.Bits == 1, &row)
			}
			if err != nil {
				row.Status = "Fail"
				row.Problem = err.Error()
			} else if row.RenderEqual != nil && !*row.RenderEqual {
				row.Status = "RequiresVisualFollowup"
				row.Problem = "Decoded mono pixels match; independent page-crop render differs. Stage 07 release QA must resolve the cause."
			} else if !row.FeaturesEqual {
				row.Status = "RequiresSemanticFallback"
				row.Problem = "Stage 05 must reject this GS candidate and use lossless/original"
			}
			result.Observations = append(result.Observations, row)
		}
	}
	// A trusted permit path must not accidentally grant adjacent file access.
	denied := filepath.Join(outDir, "not-permitted.txt")
	if err = os.WriteFile(denied, []byte("synthetic forbidden marker"), 0600); err != nil {
		return err
	}
	_, err = command("gs", "-q", "-dBATCH", "-dNOPAUSE", "-dSAFER", "-sDEVICE=nullpage", "-c", fmt.Sprintf("(%s) (r) file closefile", denied))
	last := result.Commands[len(result.Commands)-1]
	result.SAFERDenied = err != nil && strings.Contains(last.Stdout+last.Stderr, "invalidfileaccess")
	if !result.SAFERDenied {
		return fmt.Errorf("SAFER adjacent-file read denial not proven")
	}
	_, err = command("gs", "-q", "-dBATCH", "-dNOPAUSE", "-dSAFER", "-sDEVICE=nullpage", "-c", fmt.Sprintf("(%s) (w) file closefile", filepath.Join(outDir, "not-permitted-output.txt")))
	last = result.Commands[len(result.Commands)-1]
	result.SAFERDenied = err != nil && strings.Contains(last.Stdout+last.Stderr, "invalidfileaccess")
	if !result.SAFERDenied {
		return fmt.Errorf("SAFER adjacent-file write denial not proven")
	}
	if qa {
		a, err := os.ReadFile(filepath.Join(outDir, "mono-flate-600dpi", "source-crop.ppm"))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(outDir, "jbig2-600dpi", "source-crop.ppm"))
		if err != nil {
			return err
		}
		same := bytes.Equal(a, b)
		result.JBIG2MatchesMono = &same
		if !same {
			return fmt.Errorf("JBIG2 decoded pixels differ from owned mono fixture")
		}
	}
	return nil
}

// Production profile execution implemented in Stage 04.
func process(candidate, input, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if candidate == "lossless" {
		return qpdfClient.WriteLossless(ctx, input, output)
	}

	profile := ghostscript.ProfileName(candidate)
	_, err := gsClient.Rewrite(ctx, input, output, ghostscript.RewriteOptions{
		Profile: profile,
	})
	return err
}

func text(input, output string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return gsClient.ExtractText(ctx, input, output)
}

func normalize(s string) string {
	return ghostscript.NormalizeText(s)
}

type document struct {
	Images    []imageSize
	Geometry  []string
	Links     int
	Bookmarks int
	Features  []string
}

func snapshot(input string) (document, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	data, parsed, err := qpdfClient.AnalyzeRaw(ctx, input)
	if err != nil {
		return document{}, err
	}
	if err = os.WriteFile(strings.TrimSuffix(input, ".pdf")+".json", data, 0600); err != nil {
		return document{}, err
	}
	if len(parsed.Pages) != 1 {
		return document{}, fmt.Errorf("page count changed")
	}
	doc := document{}
	for _, img := range parsed.Pages[0].Images {
		doc.Images = append(doc.Images, imageSize{
			Width:  img.Width,
			Height: img.Height,
		})
	}
	for _, section := range parsed.QPDF {
		for _, raw := range section {
			var entry qpdf.ObjectEntry
			if json.Unmarshal(raw, &entry) != nil {
				continue
			}
			dict := entry.GetDict()
			if dict == nil {
				continue
			}
			var kind string
			_ = json.Unmarshal(dict["/Type"], &kind)
			if kind == "/Page" {
				for _, key := range []string{"/MediaBox", "/CropBox", "/Rotate", "/UserUnit"} {
					// qpdf numbers are canonical here; fixture geometry is deliberately simple.
					v := dict[key]
					if len(v) == 0 {
						if key == "/CropBox" {
							v = dict["/MediaBox"]
						} else if key == "/Rotate" {
							v = []byte("0")
						} else if key == "/UserUnit" {
							v = []byte("1")
						}
					}
					var compact bytes.Buffer
					_ = json.Compact(&compact, v)
					doc.Geometry = append(doc.Geometry, key+":"+compact.String())
				}
			}
			var subtype string
			_ = json.Unmarshal(dict["/Subtype"], &subtype)
			if subtype == "/Link" {
				doc.Links++
				var action map[string]string
				_ = json.Unmarshal(dict["/A"], &action)
				var rect bytes.Buffer
				_ = json.Compact(&rect, dict["/Rect"])
				doc.Features = append(doc.Features, "link:"+action["/S"]+":"+action["/URI"]+":"+rect.String())
			}
			var title string
			_ = json.Unmarshal(dict["/Title"], &title)
			if title == "u:Probe bookmark" {
				doc.Bookmarks++
				var dest []string
				// /Dest is [page reference, /Fit] in the owned fixture.
				if json.Unmarshal(dict["/Dest"], &dest) != nil || len(dest) != 2 || dest[0] != parsed.Pages[0].Object {
					doc.Features = append(doc.Features, "bookmark-destination-changed")
					continue
				}
				doc.Features = append(doc.Features, "bookmark:"+title+":"+dest[1])
			}
		}
	}
	sort.Strings(doc.Geometry)
	sort.Strings(doc.Features)
	return doc, nil
}
func independent(input, output, job, candidate string, mono bool, row *observation) error {
	a, err := command("pdftotext", "-layout", input, "-")
	if err != nil {
		return err
	}
	b, err := command("pdftotext", "-layout", output, "-")
	if err != nil {
		return err
	}
	same := normalize(string(a)) == normalize(string(b))
	row.PopplerTextEqual = &same
	if !same {
		return fmt.Errorf("Poppler text changed")
	}
	// Save full-page independent render pairs for review of lossy candidates.
	for _, item := range []struct{ file, prefix string }{{input, "source"}, {output, candidate}} {
		prefix := filepath.Join(job, item.prefix)
		if _, err = command("pdftoppm", "-r", "72", "-singlefile", "-png", item.file, prefix); err != nil {
			return err
		}
		if _, err = command("pdfinfo", item.file); err != nil {
			return err
		}
		if _, err = command("pdffonts", item.file); err != nil {
			return err
		}
		if _, err = command("pdfimages", "-list", item.file); err != nil {
			return err
		}
		if candidate == "lossless" || mono {
			args := []string{"-r", "600", "-singlefile"}
			suffix := "-full"
			if mono {
				args = append(args, "-x", "300", "-y", "300", "-W", "1200", "-H", "1200")
				suffix = "-crop"
			}
			args = append(args, item.file, prefix+suffix)
			if _, err = command("pdftoppm", args...); err != nil {
				return err
			}
		}
	}
	if candidate == "lossless" || mono {
		suffix := "-full.ppm"
		if mono {
			suffix = "-crop.ppm"
		}
		a, err = os.ReadFile(filepath.Join(job, "source"+suffix))
		if err != nil {
			return err
		}
		b, err = os.ReadFile(filepath.Join(job, candidate+suffix))
		if err != nil {
			return err
		}
		equal := bytes.Equal(a, b)
		row.RenderEqual = &equal
		if !equal && candidate == "lossless" {
			return fmt.Errorf("lossless independent render pixels changed")
		}
		if mono {
			for _, item := range []struct{ file, prefix string }{{input, "source-image"}, {output, candidate + "-image"}} {
				if _, err = command("pdfimages", "-png", item.file, filepath.Join(job, item.prefix)); err != nil {
					return err
				}
			}
			same, err := decodedEqual(filepath.Join(job, "source-image-000.png"), filepath.Join(job, candidate+"-image-000.png"))
			if err != nil {
				return err
			}
			row.DecodedImageEqual = &same
			if !same {
				return fmt.Errorf("decoded mono image pixels changed")
			}
		}
	}
	return nil
}

// QA only: compare independently decoded pixels; never resize or edit a PDF.
func decodedEqual(a, b string) (bool, error) {
	af, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer af.Close()
	bf, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer bf.Close()
	ai, err := png.Decode(af)
	if err != nil {
		return false, err
	}
	bi, err := png.Decode(bf)
	if err != nil {
		return false, err
	}
	if ai.Bounds() != bi.Bounds() {
		return false, nil
	}
	for y := ai.Bounds().Min.Y; y < ai.Bounds().Max.Y; y++ {
		for x := ai.Bounds().Min.X; x < ai.Bounds().Max.X; x++ {
			ar, ag, ab, aa := ai.At(x, y).RGBA()
			br, bg, bb, ba := bi.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false, nil
			}
		}
	}
	return true, nil
}
