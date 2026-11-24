/* Form F register — the interface.
 *
 * The form is built from the field catalogue the program serves, so the boxes
 * on screen and the items the checker walks cannot drift apart: add a field in
 * one place and it appears in the other.
 *
 * Everything here talks to this program on this machine over the loopback
 * address. There is no other destination, and no code in this file that could
 * reach one. */
"use strict";

(function () {
  var state = null;
  var editing = 0;            // the serial being corrected, or 0 for a new entry
  var shownEntry = 0;         // the serial whose Form F face is open
  var currentMonth = "";

  // ------------------------------------------------------------- utilities

  function $(id) { return document.getElementById(id); }

  // asArray normalises a value that may be null into an array. Go writes an
  // empty slice as null, and a page that forgot to allow for that would break
  // on exactly the register a clinic sees on its first day: the empty one.
  // The name matters: several functions here declare a local `list`, and a
  // var declaration hoists over a helper for the whole of its function body.
  function asArray(v) { return v || []; }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) { n.className = cls; }
    if (text !== undefined && text !== null) { n.textContent = String(text); }
    return n;
  }

  function clear(node) { while (node.firstChild) { node.removeChild(node.firstChild); } }

  function say(msg, bad) {
    var b = $("banner");
    b.textContent = msg;
    b.className = bad ? "banner bad" : "banner";
    b.hidden = false;
    if (!bad) { window.setTimeout(function () { b.hidden = true; }, 6000); }
  }

  function get(url) {
    return window.fetch(url, { headers: { "Accept": "application/json" } }).then(function (res) {
      return res.json().then(function (body) {
        if (!res.ok) { throw new Error(body && body.error ? body.error : "request failed"); }
        return body;
      });
    });
  }

  function post(url, payload) {
    return window.fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json", "Accept": "application/json" },
      body: JSON.stringify(payload)
    }).then(function (res) {
      return res.json().then(function (body) { return { ok: res.ok, body: body }; });
    });
  }

  function theTab() {
    var b = document.querySelector("#tabs button[aria-pressed=true]");
    return b ? b.getAttribute("data-tab") : "entry";
  }

  function showTab(name) {
    var buttons = document.querySelectorAll("#tabs button");
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].setAttribute("aria-pressed", buttons[i].getAttribute("data-tab") === name ? "true" : "false");
    }
    ["entry", "register", "month", "settings"].forEach(function (n) {
      $("panel-" + n).hidden = n !== name;
    });
    if (name === "register") { loadRegister(); }
    if (name === "month") { loadMonth(); }
  }

  // --------------------------------------------------------- the entry form

  function fieldNode(f) {
    var wrap = el("div", "field");
    wrap.setAttribute("data-field", f.key);

    var label = el("label");
    label.setAttribute("for", "f-" + f.key);
    if (f.item) { label.appendChild(el("span", "item", f.item)); }
    label.appendChild(document.createTextNode(f.label));
    if (f.mandatory) {
      label.appendChild(el("span", "hint", " required"));
    } else if (f.conditional) {
      label.appendChild(el("span", "hint", " required " + f.when));
    }

    var input;
    if (f.kind === "check") {
      var line = el("div", "check");
      input = el("input");
      input.type = "checkbox";
      input.id = "f-" + f.key;
      line.appendChild(input);
      line.appendChild(el("span", null, f.label));
      wrap.appendChild(line);
    } else if (f.kind === "multi" || f.kind === "choice") {
      wrap.appendChild(label);
      if (f.kind === "choice") {
        input = el("select");
        input.id = "f-" + f.key;
        input.appendChild(el("option", null, ""));
        (f.choices || []).forEach(function (c) {
          var o = el("option", null, c);
          o.value = c;
          input.appendChild(o);
        });
        var box = el("div", "inp");
        box.appendChild(input);
        wrap.appendChild(box);
      } else {
        var group = el("div", "choices");
        group.id = "f-" + f.key;
        (f.choices || []).forEach(function (c) {
          var row = el("label");
          var cb = el("input");
          cb.type = "checkbox";
          cb.value = c;
          row.appendChild(cb);
          var num = numberFor(f.key, c);
          if (num) { row.appendChild(el("span", "num", num)); }
          row.appendChild(el("span", null, c));
          group.appendChild(row);
        });
        wrap.appendChild(group);
      }
    } else {
      wrap.appendChild(label);
      if (f.kind === "longtext") {
        input = el("textarea");
      } else {
        input = el("input");
        input.type = f.kind === "date" ? "date" : (f.kind === "number" ? "number" : "text");
        if (f.kind === "number") { input.min = "0"; input.step = "1"; }
      }
      input.id = "f-" + f.key;
      var holder = el("div", "inp");
      holder.appendChild(input);
      wrap.appendChild(holder);
    }

    if (f.help) { wrap.appendChild(el("p", "help", f.help)); }
    return wrap;
  }

  function numberFor(key, choice) {
    if (key !== "indications" || !state) { return ""; }
    for (var i = 0; i < state.indications.length; i++) {
      if (state.indications[i].text === choice) { return state.indications[i].num; }
    }
    return "";
  }

  function buildEntryForm() {
    var host = $("entry-fields");
    clear(host);
    var group = null;
    var name = "";
    state.fields.forEach(function (f) {
      if (f.key === "serial") { return; }   // assigned by the register
      if (f.group !== name) {
        name = f.group;
        group = el("fieldset", "group");
        group.appendChild(el("legend", null, name));
        host.appendChild(group);
      }
      group.appendChild(fieldNode(f));
    });
  }

  function readForm() {
    var rec = { serial: editing };
    state.fields.forEach(function (f) {
      if (f.key === "serial") { return; }
      var node = $("f-" + f.key);
      if (!node) { return; }
      if (f.kind === "check") {
        rec[f.key] = node.checked;
      } else if (f.kind === "multi") {
        var chosen = [];
        var boxes = node.querySelectorAll("input[type=checkbox]");
        for (var i = 0; i < boxes.length; i++) { if (boxes[i].checked) { chosen.push(boxes[i].value); } }
        rec[f.key] = chosen;
      } else if (f.kind === "number") {
        var raw = node.value.trim();
        if (raw === "") {
          // The living-children item has to tell nil apart from
          // unanswered, so a blank there is null rather than zero.
          rec[f.key] = f.key === "livingChildren" ? null : 0;
        } else {
          rec[f.key] = parseInt(raw, 10);
        }
      } else {
        rec[f.key] = node.value;
      }
    });
    return rec;
  }

  function writeForm(rec) {
    state.fields.forEach(function (f) {
      if (f.key === "serial") { return; }
      var node = $("f-" + f.key);
      if (!node) { return; }
      var v = rec ? rec[f.key] : null;
      if (f.kind === "check") {
        node.checked = !!v;
      } else if (f.kind === "multi") {
        var boxes = node.querySelectorAll("input[type=checkbox]");
        for (var i = 0; i < boxes.length; i++) {
          boxes[i].checked = !!(v && v.indexOf(boxes[i].value) >= 0);
        }
      } else if (f.kind === "number") {
        node.value = (v === null || v === undefined || v === 0) ? "" : String(v);
      } else {
        node.value = v === null || v === undefined ? "" : String(v);
      }
    });
  }

  function markBlanks(validation) {
    var blank = {};
    (validation.issues || []).forEach(function (i) {
      if (i.kind === "missing" || i.kind === "refused") { blank[i.field] = true; }
    });
    var wraps = document.querySelectorAll("#entry-fields .field");
    for (var i = 0; i < wraps.length; i++) {
      var key = wraps[i].getAttribute("data-field");
      if (blank[key]) { wraps[i].classList.add("blank"); } else { wraps[i].classList.remove("blank"); }
    }
  }

  function showValidation(v) {
    $("check-count").textContent = v.filledMandatoryFields + " / " + v.mandatoryFields;
    $("check-summary").textContent = v.summary;

    var host = $("check-issues");
    clear(host);
    if (!v.issues || v.issues.length === 0) {
      host.appendChild(el("p", "ok-line", "Nothing outstanding. This entry is a complete Form F as far as the prescribed items go."));
      markBlanks(v);
      return;
    }
    var list = el("ul", "issues");
    v.issues.forEach(function (i) {
      var li = el("li", i.kind);
      var where = el("span", "where");
      if (i.item) { where.appendChild(el("span", "n", i.item + " · ")); }
      where.appendChild(document.createTextNode(i.label || i.field));
      li.appendChild(where);
      li.appendChild(document.createTextNode(i.message));
      list.appendChild(li);
    });
    host.appendChild(list);
    markBlanks(v);
  }

  function checkDraft() {
    return post("/api/check", readForm()).then(function (r) {
      if (!r.ok) { say(r.body.error || "could not check the entry", true); return null; }
      showValidation(r.body);
      return r.body;
    });
  }

  function saveEntry() {
    var rec = readForm();
    var url = editing > 0 ? "/api/entries/update" : "/api/entries";
    return post(url, rec).then(function (r) {
      if (!r.ok) {
        if (r.body.validation) { showValidation(r.body.validation); }
        say(r.body.error || "the entry was not saved", true);
        return;
      }
      showValidation(r.body.validation);
      var complete = asArray(r.body.validation.issues).length === 0;
      if (editing > 0) {
        say("Entry " + r.body.serial + " corrected." + (complete ? "" : " It still has items outstanding."));
      } else {
        say("Saved as entry " + r.body.serial + "." + (complete ? "" : " It still has items outstanding — correct it from the register."));
      }
      editing = 0;
      $("entry-heading").textContent = "Form F — new entry";
      $("entry-save").textContent = "Save this entry";
      return refresh().then(function () { startFresh(true); });
    });
  }

  function startFresh(keepMessages) {
    editing = 0;
    $("entry-heading").textContent = "Form F — new entry";
    $("entry-save").textContent = "Save this entry";
    writeForm(null);
    // Prefill the things that are the same for every entry of the day.
    if (state) {
      ["firstReportedOn", "procedureDate", "womanDeclarationDate", "signedOn", "doctorDeclarationDate"].forEach(function (k) {
        var n = $("f-" + k);
        if (n) { n.value = state.today; }
      });
    }
    var host = $("check-issues");
    clear(host);
    $("check-count").textContent = "—";
    if (!keepMessages) { $("check-summary").textContent = "Fill the form and the whole shortfall appears here at once."; }
    markBlanks({ issues: [] });
  }

  function editEntry(serial) {
    get("/api/entry?serial=" + encodeURIComponent(serial)).then(function (body) {
      editing = serial;
      writeForm(body.entry);
      showValidation(body.validation);
      $("entry-heading").textContent = "Form F — correcting entry " + serial;
      $("entry-save").textContent = "Save the correction";
      showTab("entry");
      window.scrollTo(0, 0);
    }).catch(function (e) { say(String(e.message || e), true); });
  }

  // ------------------------------------------------------------- register

  function cell(row, text, cls) {
    var td = el("td", cls, text);
    row.appendChild(td);
    return td;
  }

  function loadRegister() {
    get("/api/register").then(function (reg) {
      reg.rows = asArray(reg.rows);
      reg.serial.issues = asArray(reg.serial.issues);
      reg.incompleteSerials = asArray(reg.incompleteSerials);
      var status = $("register-status");
      clear(status);

      var strip = el("div", "strip");
      strip.appendChild(statCell("Entries", String(reg.rows.length), ""));
      var numbering = reg.serial.issues.filter(function (i) { return i.problem !== "date-out-of-order"; });
      var ordering = reg.serial.issues.filter(function (i) { return i.problem === "date-out-of-order"; });
      strip.appendChild(statCell("Numbering",
        reg.serial.intact ? "intact" : (numbering.length + " to fix"),
        reg.serial.intact ? "good" : "bad"));
      if (ordering.length) {
        strip.appendChild(statCell("Dates against numbers", ordering.length + " to explain", "warn"));
      }
      strip.appendChild(statCell("Incomplete Form F",
        String(reg.incompleteSerials.length),
        reg.incompleteSerials.length ? "bad" : "good"));
      strip.appendChild(statCell("Scrutinised at an inspection", String(reg.inspectionSample), ""));
      strip.appendChild(statCell("Still to be held", String(reg.retention.holding), ""));
      strip.appendChild(statCell("Window closed", String(reg.retention.releasable), reg.retention.releasable ? "warn" : ""));
      if (reg.retention.openEnded) { strip.appendChild(statCell("Open-ended", String(reg.retention.openEnded), "warn")); }
      status.appendChild(strip);

      if (reg.serial.issues.length) {
        var list = el("ul", "issues");
        reg.serial.issues.forEach(function (i) {
          var late = i.problem === "date-out-of-order";
          var li = el("li", late ? "" : "missing");
          li.appendChild(el("span", "where", late ? "worth explaining · " + i.problem : i.problem));
          li.appendChild(document.createTextNode(i.message));
          list.appendChild(li);
        });
        status.appendChild(list);
      }

      var table = $("register-table");
      clear(table);
      var head = el("tr");
      reg.columns.forEach(function (c) {
        var th = el("th", null, c.heading);
        th.appendChild(el("span", "src", c.source));
        head.appendChild(th);
      });
      var thead = el("thead");
      thead.appendChild(head);
      table.appendChild(thead);

      var body = el("tbody");
      if (reg.rows.length === 0) {
        var none = el("tr");
        var td = el("td", null, "No entries yet.");
        td.colSpan = reg.columns.length;
        none.appendChild(td);
        body.appendChild(none);
      }
      reg.rows.forEach(function (r) {
        var tr = el("tr", r.complete ? "" : "flagged");
        r.cells.forEach(function (v, i) {
          var isState = reg.columns[i].key === "formF";
          if (isState) {
            var td = el("td");
            td.appendChild(el("span", "tag " + (r.complete ? "good" : "bad"), v));
            tr.appendChild(td);
          } else {
            cell(tr, v, i === 2 ? "wrap" : "");
          }
        });
        body.appendChild(tr);
      });
      table.appendChild(body);

      $("register-note").textContent =
        "Entries in serial order with the five columns rule 9(1) prescribes, then three of this register's own. " +
        reg.authentication.statement;

      renderEntryList();
    }).catch(function (e) { say(String(e.message || e), true); });
  }

  function statCell(k, v, cls) {
    var c = el("div", "cell" + (cls ? " " + cls : ""));
    c.appendChild(el("div", "k", k));
    c.appendChild(el("div", "v", v));
    return c;
  }

  function renderEntryList() {
    var host = $("entry-list");
    clear(host);
    if (!state.entries || state.entries.length === 0) {
      host.appendChild(el("p", "empty", "Nothing recorded yet. Start with a new entry."));
      return;
    }
    var list = el("div", "rowlist");
    state.entries.forEach(function (v) {
      var b = el("button");
      b.type = "button";
      b.setAttribute("data-serial", String(v.entry.serial));
      b.appendChild(el("span", "sl", "#" + v.entry.serial));
      b.appendChild(el("span", null, v.entry.patientName || "(no name recorded)"));
      b.appendChild(el("span", "meta", v.entry.procedureDate || "(no date)"));
      var issues = (v.validation.issues || []).length;
      b.appendChild(el("span", "tag " + (issues ? "bad" : "good"), issues ? issues + " to fix" : "complete"));
      if (v.retention.openEnded) { b.appendChild(el("span", "tag warn", "proceedings noted")); }
      else if (v.retention.releasable) { b.appendChild(el("span", "tag warn", "window closed")); }
      list.appendChild(b);
    });
    host.appendChild(list);
  }

  function showFace(serial) {
    get("/api/entry?serial=" + encodeURIComponent(serial)).then(function (body) {
      shownEntry = serial;
      $("face-card").hidden = false;
      $("face-heading").textContent = "Form F — entry " + serial + " · " + body.validation.summary;
      $("face-text").href = "/api/text/entry?serial=" + encodeURIComponent(serial);

      var table = $("face-table");
      clear(table);
      var tbody = el("tbody");
      var group = "";
      body.face.forEach(function (l) {
        if (l.group !== group) {
          group = l.group;
          var gh = el("tr", "grouphead");
          var gtd = el("td", null, group);
          gtd.colSpan = 3;
          gh.appendChild(gtd);
          tbody.appendChild(gh);
        }
        var tr = el("tr", l.missing ? "flagged" : "");
        var first = el("td");
        if (l.missing) { first.appendChild(el("span", "mark", "! ")); }
        first.appendChild(document.createTextNode(l.item || "—"));
        tr.appendChild(first);
        cell(tr, l.label, "wrap");
        cell(tr, l.value || "—", "wrap");
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);

      $("face-note").textContent = body.retention.note;
      $("face-card").scrollIntoView();
    }).catch(function (e) { say(String(e.message || e), true); });
  }

  // ---------------------------------------------------------------- month

  function loadMonth() {
    var m = $("month-pick").value || currentMonth;
    if (!m) { return; }
    currentMonth = m;
    $("month-text").href = "/api/text/month?m=" + encodeURIComponent(m);

    get("/api/month?m=" + encodeURIComponent(m)).then(function (ret) {
      ret.blockers = asArray(ret.blockers);
      ret.entries = asArray(ret.entries);
      ret.check.days = asArray(ret.check.days);
      ret.check.findings = asArray(ret.check.findings);
      ret.check.incompleteSerials = asArray(ret.check.incompleteSerials);
      $("month-count").textContent = String(ret.totals.entries);
      $("month-due").value = ret.dueOn + (ret.overdue ? "  (overdue by " + (-ret.daysToDue) + " days)" : "  (" + ret.daysToDue + " days to go)");
      $("month-state").textContent = ret.fileable
        ? "Ready to copy onto the format your Appropriate Authority requires."
        : "Not ready to send: " + ret.blockers.length + " thing(s) to put right first.";
      $("month-rule").textContent = ret.reference;
      $("month-format").textContent = ret.formatNote;

      var host = $("month-blockers");
      clear(host);
      if (ret.fileable) {
        host.appendChild(el("p", "ok-line", "Every counted procedure has a complete Form F, and the numbering is intact."));
      } else {
        var list = el("ul", "issues");
        ret.blockers.forEach(function (b) {
          var li = el("li", "missing");
          li.appendChild(document.createTextNode(b));
          list.appendChild(li);
        });
        ret.check.findings.forEach(function (f) {
          var li = el("li");
          li.appendChild(document.createTextNode(f));
          list.appendChild(li);
        });
        host.appendChild(list);
      }

      var days = $("month-days");
      clear(days);
      var head = el("tr");
      ["Date", "Day", "Counted", "Form F", "Status", "Serials"].forEach(function (h) {
        head.appendChild(el("th", null, h));
      });
      var thead = el("thead");
      thead.appendChild(head);
      days.appendChild(thead);
      var tbody = el("tbody");
      if (ret.check.days.length === 0) {
        var tr = el("tr");
        var td = el("td", null, "Nothing recorded for this month yet.");
        td.colSpan = 6;
        tr.appendChild(td);
        tbody.appendChild(tr);
      }
      ret.check.days.forEach(function (d) {
        var tr = el("tr", d.status === "matched" ? "" : "flagged");
        cell(tr, d.date);
        cell(tr, d.weekday.slice(0, 3));
        cell(tr, String(d.tallied), "num");
        cell(tr, String(d.entries), "num");
        var std = el("td");
        std.appendChild(el("span", "tag " + (d.status === "matched" ? "good" : (d.status === "forms-missing" ? "bad" : "warn")), d.status));
        tr.appendChild(std);
        cell(tr, (d.serials || []).join(", "), "wrap");
        if (d.message) { tr.title = d.message; }
        tbody.appendChild(tr);
      });
      days.appendChild(tbody);

      var totals = $("month-totals");
      clear(totals);
      var t = ret.totals;
      var strip = el("div", "strip");
      strip.appendChild(statCell("Form F entries", String(t.entries), ""));
      strip.appendChild(statCell("Procedures counted", String(ret.check.tallied), ""));
      strip.appendChild(statCell("Form F missing", String(ret.check.formsMissing), ret.check.formsMissing ? "bad" : "good"));
      strip.appendChild(statCell("Entries on uncounted days", String(ret.check.untalliedEntries), ret.check.untalliedEntries ? "warn" : ""));
      strip.appendChild(statCell("Self-referrals", String(t.selfReferrals), ""));
      strip.appendChild(statCell("Termination indicated", String(t.mtpIndications), ""));
      totals.appendChild(strip);

      [["By indication — item 10", t.indications], ["By referral — item 7", t.referrals], ["By procedure — item 11", t.procedures]].forEach(function (pair) {
        if (!pair[1] || pair[1].length === 0) { return; }
        totals.appendChild(el("h3", null, pair[0]));
        var table = el("table", "breakdown");
        var tb = el("tbody");
        pair[1].forEach(function (c) {
          var tr = el("tr");
          cell(tr, c.name, "wrap");
          cell(tr, String(c.value), "num");
          tb.appendChild(tr);
        });
        table.appendChild(tb);
        var sc = el("div", "scroll");
        sc.appendChild(table);
        totals.appendChild(sc);
      });
    }).catch(function (e) { say(String(e.message || e), true); });
  }

  function saveTally(ev) {
    ev.preventDefault();
    post("/api/tallies", {
      date: $("tally-date").value,
      count: parseInt($("tally-count").value || "0", 10),
      source: $("tally-source").value,
      note: ""
    }).then(function (r) {
      if (!r.ok) { say(r.body.error || "the day's count was not saved", true); return; }
      say("Recorded " + r.body.saved.count + " procedure(s) on " + r.body.saved.date + ".");
      $("tally-count").value = "";
      return refresh().then(loadMonth);
    });
  }

  // ------------------------------------------------------------- settings

  function fillSettings() {
    $("c-name").value = state.clinic.name || "";
    $("c-address").value = state.clinic.address || "";
    $("c-reg").value = state.clinic.registrationNo || "";
    $("c-district").value = state.clinic.district || "";
    $("c-state").value = state.clinic.state || "";
    $("c-authority").value = state.clinic.appropriateAuthority || "";

    $("s-retention").value = String(state.settings.retentionYears);
    $("s-due").value = String(state.settings.returnDueDay);
    $("s-opening").value = String(state.settings.openingSerial);
    $("s-person").value = state.settings.responsiblePerson || "";

    $("src-retention").textContent = state.references.retention;
    $("src-report").textContent = state.references.monthlyReport;
    $("src-printed").textContent = state.references.printedCopy;

    var host = $("not-stored");
    clear(host);
    var box = el("div", "notstored");
    state.itemsNotStored.forEach(function (n) {
      var one = el("div", "one");
      one.appendChild(el("h3", null, n.item));
      one.appendChild(el("p", null, n.asks));
      one.appendChild(el("p", null, n.reason));
      one.appendChild(el("p", null, n.where));
      box.appendChild(one);
    });
    host.appendChild(box);
  }

  function saveClinic(ev) {
    ev.preventDefault();
    post("/api/clinic", {
      name: $("c-name").value,
      address: $("c-address").value,
      registrationNo: $("c-reg").value,
      district: $("c-district").value,
      state: $("c-state").value,
      appropriateAuthority: $("c-authority").value
    }).then(function (r) {
      if (!r.ok) { say(r.body.error || "the centre's details were not saved", true); return; }
      state = r.body;
      fillSettings();
      say("Saved. These go on items 1 and 2 of every Form F and on the face of every report.");
    });
  }

  function saveSettings(ev) {
    ev.preventDefault();
    post("/api/settings", {
      retentionYears: parseInt($("s-retention").value || "0", 10),
      returnDueDay: parseInt($("s-due").value || "0", 10),
      openingSerial: parseInt($("s-opening").value || "0", 10),
      responsiblePerson: $("s-person").value
    }).then(function (r) {
      if (!r.ok) { say(r.body.error || "the figures were not saved", true); return; }
      state = r.body;
      fillSettings();
      say("Saved. Any figure outside a usable range was put back to this tool's default.");
    });
  }

  // ------------------------------------------------------------------ boot

  function refresh() {
    return get("/api/state").then(function (body) {
      state = body;
      state.entries = asArray(state.entries);
      state.tallies = asArray(state.tallies);
      var months = $("month-pick");
      var chosen = months.value;
      clear(months);
      state.months.forEach(function (m) {
        var o = el("option", null, m);
        o.value = m;
        months.appendChild(o);
      });
      if (chosen && state.months.indexOf(chosen) >= 0) { months.value = chosen; }
      else if (state.months.length) { months.value = state.months[0]; }
      currentMonth = months.value;
      if (!$("tally-date").value) { $("tally-date").value = state.today; }
      renderEntryList();
      return body;
    });
  }

  function start() {
    get("/api/state").then(function (body) {
      state = body;
      state.entries = asArray(state.entries);
      state.tallies = asArray(state.tallies);
      $("policy-text").textContent = state.policy;
      $("consequences").textContent = state.consequences;
      $("file-note").textContent = "this machine only · " + asArray(state.entries).length + " entries";
      buildEntryForm();
      fillSettings();
      return refresh();
    }).then(function () {
      startFresh();
      showTab("entry");
    }).catch(function (e) {
      say("Could not read the register: " + String(e.message || e), true);
    });

    document.getElementById("tabs").addEventListener("click", function (ev) {
      var b = ev.target.closest("button[data-tab]");
      if (b) { showTab(b.getAttribute("data-tab")); }
    });
    $("entry-form").addEventListener("submit", function (ev) { ev.preventDefault(); saveEntry(); });
    $("entry-check").addEventListener("click", function () { checkDraft(); });
    $("entry-reset").addEventListener("click", function () { startFresh(); });
    $("entry-fields").addEventListener("change", function () { if (theTab() === "entry") { checkDraft(); } });

    $("entry-list").addEventListener("click", function (ev) {
      var b = ev.target.closest("button[data-serial]");
      if (b) { showFace(parseInt(b.getAttribute("data-serial"), 10)); }
    });
    $("face-edit").addEventListener("click", function () { if (shownEntry) { editEntry(shownEntry); } });
    $("register-print").addEventListener("click", function () { window.print(); });
    $("month-print").addEventListener("click", function () { window.print(); });
    $("month-pick").addEventListener("change", loadMonth);
    $("tally-form").addEventListener("submit", saveTally);
    $("clinic-form").addEventListener("submit", saveClinic);
    $("settings-form").addEventListener("submit", saveSettings);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
