// AnimeUpscale Web Server
//
// A small HTTP server that wraps the au.exe CLI and serves a browser UI.
//   go run web/server.go            # uses the au.exe next to go.mod
//   go run web/server.go -addr :8080 -au ./au.exe
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- job model ----------

type jobStatus string

const (
	statusQueued  jobStatus = "queued"
	statusRunning jobStatus = "running"
	statusDone    jobStatus = "done"
	statusError   jobStatus = "error"
)

type job struct {
	ID         string    `json:"id"`
	Status     jobStatus `json:"status"`
	InputPath  string    `json:"-"`
	OutputPath string    `json:"-"`
	InputName  string    `json:"inputName"`
	OutputName string    `json:"outputName"`
	IsVideo    bool      `json:"isVideo"`
	StartedAt  time.Time `json:"startedAt"`
	EndedAt    time.Time `json:"endedAt"`
	ExitCode   int       `json:"exitCode"`
	Error      string    `json:"error,omitempty"`
	CmdLine    string    `json:"cmdLine"`

	mu     sync.Mutex
	log    []string
	subs   map[chan jobEvent]struct{}
	closed bool
}

type jobEvent struct {
	Type    string `json:"type"` // "log" | "status" | "ping"
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
	Time    string `json:"time"`
}

func newJob(id string) *job {
	return &job{
		ID:     id,
		Status: statusQueued,
		log:    []string{},
		subs:   map[chan jobEvent]struct{}{},
	}
}

func (j *job) appendLog(line string) {
	j.mu.Lock()
	j.log = append(j.log, line)
	evt := jobEvent{Type: "log", Message: line, Time: time.Now().Format("15:04:05.000")}
	subs := make([]chan jobEvent, 0, len(j.subs))
	for c := range j.subs {
		subs = append(subs, c)
	}
	j.mu.Unlock()
	for _, c := range subs {
		select {
		case c <- evt:
		default:
		}
	}
}

func (j *job) setStatus(s jobStatus, errMsg string) {
	j.mu.Lock()
	j.Status = s
	if errMsg != "" {
		j.Error = errMsg
	}
	evt := jobEvent{Type: "status", Status: string(s), Message: errMsg, Time: time.Now().Format("15:04:05.000")}
	if s == statusDone || s == statusError {
		j.EndedAt = time.Now()
		j.closed = true
	}
	subs := make([]chan jobEvent, 0, len(j.subs))
	for c := range j.subs {
		subs = append(subs, c)
	}
	j.mu.Unlock()
	for _, c := range subs {
		select {
		case c <- evt:
		default:
		}
	}
}

func (j *job) snapshot() (logs []string, status jobStatus, errMsg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]string, len(j.log))
	copy(out, j.log)
	return out, j.Status, j.Error
}

func (j *job) subscribe() chan jobEvent {
	ch := make(chan jobEvent, 64)
	j.mu.Lock()
	logs := make([]string, len(j.log))
	copy(logs, j.log)
	status, errMsg := j.Status, j.Error
	closed := j.closed
	if !closed {
		j.subs[ch] = struct{}{}
	}
	j.mu.Unlock()
	go func() {
		for _, line := range logs {
			ch <- jobEvent{Type: "log", Message: line, Time: time.Now().Format("15:04:05.000")}
		}
		ch <- jobEvent{Type: "status", Status: string(status), Message: errMsg, Time: time.Now().Format("15:04:05.000")}
		if closed {
			close(ch)
		}
	}()
	return ch
}

func (j *job) unsubscribe(ch chan jobEvent) {
	j.mu.Lock()
	if _, ok := j.subs[ch]; ok {
		delete(j.subs, ch)
		closed := j.closed
		j.mu.Unlock()
		if closed {
			// drain remaining events then close
			go func() {
				for range ch {
				}
			}()
			close(ch)
		}
		return
	}
	j.mu.Unlock()
}

// ---------- server state ----------

type server struct {
	addr     string
	auPath   string
	workDir  string
	webDir   string
	jobsDir  string
	auMu     sync.Mutex
	engines  []engineInfo
	enginesT time.Time

	mu   sync.RWMutex
	jobs map[string]*job
}

type engineInfo struct {
	Name     string `json:"name"`
	Avail    bool   `json:"available"`
	Note     string `json:"note"`
	Raw      string `json:"raw"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	auPath := flag.String("au", "", "path to au.exe (default: auto-detect next to go.mod)")
	workDir := flag.String("workdir", "", "project root (default: parent of web/)")
	flag.Parse()

	exe, _ := os.Executable()
	_ = exe // for future use

	root := *workDir
	if root == "" {
		// assume running from project root; fall back to parent of this file's dir
		if _, err := os.Stat("au.exe"); err == nil {
			root, _ = os.Getwd()
		} else if _, err := os.Stat(filepath.Join("..", "au.exe")); err == nil {
			root, _ = filepath.Abs("..")
		} else {
			root, _ = os.Getwd()
		}
	}

	au := *auPath
	if au == "" {
		candidates := []string{
			filepath.Join(root, "au.exe"),
			filepath.Join(root, "au"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				au = c
				break
			}
		}
		if au == "" {
			// rely on PATH
			au = "au.exe"
			if runtime.GOOS != "windows" {
				au = "au"
			}
		}
	}

	webDir := filepath.Join(root, "web")
	if _, err := os.Stat(webDir); err != nil {
		// maybe we're inside web/ already
		if _, err2 := os.Stat("index.html"); err2 == nil {
			webDir, _ = os.Getwd()
			if root == "" {
				root = filepath.Dir(webDir)
			}
		} else {
			fmt.Fprintf(os.Stderr, "web dir not found (looked in %s and current dir)\n", webDir)
			os.Exit(1)
		}
	}

	jobsDir := filepath.Join(os.TempDir(), "animeupscale-web", "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir jobs: %v\n", err)
		os.Exit(1)
	}

	s := &server{
		addr:    *addr,
		auPath:  au,
		workDir: root,
		webDir:  webDir,
		jobsDir: jobsDir,
		jobs:    map[string]*job{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.routeIndex)
	mux.HandleFunc("/api/engines", s.routeEngines)
	mux.HandleFunc("/api/upscale", s.routeUpscale)
	mux.HandleFunc("/api/jobs/", s.routeJobs)
	mux.HandleFunc("/api/preview/", s.routePreview)
	mux.HandleFunc("/api/output/", s.routeOutput)

	// periodic cleanup of old jobs (1 hour)
	go s.cleanupLoop()

	fmt.Printf("AnimeUpscale web UI\n")
	fmt.Printf("  project root: %s\n", root)
	fmt.Printf("  au binary:    %s\n", au)
	fmt.Printf("  web dir:      %s\n", webDir)
	fmt.Printf("  jobs dir:     %s\n", jobsDir)
	fmt.Printf("  listening on: http://%s\n", *addr)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

// ---------- routes ----------

func (s *server) routeIndex(w http.ResponseWriter, r *http.Request) {
	// serve files from webDir (or fall back to index.html for SPA-style routes)
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if rel == "" {
		rel = "index.html"
	}
	full := filepath.Join(s.webDir, filepath.FromSlash(rel))
	// path containment
	clean := filepath.Clean(full)
	if !strings.HasPrefix(clean, filepath.Clean(s.webDir)) {
		http.NotFound(w, r)
		return
	}
	if fi, err := os.Stat(clean); err == nil && !fi.IsDir() {
		mimeType := mime.TypeByExtension(filepath.Ext(clean))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, clean)
		return
	}
	// fall back to index.html
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
}

func (s *server) routeEngines(w http.ResponseWriter, r *http.Request) {
	infos, err := s.detectEngines()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"engines": infos})
}

func (s *server) routeUpscale(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil { // 64 MiB
		http.Error(w, "parse form: "+err.Error(), 400)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file: "+err.Error(), 400)
		return
	}
	defer file.Close()

	id := newID()
	jobDir := filepath.Join(s.jobsDir, id)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		http.Error(w, "mkdir job: "+err.Error(), 500)
		return
	}

	inputName := filepath.Base(header.Filename)
	inputName = sanitizeFilename(inputName)
	inPath := filepath.Join(jobDir, inputName)
	out, err := os.Create(inPath)
	if err != nil {
		http.Error(w, "create input: "+err.Error(), 500)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		http.Error(w, "write input: "+err.Error(), 500)
		return
	}
	out.Close()

	args, err := buildArgs(r, inPath, jobDir)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	j := newJob(id)
	j.InputPath = inPath
	j.InputName = inputName
	j.IsVideo = args.video
	outExt := ".png"
	if args.video {
		outExt = ".mp4"
	} else if strings.EqualFold(filepath.Ext(inputName), ".psd") {
		outExt = ".psd"
	}
	j.OutputName = strings.TrimSuffix(inputName, filepath.Ext(inputName)) + "-4x" + outExt
	j.OutputPath = filepath.Join(jobDir, j.OutputName)
	j.CmdLine = fmt.Sprintf("%s %s", s.auPath, strings.Join(args.toCLI(), " "))

	s.mu.Lock()
	s.jobs[id] = j
	s.mu.Unlock()

	go s.runJob(j, args, j.OutputPath)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"inputName":  j.InputName,
		"outputName": j.OutputName,
		"cmdLine":    j.CmdLine,
		"isVideo":    j.IsVideo,
	})
}

func (s *server) routeJobs(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	id, sub := parts[0], parts[1]
	s.mu.RLock()
	j, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch sub {
	case "events":
		s.streamJobEvents(w, r, j)
	case "status":
		w.Header().Set("Content-Type", "application/json")
		logs, status, errMsg := j.snapshot()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         j.ID,
			"status":     status,
			"error":      errMsg,
			"exitCode":   j.ExitCode,
			"cmdLine":    j.CmdLine,
			"inputName":  j.InputName,
			"outputName": j.OutputName,
			"isVideo":    j.IsVideo,
			"logs":       logs,
		})
	default:
		http.NotFound(w, r)
	}
}

func (s *server) routePreview(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/preview/")
	if id == "" || strings.ContainsAny(id, "/\\") {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	j, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if j.InputPath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	mimeType := mime.TypeByExtension(filepath.Ext(j.InputPath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, j.InputPath)
}

func (s *server) routeOutput(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/output/")
	if id == "" || strings.ContainsAny(id, "/\\") {
		http.NotFound(w, r)
		return
	}
	s.mu.RLock()
	j, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if j.OutputPath == "" {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(j.OutputPath); err != nil {
		http.Error(w, "output not ready: "+err.Error(), 404)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, j.OutputName))
	mimeType := mime.TypeByExtension(filepath.Ext(j.OutputPath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, j.OutputPath)
}

// ---------- SSE ----------

func (s *server) streamJobEvents(w http.ResponseWriter, r *http.Request, j *job) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := j.subscribe()
	defer j.unsubscribe(ch)

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			fmt.Fprint(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case evt, open := <-ch:
			if !open {
				fmt.Fprintf(w, "event: end\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			payload, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, payload)
			flusher.Flush()
			if evt.Type == "status" && (evt.Status == string(statusDone) || evt.Status == string(statusError)) {
				// keep connection open briefly for any straggler events
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// ---------- engine detection ----------

var engineLineRE = regexp.MustCompile(`^(\S+)\s+(available|unavailable)\s*(?:\((.*)\))?`)

func (s *server) detectEngines() ([]engineInfo, error) {
	s.auMu.Lock()
	defer s.auMu.Unlock()
	if time.Since(s.enginesT) < 30*time.Second && len(s.engines) > 0 {
		return s.engines, nil
	}
	cmd := exec.Command(s.auPath, "-list-engines")
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list-engines: %w", err)
	}
	infos := []engineInfo{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := engineLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		infos = append(infos, engineInfo{
			Name:     m[1],
			Avail:    m[2] == "available",
			Note:     strings.TrimSpace(m[3]),
			Raw:      line,
		})
	}
	s.engines = infos
	s.enginesT = time.Now()
	return infos, nil
}

// ---------- arg building ----------

type runArgs struct {
	video        bool
	engine       string
	scale        int
	target       string
	noise        int
	gpu          string
	modelName    string
	threads      string
	tileSize     int
	sharpen      float64
	grayscale    bool
	videoWorkers int
	tta          bool
	layerMode    string
	inputPath    string
	outputPath   string
}

func (a runArgs) toCLI() []string {
	args := []string{}
	if a.video {
		args = append(args, "-video")
	}
	args = append(args, "-i", a.inputPath)
	args = append(args, "-o", a.outputPath)
	if a.engine != "" {
		args = append(args, "-engine", a.engine)
	}
	if a.scale > 0 {
		args = append(args, "-scale", strconv.Itoa(a.scale))
	}
	if a.target != "" {
		args = append(args, "-target", a.target)
	}
	if a.noise > 0 {
		args = append(args, "-noise", strconv.Itoa(a.noise))
	}
	if a.gpu != "" {
		args = append(args, "-gpu", a.gpu)
	}
	if a.modelName != "" {
		args = append(args, "-model-name", a.modelName)
	}
	if a.threads != "" {
		args = append(args, "-threads", a.threads)
	}
	if a.tileSize > 0 {
		args = append(args, "-tile-size", strconv.Itoa(a.tileSize))
	}
	if a.sharpen > 0 {
		args = append(args, "-sharpen", strconv.FormatFloat(a.sharpen, 'f', -1, 64))
	}
	if a.grayscale {
		args = append(args, "-grayscale")
	}
	if a.videoWorkers > 1 {
		args = append(args, "-video-workers", strconv.Itoa(a.videoWorkers))
	}
	if a.tta {
		args = append(args, "-tta")
	}
	if a.layerMode != "" {
		args = append(args, "-layers", a.layerMode)
	}
	return args
}

func buildArgs(r *http.Request, inPath, jobDir string) (*runArgs, error) {
	get := func(k, def string) string {
		if v := strings.TrimSpace(r.FormValue(k)); v != "" {
			return v
		}
		return def
	}
	getInt := func(k string, def int) int {
		if v := strings.TrimSpace(r.FormValue(k)); v != "" {
			n, err := strconv.Atoi(v)
			if err == nil {
				return n
			}
		}
		return def
	}
	getFloat := func(k string, def float64) float64 {
		if v := strings.TrimSpace(r.FormValue(k)); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return f
			}
		}
		return def
	}
	getBool := func(k string) bool {
		v := strings.ToLower(strings.TrimSpace(r.FormValue(k)))
		return v == "1" || v == "true" || v == "yes" || v == "on"
	}

	engine := get("engine", "auto")
	scale := getInt("scale", 2)
	target := get("target", "")
	gpu := get("gpu", "auto")

	// decide whether to use video mode based on input extension
	ext := strings.ToLower(filepath.Ext(inPath))
	isVideo := ext == ".mp4" || ext == ".mkv" || ext == ".mov" || ext == ".avi" || ext == ".webm"
	if get("mode", "") == "video" {
		isVideo = true
	} else if get("mode", "") == "image" {
		isVideo = false
	}

	if isVideo && engine == "builtin" {
		return nil, errors.New("builtin engine does not support video; choose another engine")
	}

	outExt := ".png"
	if isVideo {
		outExt = ".mp4"
	} else if strings.EqualFold(filepath.Ext(inPath), ".psd") {
		outExt = ".psd"
	}
	base := strings.TrimSuffix(filepath.Base(inPath), filepath.Ext(inPath))
	outPath := filepath.Join(jobDir, base+"-4x"+outExt)

	return &runArgs{
		video:        isVideo,
		engine:       engine,
		scale:        scale,
		target:       target,
		noise:        getInt("noise", 0),
		gpu:          gpu,
		modelName:    get("modelName", ""),
		threads:      get("threads", ""),
		tileSize:     getInt("tileSize", 0),
		sharpen:      getFloat("sharpen", 0),
		grayscale:    getBool("grayscale"),
		videoWorkers: getInt("videoWorkers", 1),
		tta:          getBool("tta"),
		layerMode:    get("layers", ""),
		inputPath:    inPath,
		outputPath:   outPath,
	}, nil
}

// ---------- job execution ----------

func (s *server) runJob(j *job, args *runArgs, outPath string) {
	j.setStatus(statusRunning, "")

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		j.appendLog("[server] mkdir: " + err.Error())
		j.setStatus(statusError, err.Error())
		return
	}

	cliArgs := args.toCLI()
	j.appendLog(fmt.Sprintf("[server] running: %s %s", s.auPath, strings.Join(cliArgs, " ")))

	cmd := exec.Command(s.auPath, cliArgs...)
	cmd.Dir = s.workDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		j.setStatus(statusError, "stdout pipe: "+err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		j.setStatus(statusError, "stderr pipe: "+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		j.setStatus(statusError, "start: "+err.Error())
		return
	}

	// pump both streams
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for s.Scan() {
			j.appendLog(s.Text())
		}
	}()
	go func() {
		defer wg.Done()
		s := bufio.NewScanner(stderr)
		s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for s.Scan() {
			j.appendLog("[err] " + s.Text())
		}
	}()
	wg.Wait()

	err = cmd.Wait()
	j.mu.Lock()
	j.ExitCode = cmd.ProcessState.ExitCode()
	j.mu.Unlock()

	if err != nil {
		// try to find any output that was produced (engine may have written to its own path)
		if alt, ok := findOutput(j.InputPath, filepath.Dir(outPath)); ok {
			j.OutputPath = alt
		}
		j.setStatus(statusError, fmt.Sprintf("exit %d: %v", j.ExitCode, err))
		return
	}

	// verify output exists; engines sometimes write to a slightly different filename
	if _, statErr := os.Stat(outPath); statErr != nil {
		if alt, ok := findOutput(j.InputPath, filepath.Dir(outPath)); ok {
			j.OutputPath = alt
		} else {
			j.setStatus(statusError, "output file not found")
			return
		}
	}
	j.setStatus(statusDone, "")
}

// findOutput looks in the job dir for any file that looks like an upscaled
// output (different from the input filename). Used as a fallback when the
// engine's actual output path differs from what we asked for.
func findOutput(inputPath, jobDir string) (string, bool) {
	entries, err := os.ReadDir(jobDir)
	if err != nil {
		return "", false
	}
	inputBase := strings.ToLower(filepath.Base(inputPath))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if name == inputBase {
			continue
		}
		return filepath.Join(jobDir, e.Name()), true
	}
	return "", false
}

// ---------- cleanup ----------

func (s *server) cleanupLoop() {
	t := time.NewTicker(15 * time.Minute)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		now := time.Now()
		for id, j := range s.jobs {
			if !j.EndedAt.IsZero() && now.Sub(j.EndedAt) > time.Hour {
				_ = os.RemoveAll(filepath.Join(s.jobsDir, id))
				delete(s.jobs, id)
			}
		}
		s.mu.Unlock()
	}
}

// ---------- helpers ----------

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		name = "input"
	}
	// strip path separators that may have slipped through
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "\x00", "")
	return name
}
