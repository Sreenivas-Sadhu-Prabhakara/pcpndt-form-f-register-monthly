// Command pcpndt-form-f-register-monthly keeps the Form F register and the
// rule 9(1) register that an ultrasound clinic or imaging centre registered
// under the Pre-conception and Pre-natal Diagnostic Techniques (Prohibition of
// Sex Selection) Act, 1994 has to maintain, checks every entry against the
// prescribed field set, and assembles the monthly report that rule 9(8)
// requires the centre to send to its Appropriate Authority.
//
// It serves a small interface on the loopback address and keeps everything in
// one file on the same machine. It makes no outbound network calls of any kind.
// Nothing is transmitted to any authority, nothing is uploaded, and no part of
// the register leaves the computer it runs on. Patient data must not leave the
// clinic, and this program gives it no way to.
//
// It files nothing. The report has to be lodged with the Appropriate Authority
// in the manner that office requires.
package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sreenivas-sadhu-prabhakara/pcpndt-form-f-register-monthly/internal/formf"
)

//go:embed web
var webFiles embed.FS

// fileVersion is the schema version written into the data file, so that a
// later release can recognise and migrate an older one.
const fileVersion = 1

// maxBodyBytes caps a request body. One entry is a couple of kilobytes.
const maxBodyBytes = 256 << 10

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to serve the register interface on")
	data := flag.String("data", "./pcpndt-form-f-register-monthly-data.json", "path to the register data file")
	flag.Parse()

	st, err := openStore(*data)
	if err != nil {
		log.Fatalf("cannot open the register file %s: %v", *data, err)
	}

	abs, err := filepath.Abs(*data)
	if err != nil {
		abs = *data
	}
	log.Printf("Form F register and monthly report")
	log.Printf("register file : %s (%d entries)", abs, st.entryCount())
	log.Printf("open http://%s/ in a browser", *addr)
	log.Printf("nothing is sent anywhere else: this program makes no outbound connections")
	log.Printf("it records no detail of what the Act prohibits, in any field")

	srv := &http.Server{
		Addr:              *addr,
		Handler:           routes(st),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------- data file

// fileData is exactly what the data file holds.
type fileData struct {
	Version  int                    `json:"version"`
	Clinic   formf.Clinic           `json:"clinic"`
	Settings formf.Settings         `json:"settings"`
	Entries  []formf.Record         `json:"entries"`
	Tallies  []formf.ProcedureTally `json:"tallies"`
}

type store struct {
	mu   sync.Mutex
	path string
	data fileData
}

func openStore(path string) (*store, error) {
	s := &store{path: path, data: fileData{Version: fileVersion, Settings: formf.DefaultSettings()}}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return s, nil
	}
	var loaded fileData
	if err := json.Unmarshal(b, &loaded); err != nil {
		return nil, fmt.Errorf("%s cannot be read: it is not valid JSON (%w)", path, err)
	}
	// A file that parses as JSON but carries no version was not written by
	// this program. Treating it as an empty register would mean overwriting
	// it on the first save, and whatever it is, it is not ours to destroy.
	if loaded.Version <= 0 {
		return nil, fmt.Errorf("%s is JSON but was not written by this program: it carries no register version. Point -data at the register file, or move that file aside first", path)
	}
	if loaded.Version > fileVersion {
		return nil, fmt.Errorf("the register file was written by a later version of this program (file version %d, this program understands %d)", loaded.Version, fileVersion)
	}
	loaded.Version = fileVersion
	loaded.Settings = loaded.Settings.Normalised()
	s.data = loaded
	return s, nil
}

// save writes the file atomically: a temporary file in the same directory,
// flushed, then renamed over the original. A register must not be left
// half-written by a power cut.
func (s *store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".register-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

func (s *store) entryCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data.Entries)
}

// snapshot returns a copy of everything, for a read that needs no lock held.
func (s *store) snapshot() fileData {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.data
	out.Entries = append([]formf.Record(nil), s.data.Entries...)
	out.Tallies = append([]formf.ProcedureTally(nil), s.data.Tallies...)
	out.Settings = out.Settings.Normalised()
	return out
}

// ------------------------------------------------------------------- routes

func routes(st *store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, buildState(st.snapshot(), today()))
	})

	mux.HandleFunc("POST /api/clinic", func(w http.ResponseWriter, r *http.Request) {
		var c formf.Clinic
		if !readJSON(w, r, &c) {
			return
		}
		st.mu.Lock()
		st.data.Clinic = c
		err := st.save()
		st.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildState(st.snapshot(), today()))
	})

	mux.HandleFunc("POST /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var in formf.Settings
		if !readJSON(w, r, &in) {
			return
		}
		st.mu.Lock()
		st.data.Settings = in.Normalised()
		err := st.save()
		st.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildState(st.snapshot(), today()))
	})

	// A draft is checked without being stored, so that the whole shortfall
	// can be shown while the form is still open.
	mux.HandleFunc("POST /api/check", func(w http.ResponseWriter, r *http.Request) {
		var rec formf.Record
		if !readJSON(w, r, &rec) {
			return
		}
		writeJSON(w, http.StatusOK, formf.ValidateOn(rec, today()))
	})

	mux.HandleFunc("POST /api/entries", func(w http.ResponseWriter, r *http.Request) {
		var rec formf.Record
		if !readJSON(w, r, &rec) {
			return
		}
		st.mu.Lock()
		defer st.mu.Unlock()

		// The serial is the register's, not the caller's.
		rec.Serial = formf.NextSerial(st.data.Entries, st.data.Settings)

		v := formf.ValidateOn(rec, today())
		if !v.Storable() {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":      "This entry cannot be stored as written.",
				"validation": v,
			})
			return
		}
		st.data.Entries = append(st.data.Entries, rec)
		if err := st.save(); err != nil {
			st.data.Entries = st.data.Entries[:len(st.data.Entries)-1]
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"serial": rec.Serial, "validation": v})
	})

	// An entry can be corrected but never removed. A gap in the numbering
	// reads as a removed record, so there is no endpoint that makes one:
	// a mistake is corrected in place and the correction noted in remarks.
	mux.HandleFunc("POST /api/entries/update", func(w http.ResponseWriter, r *http.Request) {
		var rec formf.Record
		if !readJSON(w, r, &rec) {
			return
		}
		if rec.Serial <= 0 {
			writeError(w, http.StatusBadRequest, "an update has to name the serial number of the entry it corrects")
			return
		}
		v := formf.ValidateOn(rec, today())
		if !v.Storable() {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":      "This entry cannot be stored as written.",
				"validation": v,
			})
			return
		}
		st.mu.Lock()
		defer st.mu.Unlock()
		at := -1
		for i, e := range st.data.Entries {
			if e.Serial == rec.Serial {
				at = i
				break
			}
		}
		if at < 0 {
			writeError(w, http.StatusNotFound, fmt.Sprintf("there is no entry numbered %d in this register", rec.Serial))
			return
		}
		was := st.data.Entries[at]
		st.data.Entries[at] = rec
		if err := st.save(); err != nil {
			st.data.Entries[at] = was
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"serial": rec.Serial, "validation": v})
	})

	mux.HandleFunc("POST /api/tallies", func(w http.ResponseWriter, r *http.Request) {
		var in formf.ProcedureTally
		if !readJSON(w, r, &in) {
			return
		}
		if _, err := formf.ParseDay(in.Date); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if in.Count < 0 {
			writeError(w, http.StatusBadRequest, "a day's count cannot be negative")
			return
		}
		st.mu.Lock()
		defer st.mu.Unlock()
		replaced := false
		for i, t := range st.data.Tallies {
			if t.Date == in.Date {
				st.data.Tallies[i] = in
				replaced = true
				break
			}
		}
		if !replaced {
			st.data.Tallies = append(st.data.Tallies, in)
		}
		sort.SliceStable(st.data.Tallies, func(i, j int) bool { return st.data.Tallies[i].Date < st.data.Tallies[j].Date })
		if err := st.save(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"saved": in})
	})

	mux.HandleFunc("GET /api/register", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		writeJSON(w, http.StatusOK, formf.PrintableRegister(d.Clinic, d.Entries, d.Settings, today()))
	})

	mux.HandleFunc("GET /api/index", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		writeJSON(w, http.StatusOK, formf.BuildWorkingIndex(d.Clinic, d.Entries, d.Settings, today()))
	})

	mux.HandleFunc("GET /api/month", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		m, err := requestedMonth(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, formf.AssembleReturn(d.Clinic, d.Entries, d.Tallies, m, d.Settings, today()))
	})

	mux.HandleFunc("GET /api/entry", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		rec, ok := findEntry(d.Entries, r.URL.Query().Get("serial"))
		if !ok {
			writeError(w, http.StatusNotFound, "no such entry in this register")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"entry":      rec,
			"face":       formf.FormFace(rec),
			"validation": formf.ValidateOn(rec, today()),
			"retention":  formf.RetentionOn(rec, d.Settings, today()),
		})
	})

	// The printed copy rule 9(7) calls for, as plain text, so that a centre
	// can keep or print one without a browser in the way.
	mux.HandleFunc("GET /api/text/register", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		writeText(w, formf.RenderRegisterText(formf.PrintableRegister(d.Clinic, d.Entries, d.Settings, today())))
	})

	mux.HandleFunc("GET /api/text/month", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		m, err := requestedMonth(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeText(w, formf.RenderReturnText(formf.AssembleReturn(d.Clinic, d.Entries, d.Tallies, m, d.Settings, today())))
	})

	mux.HandleFunc("GET /api/text/entry", func(w http.ResponseWriter, r *http.Request) {
		d := st.snapshot()
		rec, ok := findEntry(d.Entries, r.URL.Query().Get("serial"))
		if !ok {
			writeError(w, http.StatusNotFound, "no such entry in this register")
			return
		}
		writeText(w, formf.RenderFormFaceText(d.Clinic, rec, d.Settings, today()))
	})

	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServerFS(ui))
	return mux
}

// ------------------------------------------------------------------- state

// state is everything the interface needs in one response: the register, the
// prescribed field set it builds its form from, and the standing statements it
// has to show.
type state struct {
	Clinic   formf.Clinic   `json:"clinic"`
	Settings formf.Settings `json:"settings"`
	Defaults formf.Settings `json:"defaults"`

	Entries []entryView            `json:"entries"`
	Tallies []formf.ProcedureTally `json:"tallies"`

	Serial    formf.SerialCheck      `json:"serial"`
	Retention formf.RetentionSummary `json:"retention"`
	NextSeral int                    `json:"nextSerial"`
	Today     string                 `json:"today"`
	Months    []string               `json:"months"`

	Fields      []formf.Field         `json:"fields"`
	Indications []formf.Indication    `json:"indications"`
	Procedures  []string              `json:"procedures"`
	MTPAnswers  []string              `json:"mtpAnswers"`
	NotStored   []formf.ItemNotStored `json:"itemsNotStored"`

	Policy       string            `json:"policy"`
	Consequences string            `json:"consequences"`
	References   map[string]string `json:"references"`
	Sample       int               `json:"inspectionSample"`
}

type entryView struct {
	Entry      formf.Record     `json:"entry"`
	Validation formf.Validation `json:"validation"`
	Retention  formf.Retention  `json:"retention"`
}

func buildState(d fileData, asOf time.Time) state {
	s := state{
		Clinic:       d.Clinic,
		Settings:     d.Settings,
		Defaults:     formf.DefaultSettings(),
		Tallies:      d.Tallies,
		Serial:       formf.CheckSerials(d.Entries, d.Settings),
		Retention:    formf.SummariseRetention(d.Entries, d.Settings, asOf),
		NextSeral:    formf.NextSerial(d.Entries, d.Settings),
		Today:        formf.FormatDay(asOf),
		Fields:       formf.Fields(),
		Indications:  formf.Indications(),
		Procedures:   formf.Procedures(),
		MTPAnswers:   formf.MTPAnswers(),
		NotStored:    formf.ItemsNotStored(),
		Policy:       formf.PolicyStatement,
		Consequences: formf.Consequences,
		Sample:       formf.InspectionSample(len(d.Entries)),
		References: map[string]string{
			"retention":        formf.RetentionReference,
			"proviso":          formf.RetentionProviso,
			"monthlyReport":    formf.ReturnDueReference,
			"reportFormat":     formf.ReturnFormatNote,
			"printedCopy":      formf.PrintedCopyReference,
			"inspectionSample": formf.InspectionSampleReference,
		},
	}

	ordered := append([]formf.Record(nil), d.Entries...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Serial > ordered[j].Serial })
	for _, r := range ordered {
		s.Entries = append(s.Entries, entryView{
			Entry:      r,
			Validation: formf.ValidateOn(r, asOf),
			Retention:  formf.RetentionOn(r, d.Settings, asOf),
		})
	}

	// Every month the register has something in, newest first, plus the
	// current one so that a report can always be started.
	seen := map[string]bool{formf.MonthOf(asOf).String(): true}
	for _, r := range d.Entries {
		if day, err := formf.ParseDay(r.ProcedureDate); err == nil {
			seen[formf.MonthOf(day).String()] = true
		}
	}
	for _, t := range d.Tallies {
		if day, err := formf.ParseDay(t.Date); err == nil {
			seen[formf.MonthOf(day).String()] = true
		}
	}
	for m := range seen {
		s.Months = append(s.Months, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(s.Months)))
	return s
}

// ------------------------------------------------------------------ helpers

// today is the reference day for every check that needs one. It is the local
// calendar day: a scan done this evening in Dharwad was done today.
func today() time.Time {
	n := time.Now()
	return formf.Day(n.Year(), n.Month(), n.Day())
}

func requestedMonth(r *http.Request) (formf.Month, error) {
	q := strings.TrimSpace(r.URL.Query().Get("m"))
	if q == "" {
		return formf.MonthOf(today()), nil
	}
	return formf.ParseMonth(q)
}

func findEntry(entries []formf.Record, serial string) (formf.Record, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(serial))
	if err != nil {
		return formf.Record{}, false
	}
	for _, e := range entries {
		if e.Serial == n {
			return e, true
		}
	}
	return formf.Record{}, false
}

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "the request body is not the JSON this endpoint expects: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("writing a response: %v", err)
	}
}

func writeText(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, s)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
