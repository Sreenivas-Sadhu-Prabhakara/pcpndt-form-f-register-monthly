package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/pcpndt-form-f-register-monthly/internal/formf"
)

// TestCatalogueKeysMatchTheRecordJSONKeys is what keeps the interface and the
// stored record from drifting apart. The form on screen is built from the field
// catalogue and posted as a record, so every catalogue key has to be a key of
// the record, and every key of the record has to be in the catalogue — a
// stored field that no page shows is a field nobody can correct.
func TestCatalogueKeysMatchTheRecordJSONKeys(t *testing.T) {
	recordKeys := map[string]bool{}
	ty := reflect.TypeOf(formf.Record{})
	for i := 0; i < ty.NumField(); i++ {
		tag := ty.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			t.Fatalf("Record.%s has no json name; the interface could not read or write it", ty.Field(i).Name)
		}
		recordKeys[name] = true
	}

	catalogueKeys := map[string]bool{}
	for _, f := range formf.Fields() {
		catalogueKeys[f.Key] = true
		if !recordKeys[f.Key] {
			t.Errorf("field %q is in the catalogue but is not a key of a stored record", f.Key)
		}
	}
	for k := range recordKeys {
		if !catalogueKeys[k] {
			t.Errorf("record key %q is stored but is in no field of the catalogue, so no page shows it", k)
		}
	}
	if len(recordKeys) != len(catalogueKeys) {
		t.Errorf("%d record keys against %d catalogue fields", len(recordKeys), len(catalogueKeys))
	}
}

// completeEntry builds an entry with every applicable item filled in, using
// only the package's exported surface — the same way the interface does.
func completeEntry() formf.Record {
	children := 1
	return formf.Record{
		FirstReportedOn:         "2026-11-16",
		PatientName:             "Shalini Kamath",
		Age:                     29,
		LivingChildren:          &children,
		ChildAges:               "4 y",
		RelativeName:            "Rohit Kamath",
		Address:                 "12 Gokhale Path, Dharwad 580001",
		Phone:                   "9876500000",
		ReferrerName:            "Dr A. Kulkarni",
		ReferrerAddress:         "Kulkarni Nursing Home, Dharwad",
		ReferrerQualificatn:     "MBBS, MS",
		ReferralPapersHeld:      true,
		LMP:                     "2026-06-30",
		GestationWeeks:          20,
		ConductedByName:         "Dr M. Rao",
		Indications:             []string{formf.Indications()[1].Text},
		IndicationDetail:        "Referral slip asks for a dating scan.",
		Procedures:              []string{formf.ProcUltrasound},
		WomanDeclarationDate:    "2026-11-18",
		ProcedureDate:           "2026-11-18",
		Result:                  "Single live intrauterine pregnancy, biometry at 20 weeks.",
		ConveyedTo:              "The patient, in person",
		ConveyedOn:              "2026-11-18",
		MTPIndication:           formf.MTPNone,
		SignatoryName:           "Dr M. Rao",
		SignatoryRegNo:          "KMC/33104",
		SealAffixed:             true,
		SignedOn:                "2026-11-18",
		Place:                   "Dharwad",
		DeclarationLanguage:     "Kannada",
		DoctorDeclarationSigned: true,
		DoctorDeclarationDate:   "2026-11-18",
		Equipment:               "USG-2",
	}
}

// issuesOf reads the issue list out of a decoded validation, treating the
// absent list of a complete entry as an empty one.
func issuesOf(body map[string]any) []any {
	v, ok := body["validation"].(map[string]any)
	if !ok {
		v = body
	}
	list, _ := v["issues"].([]any)
	return list
}

func TestCompleteEntryFixtureIsComplete(t *testing.T) {
	rec := completeEntry()
	rec.Serial = 1 // the register assigns this; the fixture leaves it to the server
	if v := formf.Validate(rec); !v.Complete() {
		for _, iss := range v.Issues {
			t.Logf("[%s] %s: %s", iss.Kind, iss.Field, iss.Message)
		}
		t.Fatalf("the fixture is not a complete entry: %s", v.Summary)
	}
}

// ------------------------------------------------------------------- store

func TestOpenStore(t *testing.T) {
	dir := t.TempDir()

	t.Run("a file that does not exist yet", func(t *testing.T) {
		st, err := openStore(filepath.Join(dir, "new.json"))
		if err != nil {
			t.Fatalf("openStore on a missing file: %v", err)
		}
		if st.entryCount() != 0 {
			t.Errorf("a new register holds %d entries", st.entryCount())
		}
		if got := st.snapshot().Settings; got != formf.DefaultSettings() {
			t.Errorf("a new register starts with settings %+v, want the defaults", got)
		}
	})

	t.Run("an empty file", func(t *testing.T) {
		p := filepath.Join(dir, "empty.json")
		if err := os.WriteFile(p, []byte("   \n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := openStore(p); err != nil {
			t.Errorf("openStore on an empty file: %v", err)
		}
	})

	t.Run("a file that is not JSON", func(t *testing.T) {
		p := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(p, []byte("not json at all"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := openStore(p); err == nil {
			t.Error("openStore accepted a file that is not JSON")
		}
	})

	// Pointing -data at somebody else's JSON file must not quietly produce
	// an empty register that then overwrites it on the first save.
	t.Run("JSON that this program did not write", func(t *testing.T) {
		p := filepath.Join(dir, "someone-elses.json")
		if err := os.WriteFile(p, []byte(`{"customers":[{"name":"x"}]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := openStore(p)
		if err == nil {
			t.Fatal("openStore accepted a JSON file that is not a register, and would have overwritten it")
		}
		if !strings.Contains(err.Error(), "not written by this program") {
			t.Errorf("the error does not explain why: %v", err)
		}
		// And the file is untouched.
		b, readErr := os.ReadFile(p)
		if readErr != nil || !strings.Contains(string(b), "customers") {
			t.Error("the other file was modified")
		}
	})

	t.Run("a file from a later version", func(t *testing.T) {
		p := filepath.Join(dir, "future.json")
		if err := os.WriteFile(p, []byte(`{"version":99,"entries":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := openStore(p)
		if err == nil {
			t.Fatal("openStore accepted a file written by a later version")
		}
		if !strings.Contains(err.Error(), "later version") {
			t.Errorf("the error does not explain why: %v", err)
		}
	})

	t.Run("figures outside a usable range are put back", func(t *testing.T) {
		p := filepath.Join(dir, "handedited.json")
		if err := os.WriteFile(p, []byte(`{"version":1,"settings":{"retentionYears":-4,"returnDueDay":99,"openingSerial":0},"entries":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		st, err := openStore(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := st.snapshot().Settings; got.RetentionYears != formf.RetentionYears || got.ReturnDueDay != formf.DefaultReturnDueDay || got.OpeningSerial != 1 {
			t.Errorf("a hand-edited file produced settings %+v", got)
		}
	})
}

func TestSaveIsAtomicAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "register.json")

	st, err := openStore(p)
	if err != nil {
		t.Fatal(err)
	}
	st.data.Clinic = formf.Clinic{Name: "Sahyadri Imaging Centre", RegistrationNo: "PCPNDT/DHW/2019/0147"}
	st.data.Entries = []formf.Record{completeEntry()}
	st.data.Entries[0].Serial = 1
	st.data.Tallies = []formf.ProcedureTally{{Date: "2026-11-18", Count: 1, Source: "OPD register"}}
	if err := st.save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// No temporary file is left behind, and the file is not world-readable:
	// it holds patient names and addresses.
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 {
		var left []string
		for _, n := range names {
			left = append(left, n.Name())
		}
		t.Errorf("the directory holds %v, want only the register file", left)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("the register file is mode %04o, want 0600", mode)
	}

	again, err := openStore(p)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	got := again.snapshot()
	if got.Clinic.Name != "Sahyadri Imaging Centre" {
		t.Errorf("the clinic did not survive the round trip: %+v", got.Clinic)
	}
	if len(got.Entries) != 1 || got.Entries[0].Serial != 1 || got.Entries[0].PatientName != "Shalini Kamath" {
		t.Errorf("the entry did not survive the round trip: %+v", got.Entries)
	}
	if got.Entries[0].LivingChildren == nil || *got.Entries[0].LivingChildren != 1 {
		t.Error("the living-children answer did not survive the round trip")
	}
	if len(got.Tallies) != 1 || got.Tallies[0].Count != 1 {
		t.Errorf("the day count did not survive the round trip: %+v", got.Tallies)
	}
	if !formf.Validate(got.Entries[0]).Complete() {
		t.Error("a complete entry came back from the file incomplete")
	}
}

// TestNilIsNotZeroInTheFile pins the one field where the difference matters: a
// woman with no living children and an item nobody has answered must not look
// the same in the file.
func TestNilIsNotZeroInTheFile(t *testing.T) {
	zero := 0
	for _, c := range []struct {
		name string
		in   *int
		want string
	}{
		{"unanswered", nil, "null"},
		{"none", &zero, "0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(formf.Record{LivingChildren: c.in})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), `"livingChildren":`+c.want) {
				t.Errorf("livingChildren serialised wrongly: %s", b)
			}
			var back formf.Record
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatal(err)
			}
			if (back.LivingChildren == nil) != (c.in == nil) {
				t.Error("the difference between nil and none was lost in the round trip")
			}
		})
	}
}

// ------------------------------------------------------------------ the API

type api struct {
	t   *testing.T
	srv *httptest.Server
}

func newAPI(t *testing.T) *api {
	t.Helper()
	st, err := openStore(filepath.Join(t.TempDir(), "register.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(routes(st))
	t.Cleanup(srv.Close)
	return &api{t: t, srv: srv}
}

func (a *api) get(path string) (int, []byte) {
	a.t.Helper()
	res, err := http.Get(a.srv.URL + path)
	if err != nil {
		a.t.Fatalf("GET %s: %v", path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func (a *api) post(path string, payload any) (int, map[string]any) {
	a.t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		a.t.Fatal(err)
	}
	res, err := http.Post(a.srv.URL+path, "application/json", strings.NewReader(string(body)))
	if err != nil {
		a.t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		a.t.Fatalf("POST %s returned %d with an unreadable body: %s", path, res.StatusCode, b)
	}
	return res.StatusCode, out
}

func TestAPIServesTheInterfaceAndTheState(t *testing.T) {
	a := newAPI(t)

	code, body := a.get("/")
	if code != http.StatusOK {
		t.Fatalf("GET / = %d", code)
	}
	for _, want := range []string{"<h1>", "PCPNDT Form F register", "assets/app.js"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the served page does not contain %q", want)
		}
	}
	if code, _ := a.get("/assets/app.js"); code != http.StatusOK {
		t.Errorf("GET /assets/app.js = %d", code)
	}

	code, b := a.get("/api/state")
	if code != http.StatusOK {
		t.Fatalf("GET /api/state = %d: %s", code, b)
	}
	var st state
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("reading the state: %v", err)
	}
	if len(st.Fields) != len(formf.Fields()) {
		t.Errorf("the state carries %d fields, want %d", len(st.Fields), len(formf.Fields()))
	}
	if len(st.Indications) != 24 {
		t.Errorf("the state carries %d indications, want 24", len(st.Indications))
	}
	if st.Policy != formf.PolicyStatement {
		t.Error("the state does not carry the policy statement, so the interface cannot show it")
	}
	if st.Consequences == "" || st.References["retention"] == "" || st.References["monthlyReport"] == "" {
		t.Error("the state is missing the statutory references the interface prints")
	}
	if st.NextSeral != 1 {
		t.Errorf("NextSerial = %d on an empty register, want 1", st.NextSeral)
	}
	if st.Today == "" {
		t.Error("the state carries no reference day")
	}
	if len(st.Months) == 0 {
		t.Error("the state offers no month to report on")
	}
	if len(st.NotStored) == 0 {
		t.Error("the state does not list what the register leaves out")
	}
}

func TestAPIEntryLifecycle(t *testing.T) {
	a := newAPI(t)

	t.Run("the register assigns the serial, not the caller", func(t *testing.T) {
		rec := completeEntry()
		rec.Serial = 9999
		code, body := a.post("/api/entries", rec)
		if code != http.StatusOK {
			t.Fatalf("POST /api/entries = %d: %v", code, body)
		}
		if got := body["serial"].(float64); got != 1 {
			t.Errorf("the entry was stored as serial %v, want 1 — a caller must not choose its own number", got)
		}
	})

	t.Run("the next entry takes the next number", func(t *testing.T) {
		code, body := a.post("/api/entries", completeEntry())
		if code != http.StatusOK {
			t.Fatalf("POST /api/entries = %d: %v", code, body)
		}
		if got := body["serial"].(float64); got != 2 {
			t.Errorf("serial = %v, want 2", got)
		}
	})

	t.Run("an incomplete entry is still stored, so it can be corrected", func(t *testing.T) {
		rec := completeEntry()
		rec.Place = ""
		code, body := a.post("/api/entries", rec)
		if code != http.StatusOK {
			t.Fatalf("POST /api/entries = %d: %v", code, body)
		}
		if body["serial"].(float64) != 3 {
			t.Errorf("serial = %v, want 3", body["serial"])
		}
		if len(issuesOf(body)) == 0 {
			t.Error("an entry with a blank item was stored with no issues reported")
		}
	})

	t.Run("text naming the subject is refused and nothing is stored", func(t *testing.T) {
		_, before := a.get("/api/register")
		rec := completeEntry()
		rec.Remarks = "Family asked the sex of the foetus"
		code, body := a.post("/api/entries", rec)
		if code != http.StatusUnprocessableEntity {
			t.Fatalf("POST /api/entries = %d, want 422: %v", code, body)
		}
		if body["error"] == nil {
			t.Error("the refusal carries no explanation")
		}
		found := false
		for _, raw := range issuesOf(body) {
			iss := raw.(map[string]any)
			if iss["kind"] == "refused" && iss["field"] == "remarks" {
				found = true
			}
		}
		if !found {
			t.Errorf("the refusal does not name the field: %v", issuesOf(body))
		}
		_, after := a.get("/api/register")
		if len(after) != len(before) {
			t.Error("a refused entry changed the register")
		}
	})

	t.Run("a correction replaces the entry in place", func(t *testing.T) {
		rec := completeEntry()
		rec.Serial = 3
		rec.Place = "Dharwad"
		rec.Remarks = "Place added on review."
		code, body := a.post("/api/entries/update", rec)
		if code != http.StatusOK {
			t.Fatalf("POST /api/entries/update = %d: %v", code, body)
		}
		if n := issuesOf(body); len(n) != 0 {
			t.Errorf("the corrected entry still reports issues: %v", n)
		}
		code, b := a.get("/api/entry?serial=3")
		if code != http.StatusOK {
			t.Fatalf("GET /api/entry?serial=3 = %d", code)
		}
		if !strings.Contains(string(b), "Place added on review.") {
			t.Error("the correction was not stored")
		}
	})

	t.Run("a correction to an entry that does not exist", func(t *testing.T) {
		rec := completeEntry()
		rec.Serial = 77
		code, body := a.post("/api/entries/update", rec)
		if code != http.StatusNotFound {
			t.Errorf("POST /api/entries/update for serial 77 = %d, want 404: %v", code, body)
		}
	})

	t.Run("a correction with no serial", func(t *testing.T) {
		code, _ := a.post("/api/entries/update", completeEntry())
		if code != http.StatusBadRequest {
			t.Errorf("an update with no serial = %d, want 400", code)
		}
	})

	t.Run("there is no way to remove an entry", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, a.srv.URL+"/api/entries", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusOK {
			t.Error("DELETE /api/entries succeeded; a gap in the numbering reads as a removed record and this tool must have no way to make one")
		}
	})

	t.Run("the numbering is unbroken after all of that", func(t *testing.T) {
		code, b := a.get("/api/register")
		if code != http.StatusOK {
			t.Fatalf("GET /api/register = %d", code)
		}
		var reg formf.PrintedRegister
		if err := json.Unmarshal(b, &reg); err != nil {
			t.Fatal(err)
		}
		if len(reg.Rows) != 3 {
			t.Fatalf("%d entries in the register, want 3", len(reg.Rows))
		}
		if !reg.Serial.Intact {
			t.Errorf("the numbering is not intact: %+v", reg.Serial.Issues)
		}
		if len(reg.Incomplete) != 0 {
			t.Errorf("entries still incomplete: %v", reg.Incomplete)
		}
	})
}

func TestAPIDraftCheckNamesEverythingAtOnce(t *testing.T) {
	a := newAPI(t)
	code, body := a.post("/api/check", formf.Record{})
	if code != http.StatusOK {
		t.Fatalf("POST /api/check = %d: %v", code, body)
	}
	issues := issuesOf(body)
	if len(issues) < 25 {
		t.Errorf("checking an empty draft reported %d issues; the prescribed set is longer than that", len(issues))
	}
	if body["mandatoryFields"].(float64) == 0 {
		t.Error("the check reports no mandatory fields at all")
	}
	if body["filledMandatoryFields"].(float64) != 0 {
		t.Error("an empty draft reports filled items")
	}
	// Nothing was stored by checking.
	_, b := a.get("/api/state")
	var st state
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Entries) != 0 {
		t.Error("checking a draft stored it")
	}
}

func TestAPITallies(t *testing.T) {
	a := newAPI(t)

	if code, body := a.post("/api/tallies", formf.ProcedureTally{Date: "2026-11-18", Count: 3, Source: "OPD register"}); code != http.StatusOK {
		t.Fatalf("POST /api/tallies = %d: %v", code, body)
	}
	// The same date again replaces the count rather than adding a second row.
	if code, _ := a.post("/api/tallies", formf.ProcedureTally{Date: "2026-11-18", Count: 4, Source: "OPD register"}); code != http.StatusOK {
		t.Fatalf("POST /api/tallies again = %d", code)
	}
	_, b := a.get("/api/state")
	var st state
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Tallies) != 1 || st.Tallies[0].Count != 4 {
		t.Errorf("tallies = %+v, want one row holding 4", st.Tallies)
	}

	for _, bad := range []formf.ProcedureTally{
		{Date: "18-11-2026", Count: 1},
		{Date: "", Count: 1},
		{Date: "2026-11-18", Count: -2},
	} {
		if code, _ := a.post("/api/tallies", bad); code != http.StatusBadRequest {
			t.Errorf("POST /api/tallies %+v = %d, want 400", bad, code)
		}
	}
}

func TestAPIMonthAndText(t *testing.T) {
	a := newAPI(t)
	if code, _ := a.post("/api/clinic", formf.Clinic{
		Name: "Sahyadri Imaging Centre", RegistrationNo: "PCPNDT/DHW/2019/0147",
		Address: "Court Circle, Dharwad", District: "Dharwad", State: "Karnataka",
		AppropriateAuthority: "District Appropriate Authority, Dharwad",
	}); code != http.StatusOK {
		t.Fatal("the clinic was not saved")
	}
	if code, _ := a.post("/api/entries", completeEntry()); code != http.StatusOK {
		t.Fatal("the entry was not saved")
	}
	if code, _ := a.post("/api/tallies", formf.ProcedureTally{Date: "2026-11-18", Count: 1, Source: "OPD register"}); code != http.StatusOK {
		t.Fatal("the day count was not saved")
	}

	code, b := a.get("/api/month?m=2026-11")
	if code != http.StatusOK {
		t.Fatalf("GET /api/month = %d: %s", code, b)
	}
	var ret formf.MonthlyReturn
	if err := json.Unmarshal(b, &ret); err != nil {
		t.Fatal(err)
	}
	if !ret.Fileable {
		t.Errorf("a reconciled month is not fileable: %v", ret.Blockers)
	}
	if ret.DueOn != "2026-12-05" {
		t.Errorf("DueOn = %q, want 2026-12-05", ret.DueOn)
	}
	if len(ret.Entries) != 1 {
		t.Errorf("the report carries %d entries, want 1", len(ret.Entries))
	}

	if code, _ := a.get("/api/month?m=2026-13"); code != http.StatusBadRequest {
		t.Errorf("GET /api/month for month thirteen = %d, want 400", code)
	}
	if code, _ := a.get("/api/month"); code != http.StatusOK {
		t.Error("GET /api/month with no month should fall back to the current one")
	}

	for _, p := range []string{"/api/text/register", "/api/text/month?m=2026-11", "/api/text/entry?serial=1"} {
		res, err := http.Get(a.srv.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d", p, res.StatusCode)
			continue
		}
		if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("GET %s served %q", p, ct)
		}
		if len(body) < 400 {
			t.Errorf("GET %s returned %d bytes, which is too little to be a page", p, len(body))
		}
		// The printed copy rule 9(7) asks for has to carry the
		// authentication block, or it is not a copy anyone can certify.
		if !strings.Contains(string(body), "Signature ____") && !strings.Contains(string(body), "COMPLETENESS") {
			t.Errorf("GET %s has neither an authentication block nor a completeness block", p)
		}
	}

	if code, _ := a.get("/api/text/entry?serial=404"); code != http.StatusNotFound {
		t.Error("the text of an entry that does not exist was served")
	}
	if code, _ := a.get("/api/entry?serial=notanumber"); code != http.StatusNotFound {
		t.Error("an entry was served for a serial that is not a number")
	}
}

func TestAPISettingsAreNormalisedAndPersisted(t *testing.T) {
	a := newAPI(t)
	code, b := a.post("/api/settings", map[string]any{
		"retentionYears":    400,
		"returnDueDay":      0,
		"openingSerial":     412,
		"responsiblePerson": "Dr M. Rao",
	})
	if code != http.StatusOK {
		t.Fatalf("POST /api/settings = %d: %v", code, b)
	}
	raw, _ := json.Marshal(b)
	var st state
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.Settings.RetentionYears != formf.RetentionYears {
		t.Errorf("a retention window of 400 years was accepted as %d", st.Settings.RetentionYears)
	}
	if st.Settings.ReturnDueDay != formf.DefaultReturnDueDay {
		t.Errorf("a due day of 0 was accepted as %d", st.Settings.ReturnDueDay)
	}
	if st.Settings.OpeningSerial != 412 {
		t.Errorf("OpeningSerial = %d, want 412", st.Settings.OpeningSerial)
	}
	if st.NextSeral != 412 {
		t.Errorf("NextSerial = %d after opening at 412, want 412", st.NextSeral)
	}

	// A district that really does require five years can have it.
	if code, _ := a.post("/api/settings", map[string]any{"retentionYears": 5, "returnDueDay": 7, "openingSerial": 1}); code != http.StatusOK {
		t.Fatal("a lawful override was refused")
	}
	_, b2 := a.get("/api/state")
	var st2 state
	if err := json.Unmarshal(b2, &st2); err != nil {
		t.Fatal(err)
	}
	if st2.Settings.RetentionYears != 5 || st2.Settings.ReturnDueDay != 7 {
		t.Errorf("the override did not persist: %+v", st2.Settings)
	}
	if !st2.Settings.Overridden() {
		t.Error("changed figures do not report themselves as overridden, so no page would say so")
	}
}

func TestAPIRejectsBodiesItDoesNotUnderstand(t *testing.T) {
	a := newAPI(t)
	for _, path := range []string{"/api/entries", "/api/clinic", "/api/settings", "/api/tallies", "/api/check"} {
		if code, _ := a.post(path, map[string]any{"somethingElse": 1}); code != http.StatusBadRequest {
			t.Errorf("POST %s with an unknown field = %d, want 400", path, code)
		}
	}
	res, err := http.Post(a.srv.URL+"/api/entries", "application/json", strings.NewReader("{not json"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /api/entries with a broken body = %d, want 400", res.StatusCode)
	}
}

func TestStateListsMonthsNewestFirst(t *testing.T) {
	d := fileData{
		Settings: formf.DefaultSettings(),
		Entries: []formf.Record{
			{Serial: 1, ProcedureDate: "2026-09-11"},
			{Serial: 2, ProcedureDate: "2026-11-03"},
			{Serial: 3, ProcedureDate: ""},
		},
		Tallies: []formf.ProcedureTally{{Date: "2026-10-04", Count: 2}, {Date: "nonsense", Count: 1}},
	}
	got := buildState(d, formf.Day(2026, 12, 2)).Months
	want := []string{"2026-12", "2026-11", "2026-10", "2026-09"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("months = %v, want %v", got, want)
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i] > got[j] }) {
		t.Error("the months are not newest first")
	}
}

func TestStateListsEntriesNewestFirst(t *testing.T) {
	d := fileData{
		Settings: formf.DefaultSettings(),
		Entries:  []formf.Record{{Serial: 1}, {Serial: 3}, {Serial: 2}},
	}
	st := buildState(d, formf.Day(2026, 12, 2))
	var serials []int
	for _, e := range st.Entries {
		serials = append(serials, e.Entry.Serial)
	}
	if want := []int{3, 2, 1}; !reflect.DeepEqual(serials, want) {
		t.Errorf("entries listed as %v, want %v", serials, want)
	}
	for _, e := range st.Entries {
		if e.Validation.Serial != e.Entry.Serial {
			t.Errorf("entry %d carries a validation for serial %d", e.Entry.Serial, e.Validation.Serial)
		}
	}
}
